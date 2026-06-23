package app

import (
	"os"
	"testing"

	"manager-backend/framework"
)

// testUser 是仅供测试用的最小 users 表映射，用来给 AdminList 的 LEFT JOIN users 造数据。
// app 模块 internal 不能 import user internal，故在测试包内本地声明同名表。
type testUser struct {
	ID       uint   `gorm:"primaryKey"`
	Phone    string `gorm:"column:phone"`
	Nickname string `gorm:"column:nickname"`
}

func (testUser) TableName() string { return "users" }

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&CustomerApp{}, &testUser{}); err != nil {
		panic(err)
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
