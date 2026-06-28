package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"
)

// lockProbe 复现「结算时 database is locked」用的最小模型：
// 一行被多 goroutine 反复 read-then-write（FirstOrCreate + Update），
// 正是 bumpSessionSettled / addDaily / addMinutes 的访问形态。
type lockProbe struct {
	ID    uint `gorm:"primaryKey"`
	Key   string
	Count int
}

func (lockProbe) TableName() string { return "framework_lock_probe" }

// TestSQLiteConcurrentWritesNoLock 断言：sqlite 下多 goroutine 并发做
// read-then-write 事务不应出现 "database is locked"。
//
// 复现原因（修复前会失败）：
//   - PRAGMA busy_timeout 是「按连接」生效的，旧代码用 DB.Exec 在打开后设置，
//     只覆盖连接池里的一条连接；池里其它连接 busy_timeout=0，遇争用立刻报 BUSY。
//   - GORM 事务默认 BEGIN(deferred)：先 SELECT 再 UPDATE，两连接同时持读锁再升级写锁
//     形成升级死锁，sqlite 即便设了 busy_timeout 也会立刻返回 BUSY。
func TestSQLiteConcurrentWritesNoLock(t *testing.T) {
	if dbTypeForTest() != "sqlite" {
		t.Skip("仅 sqlite 复现该锁问题")
	}

	dir := t.TempDir()
	withSQLiteDB(t, filepath.Join(dir, "lock_probe.db"), func(db *gorm.DB) {
		if err := db.AutoMigrate(&lockProbe{}); err != nil {
			t.Fatalf("AutoMigrate: %v", err)
		}

		const goroutines = 16
		const iters = 30
		var wg sync.WaitGroup
		errs := make(chan error, goroutines*iters)
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				// 多数 goroutine 抢同一行（key=hot），制造写锁争用。
				key := "hot"
				if g%4 == 0 {
					key = fmt.Sprintf("k%d", g)
				}
				for i := 0; i < iters; i++ {
					err := db.Transaction(func(tx *gorm.DB) error {
						var p lockProbe
						if err := tx.Where(lockProbe{Key: key}).
							FirstOrCreate(&p).Error; err != nil {
							return err
						}
						return tx.Model(&lockProbe{}).Where("id = ?", p.ID).
							Update("count", gorm.Expr("count + 1")).Error
					})
					if err != nil {
						errs <- err
					}
				}
			}(g)
		}
		wg.Wait()
		close(errs)

		var locked, other int
		var sample string
		for err := range errs {
			if isLockedErr(err) {
				locked++
				if sample == "" {
					sample = err.Error()
				}
			} else {
				other++
			}
		}
		if other > 0 {
			t.Fatalf("非锁错误 %d 个（不该出现）", other)
		}
		if locked > 0 {
			t.Fatalf("出现 %d 次 database is locked（样例：%s）", locked, sample)
		}
	})
}

func isLockedErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") || strings.Contains(s, "database table is locked")
}

func dbTypeForTest() string {
	t := os.Getenv("DB_TYPE")
	if t == "" {
		return "sqlite"
	}
	return t
}

// withSQLiteDB 用生产 InitDB 路径打开一个独立 sqlite 文件库并在结束后清理，
// 确保测试覆盖到 db.go 里真实的连接/DSN 配置。
func withSQLiteDB(t *testing.T, path string, fn func(db *gorm.DB)) {
	t.Helper()
	prevCfg := AppConfig
	prevDB := DB
	t.Cleanup(func() {
		CloseDB()
		AppConfig = prevCfg
		DB = prevDB
	})

	LoadConfig()
	AppConfig.DBType = "sqlite"
	AppConfig.DBName = path
	AppConfig.LogLevel = "error"

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	fn(DB)
}
