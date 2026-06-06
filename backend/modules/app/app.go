// Package app 是「应用库管理」业务模块的公开入口。
//
// 模块实现位于 modules/app/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。应用库是租户全局资源（无 owner 隔离），全部透传云手机中台。
package app

import _ "manager-backend/modules/app/internal"
