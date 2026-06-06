package phone

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&CloudPhone{}, &CpTask{}); err != nil {
		panic(err)
	}
	if err := (&phoneModule{}).Init(framework.DB); err != nil {
		panic(err)
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
