package phone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
)

// CreateArgs 是「异步创建一台云手机」的入参（业务侧已知信息；vmId/imageId 可留空由适配层调度）。
type CreateArgs struct {
	Region  string
	ImageID string
	VmID    string
}

// CreateResult 是创建受理结果：中台分配的 cpId + 适配层最终选定的 vmId/imageId/region（回填本地档案）。
type CreateResult struct {
	CpID    string
	VmID    string
	ImageID string
	Region  string
}

// midplatPort 是 phone 模块对「云手机中台」的依赖端口（六边形架构里的出站端口）。
//
// 之所以抽成接口：① 业务服务只依赖它、不直接依赖 SDK，符合 DDD 的依赖倒置；
// ② 单测可注入假实现，无需真打中台 / mock HTTP 即可验证「归属校验 + 透传正确 cpId」。
// 所有方法都按「单台云手机」语义封装（SDK 多为批量接口，这里只传一台）。
type midplatPort interface {
	// Create 异步创建一台云手机，返回中台分配的 cpId 与最终选定的资源信息（vmId/imageId/region）。
	Create(ctx context.Context, args CreateArgs) (*CreateResult, error)

	StartOrShutdown(ctx context.Context, cpID, operation string) error
	Restart(ctx context.Context, cpID string) error
	Reset(ctx context.Context, cpID, imageID string) error
	Destroy(ctx context.Context, cpID string) error
	RefreshPhone(ctx context.Context, cpID string) error

	WebRTCAuth(ctx context.Context, cpID string) (*midplat.WebRTCAuthInfo, error)
	WebRTCState(ctx context.Context, cpID string) (bool, error)

	Screenshot(ctx context.Context, cpID, format string) error
	SetVolume(ctx context.Context, cpID string, volume int) error
	Rotate(ctx context.Context, cpID, orientation string) error
	Shake(ctx context.Context, cpID string) error

	// 文件管理（需 vmID + cpID）：列目录 / 流式下载 / 批量上传。
	FileList(ctx context.Context, vmID, cpID, path string) ([]midplat.PhoneFile, error)
	FileDownload(ctx context.Context, vmID, cpID, path string) ([]byte, error)
	FileUpload(ctx context.Context, vmID, cpID, folderPath string, files []midplat.UploadFile) error
	FileDelete(ctx context.Context, vmID, cpID, path string) error

	InstalledApps(ctx context.Context, cpID string) ([]midplat.InstalledApp, error)
	InstallApp(ctx context.Context, cpID string, appIDs []int64) error
	UninstallApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error
	StartApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error
	StopApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error
	KillAllApps(ctx context.Context, cpID string) error

	// ADB Token 接管（spec v3.25.9 §3.5）。enable 签发 token+地址，disable 吊销，§2.6 补过期时间。
	AdbEnableToken(ctx context.Context, cpID string) (*midplat.ADBTokenContainer, error)
	AdbDisableToken(ctx context.Context, cpID string) error
	AdbInfo(ctx context.Context, cpID string) (*midplat.CloudPhoneAdbInfo, error)

	// Statuses 批量查询云手机的实时状态（cpId → status），供列表用中台真实状态覆盖本地档案。
	Statuses(ctx context.Context, cpIDs []string) (map[string]string, error)

	// AdbEnabledMap 批量查询哪些 cp 已开 ADB（cpId → 是否有 adbToken），供列表标记。
	AdbEnabledMap(ctx context.Context, cpIDs []string) (map[string]bool, error)

	// Root 切换单台云手机的 root 权限（enable=true 开启 / false 关闭）。需 vmID + cpID。
	Root(ctx context.Context, vmID, cpID string, enable bool) error

	// RootEnabledMap 批量查询哪些 cp 已 root（cpId → isRooted），供列表标记 + 操作门禁。
	RootEnabledMap(ctx context.Context, cpIDs []string) (map[string]bool, error)

	// RunLogs 分页查询某台云手机的运行会话日志（spec §2.9）。
	RunLogs(ctx context.Context, cpID string, page, size int) (*midplat.RunLogPage, error)
}

// sdkAdapter 用真实 midplat.Client 实现 midplatPort（把单台调用包成 SDK 的批量入参）。
type sdkAdapter struct{ c *midplat.Client }

