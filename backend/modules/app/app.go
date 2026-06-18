// Package app 是「应用库管理」业务模块的公开入口。
//
// 模块实现位于 modules/app/internal，受 Go internal 机制保护；公开包用于在 main 中
// blank-import 触发自注册，并对外导出少量门面（供 openapi 等读应用库 / 应用商店）。
// 应用库是租户全局资源（无 owner 隔离），全部透传云手机中台。
package app

import appinternal "manager-backend/modules/app/internal"

// CustomerApp 是应用绑定记录的公开别名（供 openapi 等引用返回类型）。
type CustomerApp = appinternal.CustomerApp

// List 返回某用户上传的应用（应用库，store=false）。
func List(userID int) ([]CustomerApp, error) {
	return appinternal.AppService.List(userID)
}

// StoreList 返回应用商店应用（store=true，面向全部用户）。
func StoreList() ([]CustomerApp, error) {
	return appinternal.AppService.StoreList()
}
