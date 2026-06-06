// Package accesslog 是接口访问日志模块的公开入口。
//
// 模块实现位于 modules/accesslog/internal，受 Go internal 机制保护。它通过框架的
// AccessLogMiddlewareFunc 注入全局访问日志中间件，并提供日志查询接口；不对其他业务
// 模块发布契约，公开包仅用于在 main 中 blank-import 以触发自注册。
package accesslog

// 导入内部实现以触发其 init()（注册模块、中间件与数据库迁移）。
import _ "manager-backend/modules/accesslog/internal"