// Create 异步创建一台云手机，按官方控制台（cp-glory-service）验证过的级联自动调度：
// 选一台有空闲容量的在线服务器(server/page) → 按其规格查套餐(plan) → 查套餐镜像 → 组装
// cp/create 必填参数（vmId(UUID)/planId/imageId + 资源约束 + 启动参数）。
// 注：基础流程不下发代理（proxyMode 留空 = 无代理；代理校验属后续阶段）。
func (a *sdkAdapter) Create(ctx context.Context, args CreateArgs) (*CreateResult, error) {
	srv, err := a.pickServer(ctx)
	if err != nil {
		return nil, err
	}
	plans, err := a.c.ListBootPlansBySpec(ctx, srv.SpecificationID)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("规格 %d 无可用套餐", srv.SpecificationID)
	}
	plan := plans[0]
	imgs, err := a.c.ListImagesByPlan(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	if len(imgs) == 0 {
		return nil, fmt.Errorf("套餐 %d 无可用镜像", plan.ID)
	}
	imageID := imgs[0].ImageID
	bpID, w, h, fps, mbc := bootParamsOf(plan)
	zero := 0

	// 套餐里 min/maxCore、min/maxMemory、storage、bandwidth 才是「单台云手机」的资源；
	// specCore/specMemory/specStorage/specBandwidth 是宿主云主机的总量，不能拿来当手机规格。
	// 映射到 §2.1 cp/create 的 minCore/maxCore/minMemory/maxMemory/storage/bandwidth。
	minCore, maxCore := plan.MinCore, plan.MaxCore
	if minCore == 0 {
		minCore = float64(plan.Core)
	}
	if maxCore == 0 {
		maxCore = float64(plan.Core)
	}
	minMem, maxMem := plan.MinMemory, plan.MaxMemory
	if minMem == 0 {
		minMem = plan.Memory
	}
	if maxMem == 0 {
		maxMem = plan.Memory
	}
	storage := plan.Storage
	bandwidth := plan.Bandwidth
	// 资源利用模式以套餐为准；为空才兜底 SHARED（避免中台对 null 拆箱 NPE）。
	resourceUtil := plan.ResourceUtilization
	if resourceUtil == "" {
		resourceUtil = "SHARED"
	}

	resp, err := a.c.CreateCloudPhones(ctx, midplat.CreateCPRequest{
		Number:              1,
		NeedStart:           false, // 创建后默认不开机
		VmID:                srv.VmID,
		ImageID:             imageID,
		PlanID:              plan.ID,
		MinCore:             minCore,
		MaxCore:             maxCore,
		MinMemory:           minMem,
		MaxMemory:           maxMem,
		Storage:             storage,
		Bandwidth:           bandwidth,
		BootParamID:         bpID,
		ScenarioID:          &zero,
		NetworkType:         &zero,
		Width:               w,
		Height:              h,
		FPS:                 fps,
		MaxBootCount:        mbc,
		ResourceUtilization: resourceUtil,
		AndroidVersion:      imgs[0].AndroidVersion,
		RegionOption:        "CUSTOM",
		TimezoneOption:      "CUSTOM",
		LanguageOption:      "CUSTOM",
		Region:              args.Region,
	})
	if err != nil {
		return nil, err
	}
	ids := resp.IDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("中台未返回云手机 ID")
	}
	return &CreateResult{CpID: ids[0], VmID: srv.VmID, ImageID: imageID, Region: args.Region}, nil
}

