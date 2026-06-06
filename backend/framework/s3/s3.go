// Package s3 封装一个最小可用的 S3 / S3 兼容对象存储客户端，作为框架层基础能力。
//
// 设计取向与 framework/midplat 一致：
//   - 仅暴露业务常用的几个操作（Put/Get/Delete/Head/List + 预签名 URL），不透出底层 SDK 类型；
//   - 配置缺失时由调用方判断并保持 nil，相关接口返回「未配置」错误，不影响其它模块与测试；
//   - 支持自定义 Endpoint（MinIO / 阿里云 OSS / 腾讯云 COS 等 S3 兼容服务）与 path-style 寻址。
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Config 是创建 Client 所需的配置。AccessKeyID/SecretAccessKey/Bucket 必填。
type Config struct {
	// Endpoint 自定义服务端点（含协议，如 https://oss-cn-hangzhou.aliyuncs.com）。
	// 为空时走 AWS S3 官方端点（由 Region 决定）。
	Endpoint string
	// Region 区域；S3 兼容服务可填任意占位值（如 us-east-1），但不能为空。
	Region string

	AccessKeyID     string
	SecretAccessKey string
	// SessionToken 临时凭证可选；长期 AK/SK 留空即可。
	SessionToken string

	// Bucket 默认操作的存储桶。
	Bucket string

	// UsePathStyle 使用 path-style 寻址（endpoint/bucket/key）。
	// MinIO 及多数自建兼容服务需要 true；AWS S3 / 多数云厂商用 virtual-hosted（false）。
	UsePathStyle bool

	// PublicBaseURL 公开访问 / CDN 前缀（如 https://cdn.example.com 或 https://bucket.example.com）。
	// 用于 PublicURL 拼接对象的公开地址；为空时回退到 Endpoint+Bucket 拼接。
	PublicBaseURL string

	// HTTPTimeout 单次请求超时；为 0 时默认 30s（仅作用于非预签名的实际调用）。
	HTTPTimeout time.Duration
}

// Client 是对象存储客户端。
type Client struct {
	cfg     Config
	api     *awss3.Client
	presign *awss3.PresignClient
}

// New 根据配置创建一个 Client。
func New(cfg Config) (*Client, error) {
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3: AccessKeyID/SecretAccessKey 必填")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: Bucket 必填")
	}
	if cfg.Region == "" {
		// S3 兼容服务对 Region 不敏感，但 SDK 签名需要一个非空值。
		cfg.Region = "us-east-1"
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")

	awsCfg := aws.Config{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
	}

	api := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Client{
		cfg:     cfg,
		api:     api,
		presign: awss3.NewPresignClient(api),
	}, nil
}

// Bucket 返回默认存储桶名，主要供调用方/测试使用。
func (c *Client) Bucket() string { return c.cfg.Bucket }

// PutObject 上传一个对象。contentType 为空时不显式设置（由服务端推断）。
func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	in := &awss3.PutObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if _, err := c.api.PutObject(ctx, in); err != nil {
		return fmt.Errorf("s3: 上传对象 %s 失败: %w", key, err)
	}
	return nil
}

// GetObject 下载一个对象，调用方负责关闭返回的 ReadCloser。
func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.api.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: 下载对象 %s 失败: %w", key, err)
	}
	return out.Body, nil
}

// DeleteObject 删除一个对象。
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	if _, err := c.api.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("s3: 删除对象 %s 失败: %w", key, err)
	}
	return nil
}

// Exists 判断对象是否存在。
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.api.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	return false, fmt.Errorf("s3: 查询对象 %s 失败: %w", key, err)
}

// ListObjects 列出指定前缀下的对象 key（最多 limit 个；limit<=0 时由服务端默认上限决定）。
func (c *Client) ListObjects(ctx context.Context, prefix string, limit int32) ([]string, error) {
	in := &awss3.ListObjectsV2Input{
		Bucket: aws.String(c.cfg.Bucket),
		Prefix: aws.String(prefix),
	}
	if limit > 0 {
		in.MaxKeys = aws.Int32(limit)
	}
	out, err := c.api.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("s3: 列举前缀 %s 失败: %w", prefix, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

// PresignGetURL 生成一个带时效的下载预签名 URL。
func (c *Client) PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("s3: 生成下载预签名 %s 失败: %w", key, err)
	}
	return req.URL, nil
}

// PresignPutURL 生成一个带时效的上传预签名 URL，供前端直传。
func (c *Client) PresignPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignPutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}, awss3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("s3: 生成上传预签名 %s 失败: %w", key, err)
	}
	return req.URL, nil
}

// PublicURL 拼接对象的公开访问地址（不做签名，要求对象/桶本身可公开读）。
// 优先使用 PublicBaseURL；否则用 Endpoint+Bucket 兜底（path-style）。
func (c *Client) PublicURL(key string) string {
	key = strings.TrimLeft(key, "/")
	if c.cfg.PublicBaseURL != "" {
		return c.cfg.PublicBaseURL + "/" + key
	}
	if c.cfg.Endpoint != "" {
		return c.cfg.Endpoint + "/" + c.cfg.Bucket + "/" + key
	}
	return key
}
