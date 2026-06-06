package framework

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TestDB 测试数据库上下文
type TestDB struct {
	DBName string
}

// SetupTestDB 为测试模块创建一个随机后缀名称的隔离数据库。
func SetupTestDB(m *testing.M) (*TestDB, int) {
	suffix := randomSuffix()

	os.Setenv("GIN_MODE", "test")
	os.Setenv("LOG_LEVEL", "silent")

	LoadConfig()

	dbType := AppConfig.DBType
	if dbType == "" {
		dbType = "sqlite"
	}

	var dbName string
	switch dbType {
	case "sqlite":
		dbName = filepath.Join(os.TempDir(), fmt.Sprintf("test_%s.db", suffix))
	default:
		dbName = fmt.Sprintf("test_%s", suffix)
	}

	os.Setenv("DB_NAME", dbName)
	AppConfig.DBName = dbName

	tdb := &TestDB{DBName: dbName}

	if err := InitDB(); err != nil {
		log.Fatalf("[testutil] 连接测试数据库失败: %v", err)
	}

	return tdb, 0
}

// RunSetup 执行全局注册的建表与初始数据（需要先 import _ "business/modules"）
func (tdb *TestDB) RunSetup() {
	if err := RunSetup(DB); err != nil {
		log.Fatalf("[testutil] 建表与初始化失败: %v", err)
	}
}

// Teardown 关闭连接并删除测试数据库
func (tdb *TestDB) Teardown() {
	CloseDB()

	dbType := AppConfig.DBType
	if dbType == "" {
		dbType = "sqlite"
	}

	switch dbType {
	case "sqlite":
		os.Remove(tdb.DBName)
		log.Printf("[testutil] 已清理测试数据库文件 %s", tdb.DBName)
	default:
		if err := dropDatabase(tdb.DBName); err != nil {
			log.Printf("[testutil] 删除测试数据库 %s 失败: %v", tdb.DBName, err)
		} else {
			log.Printf("[testutil] 已清理测试数据库 %s", tdb.DBName)
		}
	}
}

// CleanTable 清空指定表的数据
func CleanTable(tables ...string) {
	dialect := DB.Dialector.Name()
	for _, table := range tables {
		switch dialect {
		case "sqlite":
			DB.Exec("DELETE FROM `" + table + "`")
			DB.Exec("DELETE FROM sqlite_sequence WHERE name=?", table)
		case "postgres":
			DB.Exec("TRUNCATE TABLE \"" + table + "\" RESTART IDENTITY CASCADE")
		default:
			DB.Exec("TRUNCATE TABLE `" + table + "`")
		}
	}
}

func adminDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True",
		AppConfig.DBUser, AppConfig.DBPassword, AppConfig.DBHost, AppConfig.DBPort)
}

func dropDatabase(dbName string) error {
	dbType := AppConfig.DBType
	if dbType == "" {
		dbType = "sqlite"
	}

	switch dbType {
	case "mysql":
		db, err := sql.Open("mysql", adminDSN())
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
		return err
	case "postgres", "postgresql":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
			AppConfig.DBHost, AppConfig.DBPort, AppConfig.DBUser, AppConfig.DBPassword)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\"", dbName))
		return err
	}
	return nil
}

func randomSuffix() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}