// pickServer 选一台「在线、非维护、有空闲云手机容量」的服务器用于创建。
// 不按 vmStatusList 服务端过滤（该过滤会异常返回空），改为取全量后本地筛选。
func (a *sdkAdapter) pickServer(ctx context.Context) (*midplat.Server, error) {
	servers, err := a.c.ListServers(ctx, midplat.ListServersRequest{})
	if err != nil {
		return nil, err
	}
	for i := range servers {
		s := servers[i]
		if s.IsMaintain || s.VmStatus != midplat.VMStatusOnline {
			continue
		}
		if s.MaxPhone == 0 || s.CreatePhoneNumber < s.MaxPhone {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("没有可用的服务器用于创建云手机（在线且有空闲容量）")
}

// bootParamsOf 取套餐的启动参数（优先 bootParamsList[0]，否则回退套餐顶层字段）。
// §5.6 套餐顶层无 maxBootCount，仅 bootParamsList 内有；无启动参数变体时回退 0。
func bootParamsOf(p midplat.BootPlan) (bootParamID, width, height, fps, maxBootCount int) {
	if len(p.BootParamsList) > 0 {
		bp := p.BootParamsList[0]
		return bp.ID, bp.Width, bp.Height, bp.FPS, bp.MaxBootCount
	}
	return 0, p.Width, p.Height, p.FPS, 0
}

// StartOrShutdown 按操作路由到中台的两个独立端点：开机 → cp/start，关机 → cp/shutdown。
func (a *sdkAdapter) StartOrShutdown(ctx context.Context, cpID, operation string) error {
	if operation == "关机" {
		return a.c.ShutdownCloudPhones(ctx, []string{cpID})
	}
	return a.c.StartCloudPhones(ctx, []string{cpID})
}

func (a *sdkAdapter) Restart(ctx context.Context, cpID string) error {
	return a.c.RestartCloudPhones(ctx, []string{cpID})
}

func (a *sdkAdapter) Reset(ctx context.Context, cpID, imageID string) error {
	return a.c.BatchResetCloudPhones(ctx, []midplat.PhoneResetOperation{{CpID: cpID, ImageID: imageID}})
}

func (a *sdkAdapter) Destroy(ctx context.Context, cpID string) error {
	return a.c.DestroyCloudPhones(ctx, []string{cpID})
}

func (a *sdkAdapter) RefreshPhone(ctx context.Context, cpID string) error {
	// 一键新机：保留代理绑定（keepBindAgent=true）。
	return a.c.RefreshCloudPhones(ctx, []string{cpID}, true)
}

func (a *sdkAdapter) WebRTCAuth(ctx context.Context, cpID string) (*midplat.WebRTCAuthInfo, error) {
	list, err := a.c.AuthWebRTC(ctx, []string{cpID})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

func (a *sdkAdapter) WebRTCState(ctx context.Context, cpID string) (bool, error) {
	list, err := a.c.QueryWebRTCState(ctx, []string{cpID})
	if err != nil {
		return false, err
	}
	for _, s := range list {
		if s.ContainerID == cpID {
			return s.InWebRTC, nil
		}
	}
	return false, nil
}

func (a *sdkAdapter) Screenshot(ctx context.Context, cpID, format string) error {
	if format == "" {
		format = "png"
	}
	return a.c.ScreenShot(ctx, midplat.ScreenShotRequest{
		Containers: []string{cpID},
		Path:       "/data/local/tmp/screenshoots/" + cpID + "." + format,
		Format:     format,
	})
}

func (a *sdkAdapter) SetVolume(ctx context.Context, cpID string, volume int) error {
	return a.c.UpdateVolume(ctx, midplat.UpdateVolumeRequest{Containers: []string{cpID}, Volume: volume})
}

func (a *sdkAdapter) Rotate(ctx context.Context, cpID, orientation string) error {
	return a.c.ScreenRotate(ctx, midplat.ScreenRotateRequest{Containers: []string{cpID}, Orientation: orientation})
}

func (a *sdkAdapter) Shake(ctx context.Context, cpID string) error {
	return a.c.Shake(ctx, midplat.ShakeRequest{Containers: []string{cpID}})
}

func (a *sdkAdapter) FileList(ctx context.Context, vmID, cpID, path string) ([]midplat.PhoneFile, error) {
	return a.c.ListPhoneFiles(ctx, vmID, cpID, path)
}

func (a *sdkAdapter) FileDownload(ctx context.Context, vmID, cpID, path string) ([]byte, error) {
	return a.c.DownloadPhoneFile(ctx, vmID, cpID, path)
}

func (a *sdkAdapter) FileUpload(ctx context.Context, vmID, cpID, folderPath string, files []midplat.UploadFile) error {
	// 中台 batch-upload 的 folderPath 是相对「外部存储根 /sdcard」的路径（绝对路径报 PARAMETER_NOT_SUPPORT）。
	// 去掉 /sdcard 前缀转成相对：/sdcard/Download -> Download；/sdcard -> ""（空则中台落默认 /sdcard/Download）。
	rel := strings.TrimPrefix(folderPath, "/sdcard")
	rel = strings.TrimPrefix(rel, "/")
	return a.c.BatchUploadPhoneFiles(ctx, vmID, cpID, rel, true, files)
}

func (a *sdkAdapter) FileDelete(ctx context.Context, vmID, cpID, path string) error {
	return a.c.DeletePhoneFile(ctx, vmID, cpID, path)
}

func (a *sdkAdapter) InstalledApps(ctx context.Context, cpID string) ([]midplat.InstalledApp, error) {
	list, err := a.c.GetInstalledApps(ctx, []string{cpID})
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if e.CpID == cpID {
			return e.Apps, nil
		}
	}
	if len(list) > 0 {
		return list[0].Apps, nil
	}
	return nil, nil
}

func (a *sdkAdapter) InstallApp(ctx context.Context, cpID string, appIDs []int64) error {
	_, err := a.c.InstallApp(ctx, midplat.InstallAppRequest{CpIDs: []string{cpID}, AppIDs: appIDs})
	return err
}

func (a *sdkAdapter) UninstallApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error {
	return a.c.UninstallApp(ctx, midplat.UninstallAppRequest{CpIDs: []string{cpID}, AppIDs: appIDs, PackageNames: pkgs})
}

func (a *sdkAdapter) StartApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error {
	_, err := a.c.StartApp(ctx, midplat.StartStopAppRequest{CpIDs: []string{cpID}, AppIDs: appIDs, PackageNames: pkgs})
	return err
}

func (a *sdkAdapter) StopApp(ctx context.Context, cpID string, appIDs []int64, pkgs []string) error {
	_, err := a.c.StopApp(ctx, midplat.StartStopAppRequest{CpIDs: []string{cpID}, AppIDs: appIDs, PackageNames: pkgs})
	return err
}

func (a *sdkAdapter) KillAllApps(ctx context.Context, cpID string) error {
	_, err := a.c.KillAllApps(ctx, []string{cpID})
	return err
}

func (a *sdkAdapter) AdbEnableToken(ctx context.Context, cpID string) (*midplat.ADBTokenContainer, error) {
	// 默认有效期 1 天（validTime 单位为天）。注意：实测中台当前忽略此值、固定 +60 天，
	// 这里按业务期望显式下发 1，待中台修复后即生效。
	res, err := a.c.EnableADBToken(ctx, midplat.ADBTokenRequest{Containers: []string{cpID}, ValidTime: 1})
	if err != nil {
		return nil, err
	}
	if res == nil || len(res.Data.Containers) == 0 {
		return nil, nil
	}
	return &res.Data.Containers[0], nil
}

func (a *sdkAdapter) AdbDisableToken(ctx context.Context, cpID string) error {
	_, err := a.c.DisableADBToken(ctx, midplat.ADBTokenRequest{Containers: []string{cpID}})
	return err
}

func (a *sdkAdapter) AdbEnabledMap(ctx context.Context, cpIDs []string) (map[string]bool, error) {
	return a.c.BatchQueryAdbEnabled(ctx, cpIDs)
}

func (a *sdkAdapter) Root(ctx context.Context, vmID, cpID string, enable bool) error {
	return a.c.UpdateRoot(ctx, midplat.UpdateRootRequest{VmID: vmID, Containers: []string{cpID}, Root: enable})
}

func (a *sdkAdapter) RootEnabledMap(ctx context.Context, cpIDs []string) (map[string]bool, error) {
	return a.c.BatchQueryRootEnabled(ctx, cpIDs)
}

func (a *sdkAdapter) RunLogs(ctx context.Context, cpID string, page, size int) (*midplat.RunLogPage, error) {
	return a.c.QueryRunLogs(ctx, midplat.RunLogQueryRequest{Page: page, PageSize: size, CpID: cpID})
}

func (a *sdkAdapter) AdbInfo(ctx context.Context, cpID string) (*midplat.CloudPhoneAdbInfo, error) {
	return a.c.GetCloudPhoneAdbInfo(ctx, cpID)
}

func (a *sdkAdapter) Statuses(ctx context.Context, cpIDs []string) (map[string]string, error) {
	list, err := a.c.BatchQueryStatus(ctx, cpIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, s := range list {
		out[s.CpID] = s.Status
	}
	return out, nil
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
		HTTPTimeout:  120 * time.Second, // 上限放宽以容纳文件上传；普通操作仍由各自 opCtx(30s) 控制
	})
	if err != nil {
		return nil
	}
	return &sdkAdapter{c: c}
}

// opCtx 给每次中台操作一个独立超时上下文。
func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// opCtxUpload 给文件上传更长的超时（上传耗时随文件大小增长）。
func opCtxUpload() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 120*time.Second)
}
