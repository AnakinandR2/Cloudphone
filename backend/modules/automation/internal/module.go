package automation

import (
	"context"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type automationModule struct {
	db *gorm.DB
}

func (m *automationModule) Name() string { return "automation" }

func (m *automationModule) Init(db *gorm.DB) error {
	m.db = db
	Service = newService(newRepository(db), newMidplatPort())
	return nil
}

func (m *automationModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：脚本 / 计划 / 任务，按属主隔离，要求 user 登录。
	g := router.Group("/automation")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/scripts", ListScripts)
		g.GET("/scripts/store", ListStoreScripts)
		g.GET("/scripts/usable", ListUsableScripts)
		g.POST("/scripts", CreateScript)
		g.PUT("/scripts/:id", UpdateScript)
		g.POST("/scripts/:id/toggle", ToggleScript)
		g.DELETE("/scripts/:id", DeleteScript)

		g.GET("/plans", ListPlans)
		g.POST("/plans", CreatePlan)
		g.POST("/plans/:id/start", planActionHandler("start"))
		g.POST("/plans/:id/pause", planActionHandler("pause"))
		g.DELETE("/plans/:id", planActionHandler("delete"))

		g.POST("/tasks/run", RunTask)
		g.GET("/tasks", ListTasks)
		g.GET("/tasks/:midTaskId", TaskDetail)
	}

	// 运营侧：商店脚本管理 + 用户脚本治理（staff 登录 + 权限）。
	admin := router.Group("/admin/automation")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/store", staff.PermissionMiddleware("script:view"), AdminStoreList)
		admin.POST("/store", staff.PermissionMiddleware("script:manage"), AdminStoreCreate)
		admin.PUT("/store/:id", staff.PermissionMiddleware("script:manage"), AdminStoreUpdate)
		admin.POST("/store/:id/toggle", staff.PermissionMiddleware("script:manage"), AdminStoreToggle)
		admin.DELETE("/store/:id", staff.PermissionMiddleware("script:manage"), AdminStoreDelete)

		admin.GET("/user-scripts", staff.PermissionMiddleware("script:view"), AdminUserScriptList)
		admin.POST("/user-scripts/:id/toggle", staff.PermissionMiddleware("script:manage"), AdminUserScriptToggle)
		admin.DELETE("/user-scripts/:id", staff.PermissionMiddleware("script:manage"), AdminUserScriptDelete)
	}
}

// syncRunner 任务同步 runner（仅中台已配置时运行）。
var syncRunner *framework.PeriodicRunner

func (m *automationModule) OnStart() error {
	if Service != nil && Service.ops != nil {
		// 每分钟：发现 plan 派生任务 + 刷新非终态任务状态。
		syncRunner = framework.NewPeriodicRunner(m.db, "automation:task-sync", time.Minute, 50*time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			Service.syncTasks(ctx)
			return nil
		})
		syncRunner.Start()
	}
	return nil
}

func (m *automationModule) OnStop() error {
	if syncRunner != nil {
		syncRunner.Stop()
		syncRunner = nil
	}
	return nil
}

func init() {
	framework.GlobalModule.Register(&automationModule{})

	// 建表（幂等）：脚本 / 计划 / 任务索引。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&AutomationScript{}, &AutomationPlan{}, &AutomationTask{})
	})
}
