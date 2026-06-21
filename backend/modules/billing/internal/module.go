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
	EntitlementService = newEntitlementService(newEntitlementRepository(db))
	TrialService = newTrialService(newTrialRepository(db))
	SeatService = newSeatService(newSeatRepository(db), EntitlementService)
	RuntimeService = newRuntimeService(newRuntimeRepository(db), EntitlementService, newRepository(db))
	LicenseService = newLicenseService(newLicenseRepository(db))

	// 新购买/费用模型（Phase 1c + 2）装配。
	WalletService = newWalletService(newRepository(db))
	PricingConfigService = newPricingConfigService(newPricingConfigRepository(db))
	RuntimeWalletService = newRuntimeWalletService(newRuntimeWalletRepository(db))
	FulfillService = newFulfillService(newLicenseRepository(db), newRuntimeWalletRepository(db), newRepository(db))
	BizOrderService = newBizOrderService(newBizOrderRepository(db), PricingConfigService, WalletService, FulfillService)
	RuntimeEngineService = newRuntimeEngineService(newRuntimeChargeRepository(db), newRuntimeWalletRepository(db), LicenseService, PricingConfigService)
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
		g.GET("/entitlements", GetMyEntitlements)
		g.GET("/trials", ListMyTrials)
		g.POST("/trials/:code/claim", ClaimTrial)
		g.GET("/runtime/usage", GetMyRuntimeUsage)

		// 新购买/费用模型（契约 §1）：占用 quote/orders 等契约路径。
		g.GET("/overview", GetBillingOverview)
		g.GET("/purchase-config", GetPurchaseConfig)
		g.POST("/quote", BizQuote)
		g.GET("/license-units", ListLicenseUnits)
		g.POST("/orders", CreateBizOrder)
		g.GET("/orders", ListBizOrders)
		g.GET("/orders/:id", GetBizOrder)
		g.POST("/orders/:id/pay", PayBizOrder)
		g.GET("/runtime/log", GetRuntimeLog)
	}

	// 后台：运营查看账户 + 调整余额（staff 登录 + 权限）。
	admin := router.Group("/admin/billing")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/accounts/:userId", staff.PermissionMiddleware("billing:view"), AdminGetAccount)
		admin.POST("/accounts/:userId/adjust", staff.PermissionMiddleware("billing:manage"), AdminAdjustBalance)
		// 资源赠送走统一履约（source=grant）。
		admin.POST("/accounts/:userId/adjust-resource", staff.PermissionMiddleware("billing:manage"), AdminAdjustResourceV2)
		admin.GET("/trials", staff.PermissionMiddleware("billing:view"), AdminListTrialPolicies)
		admin.POST("/trials", staff.PermissionMiddleware("billing:manage"), AdminCreateTrialPolicy)
		admin.PUT("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminUpdateTrialPolicy)
		admin.DELETE("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminDeleteTrialPolicy)
		admin.POST("/trials/:id/eligibility", staff.PermissionMiddleware("billing:manage"), AdminGrantTrialEligibility)
		admin.GET("/trials/:id/grants", staff.PermissionMiddleware("billing:view"), AdminListTrialGrants)
		// 新模型时长配置（契约 §2）。
		admin.GET("/runtime-config", staff.PermissionMiddleware("billing:view"), AdminGetRuntimePricing)
		admin.PUT("/runtime-config", staff.PermissionMiddleware("billing:manage"), AdminSaveRuntimePricing)

		// 新购买/费用模型后台配置（契约 §2）。
		admin.GET("/pricing", staff.PermissionMiddleware("billing:view"), AdminGetPricing)
		admin.PUT("/pricing", staff.PermissionMiddleware("billing:manage"), AdminSavePricing)
		admin.GET("/payment-methods", staff.PermissionMiddleware("billing:view"), AdminGetPaymentMethods)
		admin.PUT("/payment-methods", staff.PermissionMiddleware("billing:manage"), AdminSavePaymentMethods)
		admin.GET("/recharge-presets", staff.PermissionMiddleware("billing:view"), AdminGetRechargePresets)
		admin.PUT("/recharge-presets", staff.PermissionMiddleware("billing:manage"), AdminSaveRechargePresets)
		admin.GET("/notices", staff.PermissionMiddleware("billing:view"), AdminGetNotices)
		admin.PUT("/notices", staff.PermissionMiddleware("billing:manage"), AdminSaveNotices)
		admin.GET("/biz-orders", staff.PermissionMiddleware("billing:view"), AdminListBizOrders)
		admin.GET("/biz-orders/:id", staff.PermissionMiddleware("billing:view"), AdminGetBizOrder)
		admin.POST("/biz-orders/:id/mark-paid", staff.PermissionMiddleware("billing:manage"), AdminMarkBizOrderPaid)
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

	// 建表（幂等）：计费账户 + 统一流水 + 授权单元/订单等。
	// 旧 Order（billing_orders）与 EntitlementBatch 仍被 TrialService 使用，保留。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &EntitlementBatch{}, &Order{}, &TrialPolicy{}, &TrialPolicyItem{}, &TrialClaim{}, &TrialGrant{}, &TrialEligibility{}, &SeatUsage{}, &DunningState{}, &BillingRuntimeConfig{}, &RuntimeUsageSlice{}, &RuntimeSettlementWatermark{}, &LicenseUnit{},
			&PricingConfig{}, &RuntimeMinuteWallet{}, &RuntimeDailyUsage{}, &RuntimeCharge{}, &RuntimeSessionProgress{}, &BizOrder{}, &BizOrderItem{}); err != nil {
			return err
		}
		// 时长费单行配置 seed（幂等：不存在才建，默认单价 0=未启用收费）。
		if err := db.Where(BillingRuntimeConfig{ID: 1}).FirstOrCreate(&BillingRuntimeConfig{ID: 1}).Error; err != nil {
			return err
		}
		// 定价配置 seed（幂等：不存在才写默认值）。
		return seedPricingConfig(db)
	})
}

// InitForTest 供其他模块的测试装配 billing（建表 + 装配服务）。仅测试用。
func InitForTest(db *gorm.DB) error {
	if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &EntitlementBatch{},
		&Order{}, &TrialPolicy{}, &TrialPolicyItem{}, &TrialClaim{}, &TrialGrant{}, &TrialEligibility{}, &SeatUsage{}, &DunningState{},
		&BillingRuntimeConfig{}, &RuntimeUsageSlice{}, &RuntimeSettlementWatermark{}, &LicenseUnit{},
		&PricingConfig{}, &RuntimeMinuteWallet{}, &RuntimeDailyUsage{}, &RuntimeCharge{}, &RuntimeSessionProgress{}, &BizOrder{}, &BizOrderItem{}); err != nil {
		return err
	}
	return (&billingModule{}).Init(db)
}
