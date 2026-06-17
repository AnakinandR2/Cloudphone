package app

import (
	"context"
	"io"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
)

// midplatPort 是 app 模块对云手机中台「应用域」的依赖端口（六边形出站端口）。
// 封装应用库能力：列表 / 批量删除 / 从本地文件整链上传，以及浏览器驱动的分片上传流水线
// （initiate → part → complete → parse → create → status，参考 mcn 的逐片上传交互）。
type midplatPort interface {
	ListApps(ctx context.Context, req midplat.ListAppsRequest) (*midplat.ListAppsResponse, error)
	BatchDeleteApps(ctx context.Context, ids []int64) error
	// UploadAppFromFile 编排：分片上传(秒传) → 合并 → 解析 APK → 创建 app_info（服务端整链，给后台商店用）。
	UploadAppFromFile(ctx context.Context, path string, opts midplat.UploadAppOptions) (*midplat.CreatedApp, error)

	// 浏览器驱动的分片上传流水线（前台 my 用：浏览器切片、逐片上传，进度更细、支持秒传）。
	InitiateAppUpload(ctx context.Context, req midplat.InitiateUploadRequest) (*midplat.InitiateUploadResponse, error)
	UploadPart(ctx context.Context, uploadID int64, partNumber int, contentMD5 string, part io.Reader, fileName string) (*midplat.UploadPartResponse, error)
	CompleteAppUpload(ctx context.Context, uploadID int64) (string, error)
	GetAppInfoFromFile(ctx context.Context, uploadID int64) (*midplat.ParsedAppInfo, error)
	CreateAppFromUploadedFile(ctx context.Context, req midplat.CreateFromUploadedFileRequest) (*midplat.CreatedApp, error)
	QueryUploadStatus(ctx context.Context, uploadID int64) (string, error)
}

type sdkAdapter struct{ c *midplat.Client }

func (a *sdkAdapter) ListApps(ctx context.Context, req midplat.ListAppsRequest) (*midplat.ListAppsResponse, error) {
	return a.c.ListApps(ctx, req)
}

func (a *sdkAdapter) BatchDeleteApps(ctx context.Context, ids []int64) error {
	return a.c.BatchDeleteApps(ctx, ids)
}

func (a *sdkAdapter) UploadAppFromFile(ctx context.Context, path string, opts midplat.UploadAppOptions) (*midplat.CreatedApp, error) {
	return a.c.UploadAppFromFile(ctx, path, opts)
}

func (a *sdkAdapter) InitiateAppUpload(ctx context.Context, req midplat.InitiateUploadRequest) (*midplat.InitiateUploadResponse, error) {
	return a.c.InitiateAppUpload(ctx, req)
}

func (a *sdkAdapter) UploadPart(ctx context.Context, uploadID int64, partNumber int, contentMD5 string, part io.Reader, fileName string) (*midplat.UploadPartResponse, error) {
	return a.c.UploadPart(ctx, uploadID, partNumber, contentMD5, part, fileName)
}

func (a *sdkAdapter) CompleteAppUpload(ctx context.Context, uploadID int64) (string, error) {
	return a.c.CompleteAppUpload(ctx, uploadID)
}

func (a *sdkAdapter) GetAppInfoFromFile(ctx context.Context, uploadID int64) (*midplat.ParsedAppInfo, error) {
	return a.c.GetAppInfoFromFile(ctx, uploadID)
}

func (a *sdkAdapter) CreateAppFromUploadedFile(ctx context.Context, req midplat.CreateFromUploadedFileRequest) (*midplat.CreatedApp, error) {
	return a.c.CreateAppFromUploadedFile(ctx, req)
}

func (a *sdkAdapter) QueryUploadStatus(ctx context.Context, uploadID int64) (string, error) {
	return a.c.QueryUploadStatus(ctx, uploadID)
}

// newMidplatPort 从 framework.AppConfig 构造真实适配器；中台未配置时返回 nil。
func newMidplatPort() midplatPort {
	cfg := framework.AppConfig
	if cfg == nil || cfg.MidplatBaseURL == "" || cfg.MidplatAccessKey == "" || cfg.MidplatSecretKey == "" {
		return nil
	}
	c, err := midplat.New(midplat.Config{
		BaseURL:      cfg.MidplatBaseURL,
		AccessKey:    cfg.MidplatAccessKey,
		SecretKey:    cfg.MidplatSecretKey,
		TenantUID:    cfg.MidplatTenantUID,
		OperatorName: cfg.MidplatOperatorName,
		HTTPTimeout:  300 * time.Second, // 上限放宽以容纳 APK 上传；列表等快接口由各自 opCtx 控制
	})
	if err != nil {
		return nil
	}
	return &sdkAdapter{c: c}
}

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// opCtxUpload 给整链上传更长的超时（APK 较大、分片+合并耗时）。
func opCtxUpload() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 300*time.Second)
}
