package billing

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}, &Order{}, &OrderItem{}); err != nil {
		panic(err)
	}
	if err := (&billingModule{}).Init(framework.DB); err != nil {
		panic(err)
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
