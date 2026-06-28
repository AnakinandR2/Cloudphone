package app

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type appModule struct{}

func (m *appModule) Name() string { return "app" }

// Init 装配服务（app_user_meta / app_market 仓库）。
func (m *appModule) Init(db *gorm.DB) error {
	AppService = newService(newRepository(db))
	return nil
}

func (m *appModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 用户端：我的应用（素材库 app 文件 + 旁挂元数据）+ 应用市场浏览。要求 user 登录。
	g := router.Group("/app")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/user", UserList)
		g.POST("/user/:fileId/finalize", UserFinalize)
		g.POST("/user/batch-delete", UserBatchDelete)
		g.GET("/market", MarketList)
	}

	// 运营侧：跨用户应用治理 + 应用市场维护（staff 登录 + 权限）。
	admin := router.Group("/admin/apps")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("", staff.PermissionMiddleware("app:view"), AdminUserAppList)
		admin.POST("/batch-delete", staff.PermissionMiddleware("app:manage"), AdminUserAppBatchDelete)

		admin.GET("/market", staff.PermissionMiddleware("app:view"), AdminMarketList)
		admin.POST("/market/upload", staff.PermissionMiddleware("app:manage"), AdminMarketUpload)
		admin.POST("/market/batch-delete", staff.PermissionMiddleware("app:manage"), AdminMarketBatchDelete)
	}
}

func (m *appModule) OnStart() error { return nil }
func (m *appModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&appModule{})

	// 建表（幂等）：用户应用旁挂元数据 + 市场应用。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&AppUserMeta{}, &AppMarket{})
	})
}
