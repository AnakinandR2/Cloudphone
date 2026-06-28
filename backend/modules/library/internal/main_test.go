package library

import (
	"os"
	"testing"
	"time"

	"manager-backend/framework"

	// 触发 billing 模块注册：library.Init 依赖 billing 的公开门面 RegisterBizType，
	// 且套餐订单往返测试需要 billing 已装配（统一订单/支付/履约）。
	_ "manager-backend/modules/billing"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	// 建所有已注册模块的表 + 各模块 seed（含 library 七表 + library_pricing_config seed）。
	if err := framework.RunSetup(framework.DB); err != nil {
		panic(err)
	}
	// 装配所有已注册模块（library.Init 在此注册四种 billing 业务类型）。
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

// fixedNow 把 Service.now 固定到给定时刻，结束自动还原。
func fixedNow(t *testing.T, at time.Time) {
	t.Helper()
	prev := Service.now
	Service.now = func() time.Time { return at }
	t.Cleanup(func() { Service.now = prev })
}

// cleanLibraryData 清理本测试造的 library 数据（多测试共用一个 sqlite 库）。
func cleanLibraryData(t *testing.T, userID int) {
	t.Helper()
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM library_subscriptions WHERE user_id = ?", userID)
		framework.DB.Exec("DELETE FROM library_usage WHERE user_id = ?", userID)
	})
}

// restorePricingT500Monthly 临时把 t500 档位月价改成给定值，用例结束还原默认定价配置。
func restorePricingT500Monthly(t *testing.T, monthly int64) {
	t.Helper()
	t.Cleanup(func() { _ = PricingConfigService.Save(defaultPricingConfig()) })
	cfg, err := PricingConfigService.Get()
	if err != nil {
		t.Fatalf("get pricing: %v", err)
	}
	for i := range cfg.Tiers {
		if cfg.Tiers[i].Code == "t500" {
			cfg.Tiers[i].MonthlyPriceCents = monthly
		}
	}
	if err := PricingConfigService.Save(cfg); err != nil {
		t.Fatalf("save pricing: %v", err)
	}
}

// setSub 直接写入某用户当前订阅（造测试态）。
func setSub(t *testing.T, userID int, tier string, capBytes, monthly int64, expire *time.Time) {
	t.Helper()
	sub := &LibrarySubscription{
		UserID:            uint(userID),
		TierCode:          tier,
		CapacityBytes:     capBytes,
		MonthlyPriceCents: monthly,
		ExpireAt:          expire,
		Status:            SubActive,
	}
	if err := Service.repo.upsertSubscription(sub); err != nil {
		t.Fatalf("setSub: %v", err)
	}
}
