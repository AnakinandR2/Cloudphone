// Package phone 是「云手机实例」业务模块的公开入口与契约。
//
// 业务层维护云手机实例的归属与档案（业务库自有记录，含 owner/状态/绑定代理等），
// 按属主隔离：每台归属一个前台用户。前台用户增删改查自己的云手机；后台运营查看全量并可强制删除。
// 实例的实际开通/生命周期（调用云手机中台异步创建等）属后续阶段，本模块先承载档案与列表。
// 模块实现位于 modules/phone/internal，受 Go internal 机制保护。
package phone

import (
	"manager-backend/framework/midplat"
	phoneinternal "manager-backend/modules/phone/internal"
)

// CloudPhone 是云手机档案的公开别名（供 openapi 等跨模块引用返回类型）。
type CloudPhone = phoneinternal.CloudPhone

// 跨模块门面（供 automation / openapi 等调用；phone 不反向依赖这些模块）。

// OwnedCpIDs 返回某用户名下已开通（有中台 cpId）的全部 cpId。
func OwnedCpIDs(userID int) ([]string, error) {
	return phoneinternal.PhoneService.OwnedCpIDs(userID)
}

// OwnsCpID 校验某 cpId 是否属于该用户且已开通。
func OwnsCpID(userID int, cpID string) (bool, error) {
	return phoneinternal.PhoneService.OwnsCpID(userID, cpID)
}

// CpIDOf 返回某用户名下指定 phone 的中台 cpId；未开通则返回空串 + nil。
func CpIDOf(userID, id int) (string, error) {
	p, err := phoneinternal.PhoneService.GetByID(userID, id)
	if err != nil {
		return "", err
	}
	return p.CpID, nil
}

// ===== 开放 API 委托（均复用现有 service，归属/状态门禁已在内部）=====

// List 列出某用户的云手机（含中台实时状态）。
func List(userID, page, size int) ([]CloudPhone, int64, error) {
	return phoneinternal.PhoneService.GetList(userID, page, size, "", "", "", "", "")
}

// GetDisplay 取单台云手机（中台实时状态）。
func GetDisplay(userID, id int) (*CloudPhone, error) {
	return phoneinternal.PhoneService.GetByIDDisplay(userID, id)
}

// Create 创建一台云手机。
func Create(userID int, name, region, imageID string, proxyID uint) (*CloudPhone, error) {
	return phoneinternal.PhoneService.Create(userID, &phoneinternal.CloudPhoneCreate{
		Name: name, Region: region, ImageID: imageID, ProxyID: proxyID,
	})
}

// Destroy 销毁云手机（带状态门禁，并移除本地档案）。
func Destroy(userID, id int) error {
	return phoneinternal.PhoneService.Destroy(userID, id)
}

// Power 开机 / 关机（operation: 开机 / 关机）。
func Power(userID, id int, operation string) error {
	return phoneinternal.PhoneService.Power(userID, id, operation)
}

// Restart 重启。
func Restart(userID, id int) error {
	return phoneinternal.PhoneService.Restart(userID, id)
}

// BindProxy 给某台云手机绑定/改绑代理（proxyID>0）。
func BindProxy(userID, id int, proxyID uint) error {
	_, err := phoneinternal.PhoneService.Update(userID, id, &phoneinternal.CloudPhoneUpdate{ProxyID: proxyID})
	return err
}

// InstalledApps 已装应用列表。
func InstalledApps(userID, id int) ([]midplat.InstalledApp, error) {
	return phoneinternal.PhoneService.InstalledApps(userID, id)
}

// InstallApp 安装应用（按中台 appId）。
func InstallApp(userID, id int, appIDs []int64) error {
	return phoneinternal.PhoneService.InstallApp(userID, id, appIDs)
}

// UninstallApp 卸载应用（按 appId 或包名）。
func UninstallApp(userID, id int, appIDs []int64, pkgs []string) error {
	return phoneinternal.PhoneService.UninstallApp(userID, id, appIDs, pkgs)
}
