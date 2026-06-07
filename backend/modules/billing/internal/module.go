package billing

import (
	"manager-backend/framework"

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
	// 前台/后台路由在 Task 4 / Task 5 补全。
}

func (m *billingModule) OnStart() error { return nil }
func (m *billingModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&billingModule{})

	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Account{}, &LedgerEntry{})
	})
}
