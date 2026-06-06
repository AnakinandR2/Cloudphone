package example

import (
	"fmt"

	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type exampleModule struct{}

func (m *exampleModule) Name() string { return "example" }

// Init 注入数据库并装配服务（依赖注入，替代全局 DB）。
func (m *exampleModule) Init(db *gorm.DB) error {
	ExampleService = newService(newRepository(db))
	return nil
}

func (m *exampleModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 后台专用模块：所有接口都要求 staff 登录 + 对应权限（前台/匿名一律不可访问）。
	exGroup := router.Group("/example")
	exGroup.Use(middlewareFuncs...)
	{
		exGroup.GET("/list", staff.PermissionMiddleware("example:view"), GetExampleList)
		exGroup.GET("/:id", staff.PermissionMiddleware("example:view"), GetExample)
		exGroup.POST("/create", staff.PermissionMiddleware("example:create"), CreateExample)
		exGroup.PUT("/update/:id", staff.PermissionMiddleware("example:edit"), UpdateExample)
		exGroup.DELETE("/delete/:id", staff.PermissionMiddleware("example:delete"), DeleteExample)
	}
}

func (m *exampleModule) OnStart() error { return nil }
func (m *exampleModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&exampleModule{})

	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&ExampleItem{}); err != nil {
			return err
		}
		var count int64
		db.Model(&ExampleItem{}).Count(&count)
		if count == 0 {
			items := make([]ExampleItem, 100)
			for i := range items {
				items[i] = ExampleItem{
					Title:   fmt.Sprintf("示例项目 %03d", i+1),
					Content: fmt.Sprintf("这是第 %d 条示例数据的详细内容，用于演示列表分页和搜索功能。", i+1),
				}
			}
			db.CreateInBatches(items, 20)
		}
		return nil
	})
}
