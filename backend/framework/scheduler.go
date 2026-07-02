package framework

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CronLock 周期任务租约锁：name 唯一；谁的守卫式 UPDATE 抢到(expires_at<now)谁执行该 tick。
type CronLock struct {
	Name      string    `gorm:"primaryKey;type:varchar(64)" json:"name"`
	Holder    string    `gorm:"type:varchar(128)" json:"holder"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (CronLock) TableName() string { return "cron_locks" }

func init() {
	RegisterSetup(func(db *gorm.DB) error { return db.AutoMigrate(&CronLock{}) })
}

var (
	instanceID   string
	instanceOnce sync.Once
)

// InstanceID 本进程稳定标识(hostname-pid)。
func InstanceID() string {
	instanceOnce.Do(func() {
		h, _ := os.Hostname()
		instanceID = h + "-" + strconv.Itoa(os.Getpid())
	})
	return instanceID
}

// TryRunLocked 以租约锁尝试运行一次 fn：抢到 name 的租约则执行并返回(true, fn结果)；
// 未抢到(其它实例持有且未到期)返回(false,nil)。可移植 sqlite/mysql/pg。
func TryRunLocked(db *gorm.DB, name string, lease time.Duration, fn func() error) (bool, error) {
	now := time.Now()
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&CronLock{Name: name, ExpiresAt: now.Add(-time.Second)}).Error; err != nil {
		return false, err
	}
	res := db.Model(&CronLock{}).
		Where("name = ? AND expires_at < ?", name, now).
		Updates(map[string]interface{}{"holder": InstanceID(), "expires_at": now.Add(lease)})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	return true, fn()
}

// PeriodicRunner 周期性以租约锁触发 fn（HA 下每 tick 仅一个实例执行）。
type PeriodicRunner struct {
	db       *gorm.DB
	name     string
	interval time.Duration
	lease    time.Duration
	fn       func() error
	quit     chan struct{}
}

func NewPeriodicRunner(db *gorm.DB, name string, interval, lease time.Duration, fn func() error) *PeriodicRunner {
	return &PeriodicRunner{db: db, name: name, interval: interval, lease: lease, fn: fn, quit: make(chan struct{})}
}

func (p *PeriodicRunner) Start() {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.quit:
				return
			case <-ticker.C:
				p.runTick()
			}
		}
	}()
}

// runTick 执行一次带 recover 的周期触发：业务 fn 若 panic（空指针/越界/外部响应异常等），
// 只记录并跳过本轮，绝不让未捕获的 panic 击穿整个进程——gin 的 Recovery 只覆盖 HTTP 请求
// goroutine，不覆盖这些独立后台 goroutine。租约到期后锁自动释放，故 recover 是安全的（F1）。
func (p *PeriodicRunner) runTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[cron %s] panic recovered: %v", p.name, r)
		}
	}()
	if _, err := TryRunLocked(p.db, p.name, p.lease, p.fn); err != nil {
		log.Printf("[cron %s] error: %v", p.name, err)
	}
}

func (p *PeriodicRunner) Stop() { close(p.quit) }
