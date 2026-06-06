package framework

import (
	"github.com/gin-gonic/gin"
)

// AuthMiddlewareFunc 认证中间件工厂（由 framework/user 包的 init() 注入，打破循环依赖）
var AuthMiddlewareFunc func() gin.HandlerFunc

// AccessLogMiddlewareFunc 访问日志中间件工厂（由业务模块 init() 注入）
var AccessLogMiddlewareFunc func() gin.HandlerFunc

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
}
