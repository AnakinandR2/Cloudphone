package phone

import (
	"os"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/billing"
	"manager-backend/modules/library"
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
	// PushFromLibrary 经 library 公开门面消费私有素材：测试需装配 library（建表 + 服务）。
	if err := library.InitForTest(framework.DB); err != nil {
		panic(err)
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
