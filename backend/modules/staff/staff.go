// Package user 是「身份与访问」模块对外发布的公开契约（Modulith published API）。
//
// 模块的全部实现位于 modules/user/internal，受 Go internal 机制保护——其他业务模块
// 无法 import 其内部实现，只能依赖本文件再导出的少量契约。这是模块边界的编译期硬保证。
//
// 目前对外仅发布鉴权中间件：其他模块在注册路由时据此声明所需权限。
package staff

import (
	staffint "manager-backend/modules/staff/internal"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware 基于角色权限的中间件：拥有任一所需权限或为超管即放行。
// 需配合框架注入的 AuthMiddleware 使用。
func PermissionMiddleware(requiredPerms ...string) gin.HandlerFunc {
	return staffint.PermissionMiddleware(requiredPerms...)
}

// SuperuserMiddleware 仅超级管理员可访问。
func SuperuserMiddleware() gin.HandlerFunc {
	return staffint.SuperuserMiddleware()
}
