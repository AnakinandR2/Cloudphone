package billing

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"
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

	// 后台：运营查看账户 + 调整余额（staff 登录 + 权限）。
	admin := router.Group("/admin/billing")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/accounts/:userId", staff.PermissionMiddleware("billing:view"), AdminGetAccount)
		admin.POST("/accounts/:userId/adjust", staff.PermissionMiddleware("billing:manage"), AdminAdjustBalance)
	}
}

func (m *billingModule) OnStart() error { return nil }
func (m *billingModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&billingModule{})

	// 建表（幂等）：计费账户 + 统一流水 + 商品目录 + 折扣阶梯。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}); err != nil {
			return err
		}
		return SeedCatalog(db)
	})
}
