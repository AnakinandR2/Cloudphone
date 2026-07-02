package proxy

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type proxyModule struct{}

func (m *proxyModule) Name() string { return "proxy" }

// Init 注入数据库并装配服务（依赖注入，替代全局 DB）。
func (m *proxyModule) Init(db *gorm.DB) error {
	ProxyService = newService(newRepository(db), newProber())
	return nil
}

func (m *proxyModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：我的代理，按属主隔离，要求 user 登录（忽略后台 staff 中间件）。
	g := router.Group("/proxy")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/list", GetProxyList)
		g.GET("/options", GetProxyOptions)
		g.GET("/:id", GetProxy)
		g.POST("/:id/test", TestProxy)
		g.POST("/create", CreateProxy)
		g.POST("/probe", ProbeProxy)
		g.POST("/batch", BatchImportProxies)
		g.PUT("/update/:id", UpdateProxy)
		g.DELETE("/delete/:id", DeleteProxy)
	}

	// 管理侧：代理池全量查看/删除（staff 登录 + 权限）。
	admin := router.Group("/admin/proxies")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/list", staff.PermissionMiddleware("proxy:view"), AdminListProxies)
		admin.GET("/:id", staff.PermissionMiddleware("proxy:view"), AdminGetProxy)
		admin.DELETE("/delete/:id", staff.PermissionMiddleware("proxy:manage"), AdminDeleteProxy)
	}
}

func (m *proxyModule) OnStart() error { return nil }
func (m *proxyModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&proxyModule{})

	// 建表（幂等）：代理池表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Proxy{})
	})
}

// InitForTest 供其他模块的测试装配 proxy（建表 + 装配服务）。仅测试用。
func InitForTest(db *gorm.DB) error {
	if err := db.AutoMigrate(&Proxy{}); err != nil {
		return err
	}
	return (&proxyModule{}).Init(db)
}
