package framework

import (
	"log"

	"manager-backend/framework/s3"
)

// S3 是框架层共享的对象存储客户端；未配置（缺 AK/SK/Bucket）时为 nil。
// 业务模块在 Init 时判空使用：为 nil 即「未配置 S3」，相关接口应返回未配置错误，
// 不影响其它模块与测试（与 midplat 客户端同样的可选能力约定）。
//
// S3 指向公开资源桶（S3_BUCKET，partner 合伙人素材等经 S3_PUBLIC_BASE_URL 公开访问）。
var S3 *s3.Client

// S3Library 是素材库专用的对象存储客户端，指向一套【完全独立】配置的【私有】S3
// （S3_LIBRARY_*，可与公开 S3 是不同服务/厂商）。私有桶无匿名读、无匿名 list、无公开前缀，
// 素材库文件只能经后端鉴权后用 presigned 短链访问。S3_LIBRARY_ 的 AK/SK/Bucket 缺任一则为 nil
// （素材库存储不可用，相关接口返回未配置错误）——不回退公开 S3，杜绝私密文件落入公开桶。
var S3Library *s3.Client

// InitS3 依据全局 AppConfig 独立构造两个对象存储客户端：公开资源桶 S3（S3_*）与
// 素材库私有桶 S3Library（S3_LIBRARY_*）。两者配置完全独立、互不回退；各自缺 AK/SK/Bucket
// 则对应客户端保持 nil（相关接口返回未配置错误），让缺省部署可无 S3 正常启动。
// 构造失败（参数非法）才返回错误。应在 LoadConfig 之后、模块 Init 之前调用一次。
func InitS3() error {
	cfg := AppConfig
	if cfg == nil {
		return nil
	}

	// 公开资源桶（S3_*，带 PublicBaseURL，供 partner 等公开资源拼公开链接）。
	if cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "" || cfg.S3Bucket == "" {
		log.Println("[s3] 公开桶未配置（缺 S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY/S3_BUCKET），跳过")
	} else {
		c, err := s3.New(s3.Config{
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			Bucket:          cfg.S3Bucket,
			UsePathStyle:    cfg.S3UsePathStyle,
			PublicBaseURL:   cfg.S3PublicBaseURL,
		})
		if err != nil {
			return err
		}
		S3 = c
		log.Printf("[s3] 公开桶已初始化  bucket=%s  endpoint=%s", cfg.S3Bucket, displayOrEmpty(cfg.S3Endpoint))
	}

	// 素材库私有桶（S3_LIBRARY_*，完全独立配置；PublicBaseURL 恒空）。
	// 不回退公开 S3：缺 AK/SK/Bucket 则 S3Library 保持 nil。
	if cfg.S3LibraryAccessKeyID == "" || cfg.S3LibrarySecretAccessKey == "" || cfg.S3LibraryBucket == "" {
		log.Println("[s3] 素材库私有桶未配置（缺 S3_LIBRARY_ACCESS_KEY_ID/S3_LIBRARY_SECRET_ACCESS_KEY/S3_LIBRARY_BUCKET），素材库存储不可用")
		return nil
	}
	lib, err := s3.New(s3.Config{
		Endpoint:        cfg.S3LibraryEndpoint,
		Region:          cfg.S3LibraryRegion,
		AccessKeyID:     cfg.S3LibraryAccessKeyID,
		SecretAccessKey: cfg.S3LibrarySecretAccessKey,
		Bucket:          cfg.S3LibraryBucket,
		UsePathStyle:    cfg.S3LibraryUsePathStyle,
		PublicBaseURL:   "", // 私有桶：恒无公开前缀
	})
	if err != nil {
		return err
	}
	S3Library = lib
	log.Printf("[s3] 素材库私有桶已初始化  bucket=%s  endpoint=%s", cfg.S3LibraryBucket, displayOrEmpty(cfg.S3LibraryEndpoint))
	return nil
}
