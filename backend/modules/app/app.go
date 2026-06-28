// Package app 是「应用管理 / 应用市场」业务模块的公开入口。
//
// 模块实现位于 modules/app/internal，受 Go internal 机制保护；公开包用于在 main 中
// blank-import 触发自注册，并对外导出少量门面：
//   - 安装载荷解析 ResolveInstallSpecs（供 phone 模块按 URL 安装组载荷）；
//   - 应用清单 ListUserApps / ListMarketApps（供 openapi / mcp 列「可安装应用」）。
//
// 用户应用 = 素材库文件（占配额，私有桶）；市场应用 = 平台资产（公有桶，不计配额）。
// 二进制全部存我们自己的 S3，元数据我们自己解析，安装时只给中台一个 URL。
package app

import appinternal "manager-backend/modules/app/internal"

// AppRef 是一个待安装应用的引用。Source: "user"(=library_file_id) | "market"(=app_market.id)。
type AppRef struct {
	Source string
	ID     uint
}

// InstallSpec 是按 URL 安装的单个应用载荷（直接映射中台 install-by-url 的 app 字段）。
type InstallSpec struct {
	AppName     string
	DownloadURL string
	MD5         string
	PackageName string
	Version     string
	FileSize    string
}

// ResolveInstallSpecs 把应用引用解析为按 URL 安装载荷（见 §7.2）：
//   - user：校验 app_user_meta.parse_status=ready → library.OpenFileWithTTL(installTTL) 取 presigned GET；
//   - market：app_market(ready) 行 → DownloadURL = PublicBaseURL + "/" + S3Key（公有永久可达）。
//
// 任一未就绪 / 不存在 / 锁定 → 整请求失败（属主与超额锁定校验由 library 门面保证）。
func ResolveInstallSpecs(userID int, refs []AppRef) ([]InstallSpec, error) {
	in := make([]appinternal.AppRef, 0, len(refs))
	for _, r := range refs {
		in = append(in, appinternal.AppRef{Source: r.Source, ID: r.ID})
	}
	specs, err := appinternal.AppService.ResolveInstallSpecs(userID, in)
	if err != nil {
		return nil, err
	}
	out := make([]InstallSpec, 0, len(specs))
	for _, s := range specs {
		out = append(out, InstallSpec(s))
	}
	return out, nil
}

// AppListItem 是「可安装应用」清单条目（供 openapi / mcp 列我的应用 + 应用市场）。
//   - Ref 即安装时传给 ResolveInstallSpecs 的引用；
//   - Store=true 表示市场应用；ParseStatus 为 parsing|ready|failed。
type AppListItem struct {
	Ref         AppRef
	Name        string
	PackageName string
	Version     string
	Store       bool
	ParseStatus string
}

// ListUserApps 列出某用户的「我的应用」（素材库 app 文件 + 解析元数据）。
func ListUserApps(userID int) ([]AppListItem, error) {
	list, err := appinternal.AppService.ListUserApps(userID, 1, 500)
	if err != nil {
		return nil, err
	}
	out := make([]AppListItem, 0, len(list))
	for _, a := range list {
		out = append(out, AppListItem{
			Ref:         AppRef{Source: "user", ID: a.FileID},
			Name:        a.AppName,
			PackageName: a.PackageName,
			Version:     a.Version,
			Store:       false,
			ParseStatus: a.ParseStatus,
		})
	}
	return out, nil
}

// ListMarketApps 列出应用市场（仅就绪），供用户端 / 开放接口浏览安装。
func ListMarketApps() ([]AppListItem, error) {
	list, err := appinternal.AppService.ListMarket(true)
	if err != nil {
		return nil, err
	}
	out := make([]AppListItem, 0, len(list))
	for _, a := range list {
		out = append(out, AppListItem{
			Ref:         AppRef{Source: "market", ID: a.ID},
			Name:        a.AppName,
			PackageName: a.PackageName,
			Version:     a.Version,
			Store:       true,
			ParseStatus: a.ParseStatus,
		})
	}
	return out, nil
}
