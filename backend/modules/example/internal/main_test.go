package example

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&ExampleItem{}); err != nil {
		panic(err)
	}
	// 复用真实装配逻辑注入测试 DB。
	if err := (&exampleModule{}).Init(framework.DB); err != nil {
		panic(err)
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
