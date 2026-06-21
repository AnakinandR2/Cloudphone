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
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
