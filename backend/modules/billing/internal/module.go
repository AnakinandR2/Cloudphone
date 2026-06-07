package billing

import (
	"manager-backend/framework"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type billingModule struct{}

func (m *billingModule) Name() string { return "billing" }

func (m *billingModule) Init(db *gorm.DB) error {
	BillingService = newService(newRepository(db))
	return nil
}

func (m *billingModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：我的计费，按属主隔离，要求 user 登录。
	g := router.Group("/billing")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/account", GetMyAccount)
		g.GET("/ledger", GetMyLedger)
		g.POST("/topup", Topup)
	}
}

func (m *billingModule) OnStart() error { return nil }
func (m *billingModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&billingModule{})

	// 建表（幂等）：计费账户 + 统一流水。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Account{}, &LedgerEntry{})
	})
}
