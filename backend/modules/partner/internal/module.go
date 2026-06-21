package partner

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type partnerModule struct{}

func (m *partnerModule) Name() string { return "partner" }

// Init 注入数据库并装配服务。
func (m *partnerModule) Init(db *gorm.DB) error {
	PartnerService = newService(newRepository(db))
	return nil
}

func (m *partnerModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 公开：my/www 展示与点击上报，无强制登录（点击接口软取 user_id）。
	pub := router.Group("/partner")
	{
		pub.GET("/list", ListPartners)
		pub.POST("/:id/click", ClickPartner)
	}

	// 管理侧：合作商 CRUD + 图片上传 + 点击明细（staff 登录 + 权限）。
	admin := router.Group("/admin/partners")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("", staff.PermissionMiddleware("partner:view"), AdminListPartners)
		admin.POST("", staff.PermissionMiddleware("partner:manage"), CreatePartner)
		admin.PUT("/:id", staff.PermissionMiddleware("partner:manage"), UpdatePartner)
		admin.DELETE("/:id", staff.PermissionMiddleware("partner:manage"), DeletePartner)
		admin.POST("/upload", staff.PermissionMiddleware("partner:manage"), UploadPartnerImage)
		admin.GET("/:id/clicks", staff.PermissionMiddleware("partner:view"), AdminListClicks)
	}
}

func (m *partnerModule) OnStart() error { return nil }
func (m *partnerModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&partnerModule{})

	// 建表（幂等）：合作商表 + 点击明细表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Partner{}, &PartnerClick{})
	})
}
