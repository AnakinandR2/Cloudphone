// Package example 是示例业务模块的公开入口。
//
// 模块实现位于 modules/example/internal，受 Go internal 机制保护。本模块不对其他
// 模块发布任何契约，公开包仅用于在 main 中 blank-import 以触发自注册。
package example

// 导入内部实现以触发其 init()（注册模块与数据库迁移）。
import _ "manager-backend/modules/example/internal"
