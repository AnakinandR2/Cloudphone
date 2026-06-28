package phone

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/modules/app"
)

// AppRefInput 是按 URL 安装请求里的单个应用引用（source:'user'|'market', id）。
type AppRefInput struct {
	Source string `json:"source"`
	ID     uint   `json:"id"`
}

// resolveInstallSpecs 是对 app 公开门面的可替换接缝（测试注入桩，绕过 app 内部 DB/S3）。
// 生产指向 app.ResolveInstallSpecs：把应用引用解析为按 URL 安装载荷（私有桶 presigned / 公有桶永久 URL）。
var resolveInstallSpecs = func(userID int, refs []app.AppRef) ([]app.InstallSpec, error) {
	return app.ResolveInstallSpecs(userID, refs)
}

// AppInstallTask 是单台云手机的安装任务受理结果（透出中台 taskId ↔ instanceId）。
type AppInstallTask struct {
	TaskID     string `json:"task_id"`
	InstanceID string `json:"instance_id"`
}

// InstallByURL 按 URL 安装应用到一台或多台云手机（§7.3，替代旧的按中台 appId 安装）。
//
// 流程：
//  1. 校验 phone_ids / apps 非空。
//  2. 逐个手机属主校验（resolveCp）：任一不属主/未开通 → 整请求前置失败，不下发任何手机。
//  3. 经 app 门面 ResolveInstallSpecs(uid, refs) 把应用引用解析为按 URL 安装载荷
//     （任一未就绪/不存在/锁定 → 整请求失败）。
//  4. 一次 midplat.InstallAppByURL({cpIds, apps}) 批量下发，返回中台 taskInfoList。
func (s *serviceImpl) InstallByURL(userID int, phoneIDs []int, refs []AppRefInput) ([]AppInstallTask, error) {
	if len(phoneIDs) == 0 {
		return nil, apperr.BadRequest("未选择云手机")
	}
	if len(refs) == 0 {
		return nil, apperr.BadRequest("未选择应用")
	}

	// 1) 逐个手机属主校验 + 收集 cpId（任一不属主/未开通即整请求失败，无任何副作用）。
	cpIDs := make([]string, 0, len(phoneIDs))
	for _, id := range phoneIDs {
		p, err := s.resolveCp(userID, id)
		if err != nil {
			return nil, err
		}
		cpIDs = append(cpIDs, p.CpID)
	}

	// 2) 解析应用引用为按 URL 安装载荷（私有桶 presigned / 公有桶永久 URL）。
	appRefs := make([]app.AppRef, 0, len(refs))
	for _, r := range refs {
		appRefs = append(appRefs, app.AppRef{Source: r.Source, ID: r.ID})
	}
	specs, err := resolveInstallSpecs(userID, appRefs)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, apperr.BadRequest("无可安装的应用")
	}

	// 3) 映射为中台 install-by-url 载荷并一次性下发。
	apps := make([]midplat.InstallByURLApp, 0, len(specs))
	for _, sp := range specs {
		apps = append(apps, midplat.InstallByURLApp{
			AppName:     sp.AppName,
			DownloadURL: sp.DownloadURL,
			MD5:         sp.MD5,
			PackageName: sp.PackageName,
			Version:     sp.Version,
			FileSize:    sp.FileSize,
		})
	}

	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.InstallAppByURL(ctx, midplat.InstallByURLRequest{CpIDs: cpIDs, Apps: apps})
	if err != nil {
		return nil, err
	}

	tasks := make([]AppInstallTask, 0)
	if res != nil {
		for _, t := range res.TaskInfoList {
			tasks = append(tasks, AppInstallTask{TaskID: t.TaskID, InstanceID: t.InstanceID})
		}
	}
	return tasks, nil
}
