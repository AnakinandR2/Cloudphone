package openapi

import (
	"manager-backend/framework"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type openapiModule struct{}

func (m *openapiModule) Name() string { return "openapi" }

func (m *openapiModule) Init(db *gorm.DB) error {
	Service = newService(newRepository(db))
	return nil
}

func (m *openapiModule) RegisterRoutes(router *gin.RouterGroup, _ ...gin.HandlerFunc) {
	// 密钥管理：前台用户自助签发/撤销，需 user 登录（JWT），挂在 /api/v1 下。
	keys := router.Group("/user/api-keys")
	keys.Use(user.AuthMiddleware())
	{
		keys.GET("", ListKeys)
		keys.POST("", CreateKey)
		keys.GET("/:id/reveal", RevealKey)
		keys.POST("/:id/revoke", RevokeKey)
	}
}

// registerOpenRoutes 把开放 API 挂在引擎根的独立基址 /api/open/v1（不在 /api/v1 下，避免双 v1）。
func registerOpenRoutes(r *gin.Engine) {
	open := r.Group("/api/open/v1")
	open.Use(keyAuthMiddleware())
	{
		open.GET("/phones", OpenListPhones)
		open.POST("/phones", OpenCreatePhone)
		open.GET("/phones/:id", OpenGetPhone)
		open.DELETE("/phones/:id", OpenDestroyPhone)
		open.POST("/phones/:id/power", OpenPower)
		open.POST("/phones/:id/restart", OpenRestart)
		open.GET("/apps", OpenListApps)
		open.GET("/scripts", OpenListScripts)
		open.GET("/proxies", OpenListProxies)
		open.POST("/proxies", OpenCreateProxy)
		open.PUT("/proxies/:id", OpenUpdateProxy)
		open.DELETE("/proxies/:id", OpenDeleteProxy)
		open.POST("/phones/:id/proxy", OpenBindProxy)
		open.GET("/phones/:id/apps", OpenApps)
		open.POST("/phones/:id/apps/install", OpenInstall)
		open.POST("/phones/:id/apps/uninstall", OpenUninstall)
		open.POST("/phones/:id/run-script", OpenRunScript)
		open.GET("/tasks/:taskId", OpenTaskDetail)
	}
}

func (m *openapiModule) OnStart() error { return nil }
func (m *openapiModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&openapiModule{})
	// 开放 API 挂在引擎根的独立基址 /api/open/v1。
	framework.RegisterRootRoutes(registerOpenRoutes)
	// 建表（幂等）：API 密钥表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&APIKey{})
	})
}
