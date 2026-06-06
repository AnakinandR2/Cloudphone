package cloudphone

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type cloudPhoneModule struct{}

func (m *cloudPhoneModule) Name() string { return "cloudphone" }

// Init 构造中台客户端（依赖 framework.AppConfig，已在 main/LoadConfig 中就绪）。
// 本模块不落本地库，无需建表。
func (m *cloudPhoneModule) Init(db *gorm.DB) error {
	initClient()
	return nil
}

// RegisterRoutes 注册后台只读资源浏览路由。
//
// 全部要求 staff 登录（注入的 middlewareFuncs）+ cloudphone:view 权限；
// 每次请求实时透传到云手机中台，无本地缓存、无同步动作。
func (m *cloudPhoneModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	g := router.Group("/cloudphone")
	g.Use(middlewareFuncs...)
	g.Use(staff.PermissionMiddleware("cloudphone:view"))
	{
		g.GET("/zones", ListZones)            // 可用区（规格/镜像页的区域选择来源）
		g.GET("/specs", ListSpecs)            // 规格：kind=phone(默认)|vm，需 zoneId
		g.GET("/vms", ListVMs)                // 云虚机（云主机）
		g.GET("/vm-statuses", ListVMEnums)    // 虚机状态枚举（前端过滤项）
		g.GET("/images", ListImageList)       // 镜像
		g.GET("/boot-plans", ListBootPlans)   // 按规格查可用云手机套餐（云主机行操作）
		g.GET("/spec-images", ListSpecImages) // 按规格查可用镜像（云主机行操作）
		g.GET("/apps", ListAppList)           // 应用市场
	}
}

func (m *cloudPhoneModule) OnStart() error { return nil }
func (m *cloudPhoneModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&cloudPhoneModule{})
}
