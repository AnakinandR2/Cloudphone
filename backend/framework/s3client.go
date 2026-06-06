package framework

import (
	"log"

	"manager-backend/framework/s3"
)

// S3 是框架层共享的对象存储客户端；未配置（缺 AK/SK/Bucket）时为 nil。
// 业务模块在 Init 时判空使用：为 nil 即「未配置 S3」，相关接口应返回未配置错误，
// 不影响其它模块与测试（与 midplat 客户端同样的可选能力约定）。
var S3 *s3.Client

// InitS3 依据全局 AppConfig 构造对象存储客户端。
//
// AccessKeyID/SecretAccessKey/Bucket 缺任一则不构造（S3 保持 nil），返回 nil（非错误），
// 让缺省部署可以无 S3 正常启动。构造失败（如参数非法）才返回错误。
// 应在 LoadConfig 之后、模块 Init 之前调用一次。
func InitS3() error {
	cfg := AppConfig
	if cfg == nil || cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "" || cfg.S3Bucket == "" {
		log.Println("[s3] 未配置（缺 S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY/S3_BUCKET），跳过初始化")
		return nil
	}
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
	log.Printf("[s3] 已初始化  bucket=%s  endpoint=%s", cfg.S3Bucket, displayOrEmpty(cfg.S3Endpoint))
	return nil
}
