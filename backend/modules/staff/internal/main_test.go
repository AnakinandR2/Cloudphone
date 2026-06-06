package staff_test

import (
	"os"
	"testing"

	"manager-backend/framework"
)

var tdb *framework.TestDB

func TestMain(m *testing.M) {
	tdb, _ = framework.SetupTestDB(m)
	tdb.RunSetup()

	// 经公开装配路径注入测试 DB（等价于 main 中的模块初始化），
	// 装配 Service/RoleService 的数据库句柄。
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
