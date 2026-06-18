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

// Init 装配服务（本地绑定仓库 + 中台适配器）。
func (m *appModule) Init(db *gorm.DB) error {
	AppService = newService(newRepository(db), newMidplatPort())
	return nil
}

func (m *appModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 应用库管理：中台为租户全局，但 my 按「用户↔上传应用」本地绑定隔离；要求 user 登录。
	g := router.Group("/app")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/list", GetAppList)
		g.GET("/market", GetMarket)
		g.POST("/upload", UploadApp)
		g.POST("/batch-delete", BatchDeleteApps)

		// 浏览器驱动的分片上传流水线（参考 mcn）：浏览器切片逐片上传，进度更细、支持秒传。
		up := g.Group("/upload")
		{
			up.POST("/initiate", InitiateUpload)
			up.POST("/part", UploadPart)
			up.POST("/complete", CompleteUpload)
			up.POST("/parse", ParseApp)
			up.POST("/create", CreateAppFromUpload)
			up.GET("/status", UploadStatus)
		}
	}

	// 运营侧：查看/删除全部用户上传的应用（staff 登录 + 权限）。
	admin := router.Group("/admin/apps")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("", staff.PermissionMiddleware("app:view"), AdminListApps)
		admin.POST("/batch-delete", staff.PermissionMiddleware("app:manage"), AdminBatchDeleteApps)

		// 应用商店：admin 上传的应用即「应用市场」内容（my 端读 /app/market）。
		admin.GET("/store", staff.PermissionMiddleware("app:view"), AdminStoreList)
		admin.POST("/store/upload", staff.PermissionMiddleware("app:manage"), AdminStoreUpload)
		admin.POST("/store/batch-delete", staff.PermissionMiddleware("app:manage"), AdminStoreDelete)
	}
}

func (m *appModule) OnStart() error { return nil }
func (m *appModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&appModule{})

	// 建表（幂等）：用户↔应用 本地绑定表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&CustomerApp{})
	})
}
