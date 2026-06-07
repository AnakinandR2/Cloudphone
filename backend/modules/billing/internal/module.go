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
	CatalogService = newCatalogService(newCatalogRepository(db))
	EntitlementService = newEntitlementService(newEntitlementRepository(db))
	OrderService = newOrderService(newOrderRepository(db), CatalogService)
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
		g.GET("/skus", ListSkus)
		g.POST("/quote", Quote)
		g.GET("/entitlements", GetMyEntitlements)
		g.POST("/orders", CreateOrder)
		g.GET("/orders", ListMyOrders)
		g.GET("/orders/:id", GetMyOrder)
		g.POST("/orders/:id/pay", PayMyOrder)
	}

	// 后台：运营查看账户 + 调整余额（staff 登录 + 权限）。
	admin := router.Group("/admin/billing")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/accounts/:userId", staff.PermissionMiddleware("billing:view"), AdminGetAccount)
		admin.POST("/accounts/:userId/adjust", staff.PermissionMiddleware("billing:manage"), AdminAdjustBalance)
		admin.POST("/accounts/:userId/adjust-resource", staff.PermissionMiddleware("billing:manage"), AdminAdjustResource)
		admin.GET("/skus", staff.PermissionMiddleware("billing:view"), AdminListSkus)
		admin.POST("/skus", staff.PermissionMiddleware("billing:manage"), AdminCreateSku)
		admin.PUT("/skus/:id", staff.PermissionMiddleware("billing:manage"), AdminUpdateSku)
		admin.DELETE("/skus/:id", staff.PermissionMiddleware("billing:manage"), AdminDeleteSku)
		admin.GET("/skus/:id/tiers", staff.PermissionMiddleware("billing:view"), AdminListTiers)
		admin.POST("/skus/:id/tiers", staff.PermissionMiddleware("billing:manage"), AdminCreateTier)
		admin.PUT("/tiers/:tierId", staff.PermissionMiddleware("billing:manage"), AdminUpdateTier)
		admin.DELETE("/tiers/:tierId", staff.PermissionMiddleware("billing:manage"), AdminDeleteTier)
		admin.GET("/orders", staff.PermissionMiddleware("billing:view"), AdminListOrders)
		admin.POST("/orders/:id/mark-paid", staff.PermissionMiddleware("billing:manage"), AdminMarkOrderPaid)
	}
}

func (m *billingModule) OnStart() error { return nil }
func (m *billingModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&billingModule{})

	// 建表（幂等）：计费账户 + 统一流水 + 商品目录 + 折扣阶梯。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}, &Order{}, &OrderItem{}); err != nil {
			return err
		}
		return SeedCatalog(db)
	})
}
