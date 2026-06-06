package cloudphone

import (
	"context"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
)

// client 是本模块持有的云手机中台客户端；未配置中台时为 nil。
var client *midplat.Client

// initClient 根据 framework.AppConfig 构造中台客户端。
//
// 三项配置（BaseURL/AccessKey/SecretKey）缺任一则不构造（client 保持 nil），
// 相关接口会返回「中台未配置」错误，但不影响其它模块与测试。
func initClient() {
	cfg := framework.AppConfig
	if cfg == nil || cfg.MidplatBaseURL == "" || cfg.MidplatAccessKey == "" || cfg.MidplatSecretKey == "" {
		return
	}
	c, err := midplat.New(midplat.Config{
		BaseURL:     cfg.MidplatBaseURL,
		AccessKey:   cfg.MidplatAccessKey,
		SecretKey:   cfg.MidplatSecretKey,
		HTTPTimeout: 30 * time.Second,
	})
	if err != nil {
		return
	}
	client = c
}

// callCtx 给每次中台调用一个独立超时上下文。
func callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
