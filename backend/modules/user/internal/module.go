package user

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type userModule struct{}

func (m *userModule) Name() string { return "user" }

func (m *userModule) Init(db *gorm.DB) error {
	Service = newService(newRepository(db))
	return nil
}

// RegisterRoutes 注册前台路由。
//
// 注意：前台使用自己的 userAuth 中间件，刻意忽略框架传入的后台鉴权中间件
// （middlewareFuncs，那是 staff 身份域），以此实现前后台令牌互不通用。
func (m *userModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	g := router.Group("/user")
	{
		g.POST("/auth/register", Register)
		g.POST("/auth/login", Login)

		authed := g.Group("")
		authed.Use(UserAuth())
		{
			authed.GET("/me", GetProfile)
			authed.POST("/auth/logout", Logout)
		}
	}

	// 管理侧：后台员工管理前台用户（staff 鉴权 + 权限；前台用户自身无法访问）。
	admin := router.Group("/admin/users")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/list", staff.PermissionMiddleware("user:view"), AdminListUsers)
		admin.GET("/:id", staff.PermissionMiddleware("user:view"), AdminGetUser)
		admin.PUT("/:id/status", staff.PermissionMiddleware("user:manage"), AdminSetUserStatus)
	}
}

func (m *userModule) OnStart() error { return nil }
func (m *userModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&userModule{})

	// 建表（幂等）：前台用户表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&UserDB{})
	})
}
