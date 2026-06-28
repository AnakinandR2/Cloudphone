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

// IDByPhone 精确按手机号查前台用户 ID。phone 为空或查不到返回 (0, false, nil)。
// 供其他模块（如 billing 按手机号过滤订单）使用，避免直接查 users 表。
func IDByPhone(phone string) (uint, bool, error) {
	return userint.Service.IDByPhone(phone)
}

// PhonesByIDs 批量按前台用户 ID 取手机号，组装成 map[id]phone（缺失的 ID 不在 map 中）。
// 供其他模块（如 billing 订单列表回填手机号）使用。空 ids 返回空 map。
func PhonesByIDs(ids []uint) (map[uint]string, error) {
	return userint.Service.PhonesByIDs(ids)
}
