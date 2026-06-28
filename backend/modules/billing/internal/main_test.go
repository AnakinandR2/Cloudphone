package billing

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}, &TrialPolicy{}, &TrialPolicyItem{}, &TrialClaim{}, &TrialGrant{}, &TrialEligibility{}, &LicenseUnit{},
		&PricingConfig{}, &RuntimeMinuteWallet{}, &RuntimeDailyUsage{}, &RuntimeCharge{}, &RuntimeSessionProgress{}, &BizOrder{}, &BizOrderItem{}, &framework.CronLock{}); err != nil {
		panic(err)
	}
	if err := (&billingModule{}).Init(framework.DB); err != nil {
		panic(err)
	}
	// billing 经 user 门面（IDByPhone/PhonesByIDs）回填订单手机号，需装配 user 模块并建 users 表。
	// billing internal import 了 user 公开包，其 init() 已把 user 模块登记进 GlobalModule。
	for _, mod := range framework.GlobalModule.GetAll() {
		if mod.Name() == "user" {
			if err := mod.Init(framework.DB); err != nil {
				panic(err)
			}
		}
	}
	if err := framework.RunSetup(framework.DB); err != nil {
		panic(err)
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
