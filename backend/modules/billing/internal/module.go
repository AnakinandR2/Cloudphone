package billing

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type billingModule struct {
	db *gorm.DB
}

func (m *billingModule) Name() string { return "billing" }

func (m *billingModule) Init(db *gorm.DB) error {
	m.db = db
	BillingService = newService(newRepository(db))
	CatalogService = newCatalogService(newCatalogRepository(db))
	EntitlementService = newEntitlementService(newEntitlementRepository(db))
	OrderService = newOrderService(newOrderRepository(db), CatalogService)
	TrialService = newTrialService(newTrialRepository(db))
	SeatService = newSeatService(newSeatRepository(db), EntitlementService)
	RuntimeService = newRuntimeService(newRuntimeRepository(db), EntitlementService, newRepository(db))
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
		g.GET("/trials", ListMyTrials)
		g.POST("/trials/:code/claim", ClaimTrial)
		g.GET("/runtime/usage", GetMyRuntimeUsage)
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
		admin.GET("/trials", staff.PermissionMiddleware("billing:view"), AdminListTrialPolicies)
		admin.POST("/trials", staff.PermissionMiddleware("billing:manage"), AdminCreateTrialPolicy)
		admin.PUT("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminUpdateTrialPolicy)
		admin.DELETE("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminDeleteTrialPolicy)
		admin.POST("/trials/:id/eligibility", staff.PermissionMiddleware("billing:manage"), AdminGrantTrialEligibility)
		admin.GET("/trials/:id/grants", staff.PermissionMiddleware("billing:view"), AdminListTrialGrants)
		admin.GET("/runtime-config", staff.PermissionMiddleware("billing:view"), AdminGetRuntimeConfig)
		admin.PUT("/runtime-config", staff.PermissionMiddleware("billing:manage"), AdminSaveRuntimeConfig)
	}
}

func (m *billingModule) OnStart() error {
	if SeatService != nil {
		dunningRunner = framework.NewPeriodicRunner(m.db, "billing:dunning", dunningInterval, dunningLease, func() error {
			return SeatService.runDunning(defaultGraceDays, defaultFrozenDays)
		})
		dunningRunner.Start()
	}
	return nil
}

func (m *billingModule) OnStop() error {
	if dunningRunner != nil {
		dunningRunner.Stop()
		dunningRunner = nil
	}
	return nil
}

func init() {
	framework.GlobalModule.Register(&billingModule{})

	// 建表（幂等）：计费账户 + 统一流水 + 商品目录 + 折扣阶梯。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}, &Order{}, &OrderItem{}, &TrialPolicy{}, &TrialPolicyItem{}, &TrialClaim{}, &TrialGrant{}, &TrialEligibility{}, &SeatUsage{}, &DunningState{}, &BillingRuntimeConfig{}, &RuntimeUsageSlice{}, &RuntimeSettlementWatermark{}); err != nil {
			return err
		}
		// 时长费单行配置 seed（幂等：不存在才建，默认单价 0=未启用收费）。
		if err := db.Where(BillingRuntimeConfig{ID: 1}).FirstOrCreate(&BillingRuntimeConfig{ID: 1}).Error; err != nil {
			return err
		}
		return SeedCatalog(db)
	})
}

// InitForTest 供其他模块的测试装配 billing（建表 + 装配服务）。仅测试用。
func InitForTest(db *gorm.DB) error {
	if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{},
		&Order{}, &OrderItem{}, &TrialPolicy{}, &TrialPolicyItem{}, &TrialClaim{}, &TrialGrant{}, &TrialEligibility{}, &SeatUsage{}, &DunningState{},
		&BillingRuntimeConfig{}, &RuntimeUsageSlice{}, &RuntimeSettlementWatermark{}); err != nil {
		return err
	}
	return (&billingModule{}).Init(db)
}
