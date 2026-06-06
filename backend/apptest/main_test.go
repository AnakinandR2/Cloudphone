package apptest

import (
	"os"
	"testing"

	"manager-backend/framework"

	// blank-import 各模块的公开包以触发自注册（等价于 main.go 的装配）。
	_ "manager-backend/modules/accesslog"
	_ "manager-backend/modules/app"
	_ "manager-backend/modules/cloudphone"
	_ "manager-backend/modules/example"
	_ "manager-backend/modules/note"
	_ "manager-backend/modules/phone"
	_ "manager-backend/modules/proxy"
	_ "manager-backend/modules/staff"
	_ "manager-backend/modules/user"
	// scaffold:module-imports
)

var tdb *framework.TestDB

func TestMain(m *testing.M) {
	tdb, _ = framework.SetupTestDB(m)
	tdb.RunSetup()

	// 注入测试 DB 并初始化各模块（与 main 中一致，仅经公开 Module 接口）。
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
