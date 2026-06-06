// Package user 是前台用户模块的公开入口与契约。
//
// 它与后台员工模块 modules/staff 是两个独立的限界上下文：独立的 users 表、独立的
// 注册/登录流程、独立的 JWT 身份域（scope=user）。前台令牌无法用于后台接口，反之亦然。
//
// 对外发布前台鉴权中间件 AuthMiddleware，供其他「前台风格」模块复用（要求 user 登录）。
// 模块实现位于 modules/user/internal，受 Go internal 机制保护。
package user

import (
	userint "manager-backend/modules/user/internal"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 前台鉴权中间件：要求 user 身份域登录（后台 staff 令牌会被拒绝）。
func AuthMiddleware() gin.HandlerFunc {
	return userint.UserAuth()
}
