package phone

import (
	"os"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/billing"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&CloudPhone{}, &CpTask{}, &RunSession{}); err != nil {
		panic(err)
	}
	if err := (&phoneModule{}).Init(framework.DB); err != nil {
		panic(err)
	}
	if err := billing.InitForTest(framework.DB); err != nil {
		panic(err)
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
