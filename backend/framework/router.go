package framework

import (
	"github.com/gin-gonic/gin"
)

// AuthMiddlewareFunc 认证中间件工厂（由 framework/user 包的 init() 注入，打破循环依赖）
var AuthMiddlewareFunc func() gin.HandlerFunc

// AccessLogMiddlewareFunc 访问日志中间件工厂（由业务模块 init() 注入）
var AccessLogMiddlewareFunc func() gin.HandlerFunc

// rootRouteRegistrars 是需要挂在引擎根（而非 /api/v1 下）的路由注册器。
// 供「开放 API」等需要独立基址（如 /api/open/v1）的场景用，避免出现 /api/v1/open/v1 这种双 v1。
var rootRouteRegistrars []func(*gin.Engine)

// RegisterRootRoutes 登记一个根级路由注册器（在 SetupRouter 装配 /api/v1 之后调用）。
// 通常在模块 init() 里调用。
func RegisterRootRoutes(fn func(*gin.Engine)) {
	rootRouteRegistrars = append(rootRouteRegistrars, fn)
}

// SetupRouter 设置路由（调用前需确保已 import _ "framework/user" 和 _ "业务modules" 触发注册）
func SetupRouter(r *gin.Engine) {
	apiV1 := r.Group("/api/v1")

	if AccessLogMiddlewareFunc != nil {
		apiV1.Use(AccessLogMiddlewareFunc())
	}

	var middlewares []gin.HandlerFunc
	if AuthMiddlewareFunc != nil {
		middlewares = append(middlewares, AuthMiddlewareFunc())
	}

	for _, module := range GlobalModule.GetAll() {
		module.RegisterRoutes(apiV1, middlewares...)
	}

	// 根级路由（独立基址，如开放 API /api/open/v1）。
	for _, fn := range rootRouteRegistrars {
		fn(r)
	}
}
