# 计费系统 Phase 1 · 计划 6：实例创建门禁 + 欠费保护 cron Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 引入 HA 安全的周期任务机制（DB 租约锁），把实例创建/开机接入计费门禁（席位占用、欠费冻结），并实现欠费保护状态机（active→grace→frozen→recycled，cron 驱动），冻结强制关机、回收销毁超量实例。

**Architecture:** **跨模块单向 phone→billing**（billing 不依赖 phone，无循环）。billing 记账「实例席位占用」(`SeatUsage`)：`phone.Create` 调 `billing.TryOccupyInstanceSeat`（原子门禁），`phone` 删档调 `billing.ReleaseInstanceSeat`。billing 欠费检测全本地（占用 vs 容量）。冻结/回收**动作**由 phone 侧读 billing 状态执行（开机查 `billing.IsFrozen`；phone 执行 worker 读 `billing.ListDunningEnforcement` 做强制关机/销毁超量实例），并经 `billing.ReconcileInstanceSeats` 周期纠正占用漂移。周期任务统一走 **`framework` 租约锁**（`cron_locks` 表 + `TryRunLocked`/`PeriodicRunner`），HA 多实例每 tick 仅一个实例执行；现有 phone `taskWorker` 一并改造。状态转换皆**幂等守卫式 UPDATE**（纵深防御）。

**Tech Stack:** Go 1.26 / Gin / GORM（sqlite 测试）/ testify。模块名 `manager-backend`，分支 `feature/billing-dunning`。

---

## 跨模块契约（billing 公开门面，供 phone 调用）

`backend/modules/billing/billing.go`（公开包）导出（委托 internal）：
- `TryOccupyInstanceSeat(userID int) error` — 原子：未冻结且 占用<容量 → 占用+1；否则 `apperr.Validation`/`Forbidden`。
- `ReleaseInstanceSeat(userID int) error` — 占用-1（不低于0）。
- `ReconcileInstanceSeats(userID, actualCount int) error` — 把占用校正为 phone 的真实计数（纠漂移）。
- `IsFrozen(userID int) (bool, error)` — 用户是否处于 frozen/recycled（开机门禁）。
- `InstanceSeatCapacity(userID int) (int64, error)` — 可用实例席位容量（复用 EntitlementService）。
- `ListDunningEnforcement() ([]EnforcementTarget, error)` — frozen/recycled 用户 + 其容量（phone 执行用）。`EnforcementTarget{UserID int, State string, Capacity int64}` 也需在公开包导出类型别名。

> phone 依赖 billing 公开包；billing **不得** import phone（arch_test 边界）。

## 计划 6 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `framework/scheduler.go` | `CronLock` 模型 + `TryRunLocked` + `PeriodicRunner` + `InstanceID` | 创建 |
| `framework/scheduler_test.go` | 租约锁测试 | 创建 |
| `billing/internal/seat_model.go` | `SeatUsage`、`DunningState`、`EnforcementTarget`、常量 | 创建 |
| `billing/internal/seat_repository.go` | 占用原子增减/校正、容量、欠费扫描/状态 CRUD | 创建 |
| `billing/internal/seat_service.go` | `SeatService`：TryOccupy/Release/Reconcile/IsFrozen/Capacity/Enforcement | 创建 |
| `billing/internal/dunning.go` | 欠费 cron：状态机转换 + PeriodicRunner 装配 | 创建 |
| `billing/internal/seat_service_test.go` | 占用/欠费测试 | 创建 |
| `billing/billing.go` | 导出跨模块门面 | 修改 |
| `billing/internal/module.go` | Init 装配 SeatService；建表；OnStart 起欠费 cron；OnStop | 修改 |
| `billing/internal/main_test.go` | AutoMigrate 加 SeatUsage/DunningState/CronLock | 修改 |
| `phone/internal/worker.go` | taskWorker 走租约锁 | 修改 |
| `phone/internal/module.go` | worker 传入 db；OnStart 起 enforcement runner | 修改 |
| `phone/internal/service.go` | Create/Delete/Power 接入 billing 门禁；enforcement 方法 | 修改 |
| `phone/internal/enforce.go` | 读 billing 执行：冻结关机/回收销毁 | 创建 |

**约定速查**：`framework.TryRunLocked/PeriodicRunner`；`apperr.*`；前台 `user.AuthMiddleware`；Go 可用 `time.Now()`；测试 `framework.SetupTestDB`/`CleanTable`；既有 `EntitlementService.Capacity(uid,SubjectInstanceSeat)`、`SubjectInstanceSeat`；phone `s.ops`(中台,nil=本地降级)、`s.repo`、`StatusRunning/Stopped`、`MidplatReady`。

---

## Task 1：framework 租约锁调度 + phone worker 改造

**Files:** Create `framework/scheduler.go`, `framework/scheduler_test.go`; Modify `phone/internal/worker.go`, `phone/internal/module.go`。

- [ ] **Step 1：创建 `framework/scheduler.go`**

```go
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

// TryRunLocked 以租约锁尝试运行一次 fn：抢到 name 的租约则执行并返回(true, fn结果)，
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
				if _, err := TryRunLocked(p.db, p.name, p.lease, p.fn); err != nil {
					log.Printf("[cron %s] error: %v", p.name, err)
				}
			}
		}
	}()
}

func (p *PeriodicRunner) Stop() { close(p.quit) }
```

- [ ] **Step 2：创建 `framework/scheduler_test.go`**

```go
package framework

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryRunLockedSingleRunnerThenFailover(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&CronLock{}))
	t.Cleanup(func() { CleanTable("cron_locks") })

	runs := 0
	job := func() error { runs++; return nil }

	// 首次抢到 → 运行
	ran, err := TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.True(t, ran)
	assert.Equal(t, 1, runs)

	// 租约未到期 → 抢不到，不运行
	ran, err = TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.False(t, ran)
	assert.Equal(t, 1, runs)

	// 租约到期后 → 可再次抢到（failover）
	time.Sleep(220 * time.Millisecond)
	ran, err = TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.True(t, ran)
	assert.Equal(t, 2, runs)
}

func TestTryRunLockedFnErrorPropagates(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&CronLock{}))
	t.Cleanup(func() { CleanTable("cron_locks") })
	ran, err := TryRunLocked(DB, "t-err", time.Minute, func() error { return assert.AnError })
	assert.True(t, ran)
	assert.Error(t, err)
}
```

> framework 已有 `TestMain`？若无则需建。检查 `framework/` 是否已有 `TestMain`（`config_test.go`/`middleware_test.go` 在跑 → 已有测试基建）。若 `DB` 未初始化，在本测试文件加 `TestMain`：`func TestMain(m *testing.M){ tdb,_:=SetupTestDB(m); code:=m.Run(); tdb.Teardown(); os.Exit(code) }`（先 grep 确认是否已存在 TestMain，避免重复定义）。

- [ ] **Step 3：改 `phone/internal/worker.go` 走租约锁**

`taskWorker` 加 `db *gorm.DB` 字段；`newTaskWorker` 增参；tick 包进 `TryRunLocked`：
```go
package phone

import (
	"context"
	"time"

	"manager-backend/framework"

	"gorm.io/gorm"
)

const workerInterval = 5 * time.Second

type taskWorker struct {
	svc  *serviceImpl
	db   *gorm.DB
	quit chan struct{}
}

func newTaskWorker(svc *serviceImpl, db *gorm.DB) *taskWorker {
	return &taskWorker{svc: svc, db: db, quit: make(chan struct{})}
}

func (w *taskWorker) start() {
	go func() {
		ticker := time.NewTicker(workerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.quit:
				return
			case <-ticker.C:
				// HA：每 tick 仅一个实例执行；lease=interval，依赖 runDueTasks 幂等容忍偶发重叠。
				_, _ = framework.TryRunLocked(w.db, "phone:task-worker", workerInterval, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					w.svc.runDueTasks(ctx)
					return nil
				})
			}
		}
	}()
}

func (w *taskWorker) stop() { close(w.quit) }
```

- [ ] **Step 4：改 `phone/internal/module.go` 传 db 给 worker**

`phoneModule` 加 `db *gorm.DB` 字段，`Init` 里存：`m.db = db`（在 `PhoneService = ...` 后）。`OnStart` 里 `phoneWorker = newTaskWorker(PhoneService, m.db)`。

- [ ] **Step 5：测试 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./framework/ -run 'TestTryRunLocked' -v && go test ./... 2>&1 | tail -15 && gofmt -l framework/ modules/phone/ && go vet ./... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/framework backend/modules/phone
git commit -m "feat(framework): DB 租约锁周期任务调度 + phone worker HA 改造

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> 注：framework `RegisterSetup(CronLock)` 在 init 中登记，生产 RunSetup 自动建表；各模块 main_test 若需 cron_locks 自行 AutoMigrate。先 grep `func TestMain` in framework/，无则在 scheduler_test.go 建（含 `import "os"`）。

---

## Task 2：billing 席位占用 + 容量 + 公开门面

**Files:** Create `seat_model.go`, `seat_repository.go`, `seat_service.go`; Modify `billing.go`(门面), `module.go`(Init+建表), `main_test.go`(AutoMigrate); Test `seat_service_test.go`。

- [ ] **Step 1：创建 `seat_model.go`**

```go
package billing

import "time"

// 欠费状态
const (
	DunningActive   = "active"
	DunningGrace    = "grace"
	DunningFrozen   = "frozen"
	DunningRecycled = "recycled"
)

// SeatUsage 用户实例席位占用计数（billing 记账，phone 创建/销毁时增减，周期校正）。
type SeatUsage struct {
	UserID            uint      `gorm:"primaryKey" json:"user_id"`
	InstanceSeatsUsed int64     `gorm:"not null;default:0" json:"instance_seats_used"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SeatUsage) TableName() string { return "billing_seat_usages" }

// DunningState 用户欠费保护状态（cron 维护）。
type DunningState struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	State     string    `gorm:"type:varchar(20);not null;default:'active'" json:"state"`
	EnteredAt time.Time `json:"entered_at"` // 进入当前(非 active)状态的时间
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DunningState) TableName() string { return "billing_dunning_states" }

// EnforcementTarget phone 侧执行用：需冻结/回收的用户 + 其容量。
type EnforcementTarget struct {
	UserID   int    `json:"user_id"`
	State    string `json:"state"`
	Capacity int64  `json:"capacity"`
}
```

- [ ] **Step 2：创建 `seat_repository.go`**

```go
package billing

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type seatRepository interface {
	occupy(userID int, capacity int64) (bool, error) // 原子：used<capacity → +1，返回是否成功
	release(userID int) error
	reconcile(userID int, count int64) error
	usage(userID int) (int64, error)
	listUsages() ([]SeatUsage, error)
	getDunning(userID int) (*DunningState, error)
	upsertDunning(userID int, state string, enteredAt time.Time) error
	listDunningByStates(states []string) ([]DunningState, error)
}

type gormSeatRepository struct{ db *gorm.DB }

func newSeatRepository(db *gorm.DB) seatRepository { return &gormSeatRepository{db: db} }

// occupy 原子占用：保证行存在后，守卫式 UPDATE used=used+1 WHERE used < capacity。
func (r *gormSeatRepository) occupy(userID int, capacity int64) (bool, error) {
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&SeatUsage{UserID: uint(userID)}).Error; err != nil {
		return false, err
	}
	res := r.db.Model(&SeatUsage{}).
		Where("user_id = ? AND instance_seats_used < ?", userID, capacity).
		UpdateColumn("instance_seats_used", gorm.Expr("instance_seats_used + 1"))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *gormSeatRepository) release(userID int) error {
	return r.db.Model(&SeatUsage{}).
		Where("user_id = ? AND instance_seats_used > 0", userID).
		UpdateColumn("instance_seats_used", gorm.Expr("instance_seats_used - 1")).Error
}

func (r *gormSeatRepository) reconcile(userID int, count int64) error {
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&SeatUsage{UserID: uint(userID)}).Error; err != nil {
		return err
	}
	return r.db.Model(&SeatUsage{}).Where("user_id = ?", userID).
		UpdateColumn("instance_seats_used", count).Error
}

func (r *gormSeatRepository) usage(userID int) (int64, error) {
	var u SeatUsage
	err := r.db.Where("user_id = ?", userID).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return u.InstanceSeatsUsed, err
}

func (r *gormSeatRepository) listUsages() ([]SeatUsage, error) {
	var items []SeatUsage
	err := r.db.Find(&items).Error
	return items, err
}

func (r *gormSeatRepository) getDunning(userID int) (*DunningState, error) {
	var d DunningState
	err := r.db.Where("user_id = ?", userID).First(&d).Error
	if err == gorm.ErrRecordNotFound {
		return &DunningState{UserID: uint(userID), State: DunningActive}, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *gormSeatRepository) upsertDunning(userID int, state string, enteredAt time.Time) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"state", "entered_at", "updated_at"}),
	}).Create(&DunningState{UserID: uint(userID), State: state, EnteredAt: enteredAt}).Error
}

func (r *gormSeatRepository) listDunningByStates(states []string) ([]DunningState, error) {
	var items []DunningState
	err := r.db.Where("state IN ?", states).Find(&items).Error
	return items, err
}
```

- [ ] **Step 3：创建 `seat_service.go`**

```go
package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

type seatServiceImpl struct {
	repo seatRepository
	ent  *entitlementServiceImpl
}

var SeatService *seatServiceImpl

func newSeatService(repo seatRepository, ent *entitlementServiceImpl) *seatServiceImpl {
	return &seatServiceImpl{repo: repo, ent: ent}
}

// TryOccupyInstanceSeat 实例创建门禁：未冻结 且 占用<容量 → 占用+1。
func (s *seatServiceImpl) TryOccupyInstanceSeat(userID int) error {
	frozen, err := s.IsFrozen(userID)
	if err != nil {
		return err
	}
	if frozen {
		return apperr.Forbidden("账户已冻结，无法创建实例")
	}
	cap, err := s.ent.Capacity(userID, SubjectInstanceSeat)
	if err != nil {
		return err
	}
	ok, err := s.repo.occupy(userID, cap)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation("实例席位不足，请购买后再创建")
	}
	return nil
}

func (s *seatServiceImpl) ReleaseInstanceSeat(userID int) error { return s.repo.release(userID) }

func (s *seatServiceImpl) ReconcileInstanceSeats(userID, count int) error {
	return s.repo.reconcile(userID, int64(count))
}

func (s *seatServiceImpl) InstanceSeatCapacity(userID int) (int64, error) {
	return s.ent.Capacity(userID, SubjectInstanceSeat)
}

func (s *seatServiceImpl) IsFrozen(userID int) (bool, error) {
	d, err := s.repo.getDunning(userID)
	if err != nil {
		return false, err
	}
	return d.State == DunningFrozen || d.State == DunningRecycled, nil
}

// ListDunningEnforcement frozen/recycled 用户 + 容量（phone 执行用）。
func (s *seatServiceImpl) ListDunningEnforcement() ([]EnforcementTarget, error) {
	ds, err := s.repo.listDunningByStates([]string{DunningFrozen, DunningRecycled})
	if err != nil {
		return nil, err
	}
	out := make([]EnforcementTarget, 0, len(ds))
	for _, d := range ds {
		cap, err := s.ent.Capacity(int(d.UserID), SubjectInstanceSeat)
		if err != nil {
			return nil, err
		}
		out = append(out, EnforcementTarget{UserID: int(d.UserID), State: d.State, Capacity: cap})
	}
	return out, nil
}

// runDunning cron 一次：扫描所有占用，转换欠费状态（幂等）。X=graceDays, Y=frozenDays。
func (s *seatServiceImpl) runDunning(graceDays, frozenDays int) error {
	now := time.Now()
	usages, err := s.repo.listUsages()
	if err != nil {
		return err
	}
	for _, u := range usages {
		cap, err := s.ent.Capacity(int(u.UserID), SubjectInstanceSeat)
		if err != nil {
			return err
		}
		over := u.InstanceSeatsUsed > cap
		d, err := s.repo.getDunning(int(u.UserID))
		if err != nil {
			return err
		}
		if !over {
			if d.State != DunningActive {
				if err := s.repo.upsertDunning(int(u.UserID), DunningActive, now); err != nil {
					return err
				}
			}
			continue
		}
		// 超量：按时间推进状态
		switch d.State {
		case DunningActive:
			if err := s.repo.upsertDunning(int(u.UserID), DunningGrace, now); err != nil {
				return err
			}
		case DunningGrace:
			if now.Sub(d.EnteredAt) >= time.Duration(graceDays)*24*time.Hour {
				if err := s.repo.upsertDunning(int(u.UserID), DunningFrozen, now); err != nil {
					return err
				}
			}
		case DunningFrozen:
			if now.Sub(d.EnteredAt) >= time.Duration(frozenDays)*24*time.Hour {
				if err := s.repo.upsertDunning(int(u.UserID), DunningRecycled, now); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
```

- [ ] **Step 4：`billing.go` 导出跨模块门面**

```go
package billing

import "manager-backend/modules/billing/internal"

// 跨模块门面（供 phone 等调用；billing 不反向依赖任何业务模块）。

// EnforcementTarget 欠费执行目标。
type EnforcementTarget = internal.EnforcementTarget

func TryOccupyInstanceSeat(userID int) error { return internal.SeatService.TryOccupyInstanceSeat(userID) }
func ReleaseInstanceSeat(userID int) error   { return internal.SeatService.ReleaseInstanceSeat(userID) }
func ReconcileInstanceSeats(userID, count int) error {
	return internal.SeatService.ReconcileInstanceSeats(userID, count)
}
func IsFrozen(userID int) (bool, error)             { return internal.SeatService.IsFrozen(userID) }
func InstanceSeatCapacity(userID int) (int64, error) { return internal.SeatService.InstanceSeatCapacity(userID) }
func ListDunningEnforcement() ([]internal.EnforcementTarget, error) {
	return internal.SeatService.ListDunningEnforcement()
}
```
> 注意：`billing.go` 原为 `import _ ".../internal"`（blank）。改为具名 import 以委托。`internal.SeatService`/`internal.EnforcementTarget` 需为 internal 包导出标识符（`SeatService` 已大写导出；`EnforcementTarget` 已导出）。**这要求 internal 可被公开包 import**——公开包 `billing` import 自己的 `internal` 是允许的（只有「别的模块」不能 import 你的 internal）。

- [ ] **Step 5：`module.go` Init 装配 + 建表；`main_test.go` AutoMigrate**

`Init` 追加：`SeatService = newSeatService(newSeatRepository(db), EntitlementService)`（在 EntitlementService 之后）。
`RegisterSetup` AutoMigrate 追加 `&SeatUsage{}, &DunningState{}`（cron_locks 由 framework 注册，无需此处）。
`main_test.go` AutoMigrate 追加 `&SeatUsage{}, &DunningState{}, &framework.CronLock{}`（cron 测试可能需要；`framework` 已 import）。

- [ ] **Step 6：创建 `seat_service_test.go`**

```go
package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const seatUser = 9401

func grantSeats(t *testing.T, userID int, n int64, expireAt *time.Time) {
	t.Helper()
	_, err := EntitlementService.Grant(userID, SubjectInstanceSeat, n, expireAt, SourceAdjust, "test", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)
}

func TestOccupyReleaseGating(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	grantSeats(t, seatUser, 2, nil) // 容量 2

	require.NoError(t, SeatService.TryOccupyInstanceSeat(seatUser)) // 1/2
	require.NoError(t, SeatService.TryOccupyInstanceSeat(seatUser)) // 2/2
	err := SeatService.TryOccupyInstanceSeat(seatUser)             // 超容量 → 拒
	assert.Error(t, err)

	require.NoError(t, SeatService.ReleaseInstanceSeat(seatUser)) // 1/2
	require.NoError(t, SeatService.TryOccupyInstanceSeat(seatUser)) // 2/2 再次可占
}

func TestReconcileAndCapacity(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	grantSeats(t, seatUser, 5, nil)
	cap, err := SeatService.InstanceSeatCapacity(seatUser)
	require.NoError(t, err)
	assert.Equal(t, int64(5), cap)
	require.NoError(t, SeatService.ReconcileInstanceSeats(seatUser, 3))
	used, _ := SeatService.repo.usage(seatUser)
	assert.Equal(t, int64(3), used)
}

func TestDunningStateMachine(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	// 容量 1，占用 2 → 超量
	grantSeats(t, seatUser, 1, nil)
	require.NoError(t, SeatService.ReconcileInstanceSeats(seatUser, 2))

	// 第一次 runDunning：active → grace
	require.NoError(t, SeatService.runDunning(3, 7))
	d, _ := SeatService.repo.getDunning(seatUser)
	assert.Equal(t, DunningGrace, d.State)

	frozen, _ := SeatService.IsFrozen(seatUser)
	assert.False(t, frozen) // grace 不算冻结

	// 把 grace 进入时间改到 4 天前 → 再跑 → frozen
	past := time.Now().Add(-4 * 24 * time.Hour)
	require.NoError(t, SeatService.repo.upsertDunning(seatUser, DunningGrace, past))
	require.NoError(t, SeatService.runDunning(3, 7))
	d, _ = SeatService.repo.getDunning(seatUser)
	assert.Equal(t, DunningFrozen, d.State)
	frozen, _ = SeatService.IsFrozen(seatUser)
	assert.True(t, frozen)

	// frozen 进入时间改到 8 天前 → 再跑 → recycled
	require.NoError(t, SeatService.repo.upsertDunning(seatUser, DunningFrozen, time.Now().Add(-8*24*time.Hour)))
	require.NoError(t, SeatService.runDunning(3, 7))
	d, _ = SeatService.repo.getDunning(seatUser)
	assert.Equal(t, DunningRecycled, d.State)

	// 执行目标列表含该用户
	targets, err := SeatService.ListDunningEnforcement()
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, seatUser, targets[0].UserID)
	assert.Equal(t, int64(1), targets[0].Capacity)

	// 补足容量(再发 1 席位 → 容量 2 ≥ 占用 2) → 跑 → 回 active
	grantSeats(t, seatUser, 1, nil)
	require.NoError(t, SeatService.runDunning(3, 7))
	d, _ = SeatService.repo.getDunning(seatUser)
	assert.Equal(t, DunningActive, d.State)
}
```

- [ ] **Step 7：测试 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestOccupy|TestReconcile|TestDunning' -v && go test ./modules/billing/... -v && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing
git commit -m "feat(billing): 席位占用门禁 + 欠费状态机 + 跨模块门面

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> 本 Task 合并了原计划 Task2(占用)+Task3(欠费状态机)，因 `runDunning`/`IsFrozen`/`Enforcement` 与占用同属 SeatService、一并测更顺。dunning cron 的 PeriodicRunner 装配在 Task 3。

---

## Task 3：欠费 cron 装配（PeriodicRunner）+ 配置

**Files:** Create `billing/internal/dunning.go`; Modify `module.go`(OnStart/OnStop), `framework/config.go`(可选配置项)。

- [ ] **Step 1：创建 `dunning.go`**

```go
package billing

import (
	"time"

	"manager-backend/framework"
)

// 欠费保护默认参数（可由 config 覆盖，见 Step 3）。
const (
	dunningInterval  = time.Hour
	dunningLease     = 10 * time.Minute
	defaultGraceDays = 3
	defaultFrozenDays = 7
)

var dunningRunner *framework.PeriodicRunner

// startDunning 启动欠费保护 cron（HA 租约锁，每小时一个实例执行）。
func startDunning(db interface{ /* *gorm.DB */ }) {}

```
> 上面 `startDunning` 仅占位说明；**真实实现**：在 `module.go` 的 `OnStart` 直接用 `framework.NewPeriodicRunner` 装配（见 Step 2），`dunning.go` 只保留常量 + runner 变量。最终 `dunning.go`：

```go
package billing

import (
	"time"

	"manager-backend/framework"
)

const (
	dunningInterval   = time.Hour
	dunningLease      = 10 * time.Minute
	defaultGraceDays  = 3
	defaultFrozenDays = 7
)

var dunningRunner *framework.PeriodicRunner
```

- [ ] **Step 2：`module.go` OnStart/OnStop 装配 cron**

`OnStart`：
```go
func (m *billingModule) OnStart() error {
	if SeatService != nil {
		dunningRunner = framework.NewPeriodicRunner(m.db, "billing:dunning", dunningInterval, dunningLease, func() error {
			return SeatService.runDunning(defaultGraceDays, defaultFrozenDays)
		})
		dunningRunner.Start()
	}
	return nil
}

func (m *billingModule) OnStop() error {
	if dunningRunner != nil {
		dunningRunner.Stop()
		dunningRunner = nil
	}
	return nil
}
```
`billingModule` 需有 `db *gorm.DB` 字段，`Init` 里 `m.db = db`。`module.go` import 加 `framework`（已有）与 `time`（若需）。

- [ ] **Step 3：构建 + 全量测试 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go vet ./... && go build ./... && go test ./... 2>&1 | tail -15
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing
git commit -m "feat(billing): 欠费保护 cron 装配(租约锁周期触发)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> X/Y 暂用常量(3/7)；后续可接 config。OnStart 起 cron——HA 下多实例都起 runner，但租约锁保证每 tick 仅一个执行。

---

## Task 4：phone 创建/销毁/开机 接入 billing 门禁

**Files:** Modify `phone/internal/service.go`(Create/Delete/Power), `phone/internal/module.go`(import billing)。

- [ ] **Step 1：写失败测试（追加 `phone/internal/service_test.go`）**

> phone 测试默认 `ops==nil`（本地降级）。门禁需 billing 表 + SeatService 装配。在 phone 的 `main_test.go` 增加 billing 表 AutoMigrate + `(&billingModule{})`... 不可——phone 不能 import billing internal。改为：门禁调用走 `billing` 公开包，phone 测试需 billing 已装配。**简化**：phone 单测覆盖「无 billing 装配时 Create 仍可用(降级)」不现实。**采用集成验证**：门禁逻辑的正确性由 billing 的 `TestOccupy*` 覆盖；phone 接入用 `go build` + 一条 phone 服务测试验证「容量内可创建、超容量被拒」——需在 phone main_test 装配 billing。

在 `phone/internal/main_test.go` 里，通过 **billing 公开包**初始化（phone 可 import billing 公开包）：AutoMigrate billing 所需表并调用 billing 的测试装配入口。**为此在 billing 公开包加一个测试装配助手** `billing.InitForTest(db)`（见下）。

先在 `backend/modules/billing/billing.go` 增加：
```go
// InitForTest 供其他模块的测试装配 billing（AutoMigrate + Init）。仅测试用。
func InitForTest(db *gorm.DB) error { return internal.InitForTest(db) }
```
并在 `billing/internal/module.go` 加：
```go
// InitForTest 测试装配：建表 + 装配服务。
func InitForTest(db *gorm.DB) error {
	if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{},
		&Order{}, &OrderItem{}, &TrialPolicy{}, &TrialGrant{}, &TrialEligibility{}, &SeatUsage{}, &DunningState{}); err != nil {
		return err
	}
	return (&billingModule{}).Init(db)
}
```
（`billing.go` import 需含 `gorm.io/gorm`。）

phone `main_test.go` TestMain 调 `billing.InitForTest(framework.DB)`（在 phone 自身 AutoMigrate 之后）。phone test 追加：
```go
func TestCreateGatedByInstanceSeat(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	const u = 9501
	// 无席位 → 创建被拒
	_, err := PhoneService.Create(u, &CloudPhoneCreate{Name: "x"})
	assert.Error(t, err)

	// 发 1 席位 → 可创建 1 台，第 2 台被拒
	_, err = billing.GrantInstanceSeatsForTest(u, 1)
	require.NoError(t, err)
	_, err = PhoneService.Create(u, &CloudPhoneCreate{Name: "a"})
	require.NoError(t, err)
	_, err = PhoneService.Create(u, &CloudPhoneCreate{Name: "b"})
	assert.Error(t, err)
}
```
为此 billing 公开包再加测试助手 `GrantInstanceSeatsForTest(userID, n)`：
```go
// billing.go
func GrantInstanceSeatsForTest(userID, n int) (interface{}, error) {
	return internal.EntitlementService.Grant(userID, internal.SubjectInstanceSeat, int64(n), nil, internal.SourceAdjust, "test", internal.LedgerAdjustGrant, "staff:1")
}
```
（`SubjectInstanceSeat`/`SourceAdjust`/`LedgerAdjustGrant` 已在 internal 导出。）phone test import `manager-backend/modules/billing`。

- [ ] **Step 2：Create 接入门禁（`service.go`）**

在 `Create` 函数**最开头**（构造 item 前）加占用门禁，并在所有失败返回前释放：
```go
func (s *serviceImpl) Create(userID int, req *CloudPhoneCreate) (*CloudPhone, error) {
	if err := billing.TryOccupyInstanceSeat(userID); err != nil {
		return nil, err
	}
	item := CloudPhone{ ... } // 原样
	if s.ops == nil {
		item.Status = StatusCreated
		if err := s.repo.create(&item); err != nil {
			_ = billing.ReleaseInstanceSeat(userID) // 回滚占用
			return nil, err
		}
		return &item, nil
	}
	...
	res, err := s.ops.Create(ctx, ...)
	if err != nil {
		_ = billing.ReleaseInstanceSeat(userID)
		return nil, apperr.Internal("创建云手机失败：" + err.Error())
	}
	...
	if err := s.repo.create(&item); err != nil {
		_ = billing.ReleaseInstanceSeat(userID)
		return nil, err
	}
	... // 建任务、return
}
```
`service.go` import 加 `"manager-backend/modules/billing"`。

- [ ] **Step 3：Delete 删本地档案处释放席位**

`Delete` 的**立即删档路径**（`p.CpID=="" || s.ops==nil` → `return s.repo.delete(userID, id)`）改为删后释放：
```go
	if p.CpID == "" || s.ops == nil {
		if err := s.repo.delete(userID, id); err != nil {
			return err
		}
		_ = billing.ReleaseInstanceSeat(userID)
		return nil
	}
```
> 异步销毁路径（worker 最终删档）的释放在 Task 5 worker 改造里统一加（worker 删档后 ReleaseInstanceSeat）。本 Task 仅立即删档路径。

- [ ] **Step 4：Power 开机门禁（`service.go`）**

`Power(userID, id, operation)` 中，开机分支（operation 表示开机时；参考现有常量，如 `operation == "start"` 或 `OpStart`——按现有代码判定）调用前加：
```go
	// 开机门禁：冻结用户禁止开机
	if isStartOp(operation) {
		frozen, err := billing.IsFrozen(userID)
		if err != nil {
			return err
		}
		if frozen {
			return apperr.Forbidden("账户已冻结，无法开机，请续费实例席位")
		}
	}
```
> 实现者需读 `Power` 现有代码确定「开机」判定（operation 取值/常量），用对应判断替换 `isStartOp(operation)`。若已有 `TaskTypeStart`/字符串 "start"，据实使用。

- [ ] **Step 5：测试 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/phone/internal/ -run 'TestCreateGated' -v && go test ./... 2>&1 | tail -15 && gofmt -l modules/ && go vet ./... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules
git commit -m "feat(phone): 创建/开机接入 billing 席位与冻结门禁

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5：phone 执行 worker（冻结强制关机 / 回收销毁超量实例）⚠️ 含不可逆操作

**Files:** Create `phone/internal/enforce.go`; Modify `phone/internal/worker.go`(异步删档释放席位 + 起 enforcement)、`phone/internal/module.go`、`phone/internal/repository.go`(按用户列实例/计数，如缺)。

- [ ] **Step 1：阅读 worker 异步删档处**，在 worker 最终删除本地档案（销毁确认）后追加 `_ = billing.ReleaseInstanceSeat(userID)`。

- [ ] **Step 2：创建 `enforce.go`（读 billing 执行）**

```go
package phone

import (
	"context"
	"time"

	"manager-backend/modules/billing"
)

// runEnforcement 读 billing 欠费执行目标：frozen→强制关机运行中实例；recycled→销毁超量实例。
// 并对涉及用户做席位占用校正(reconcile)。幂等：关机/销毁均按状态/数量条件执行。
func (s *serviceImpl) runEnforcement(ctx context.Context) {
	if s.ops == nil {
		return // 本地降级无中台，跳过
	}
	targets, err := billing.ListDunningEnforcement()
	if err != nil {
		return
	}
	for _, tgt := range targets {
		phones, err := s.repo.listByUser(tgt.UserID)
		if err != nil {
			continue
		}
		// 占用校正（以 phone 真实计数为准）
		_ = billing.ReconcileInstanceSeats(tgt.UserID, len(phones))

		switch tgt.State {
		case billing.DunningFrozen:
			// 强制关机所有运行中实例
			for _, p := range phones {
				if p.Status == StatusRunning && p.CpID != "" {
					_ = s.ops.Shutdown(ctx, p.CpID)
					_ = s.repo.setStatus(p.ID, StatusStopping)
				}
			}
		case billing.DunningRecycled:
			// 销毁超量实例：保留容量内最早创建的，销毁其余(最近创建优先回收)
			over := len(phones) - int(tgt.Capacity)
			if over <= 0 {
				continue
			}
			// phones 按 id 升序；回收末尾 over 台(最近创建)
			victims := phones[len(phones)-over:]
			for _, p := range victims {
				if p.CpID != "" {
					_ = s.ops.Destroy(ctx, p.CpID)
				}
				if err := s.repo.deleteByID(p.ID); err == nil {
					_ = billing.ReleaseInstanceSeat(tgt.UserID)
				}
			}
		}
	}
}
```
> 需要的 repo 方法：`listByUser(userID int) ([]CloudPhone, error)`（按 user 升序 id）、`deleteByID(id uint) error`。若不存在，在 `repository.go` 增加。`billing.DunningFrozen`/`DunningRecycled` 常量需在 billing 公开包导出（加 `const DunningFrozen = internal.DunningFrozen` 等，或直接比较字符串 "frozen"/"recycled"——推荐导出常量）。

billing.go 追加导出：
```go
const (
	DunningFrozen   = internal.DunningFrozen
	DunningRecycled = internal.DunningRecycled
)
```

- [ ] **Step 3：起 enforcement runner（worker.go / module.go）**

phone `OnStart` 再起一个 PeriodicRunner（独立租约锁名 `phone:enforcement`，间隔如 1 分钟、lease 50s）：
```go
// module.go OnStart 内，taskWorker 之后
if PhoneService != nil && PhoneService.ops != nil {
	enforceRunner = framework.NewPeriodicRunner(m.db, "phone:enforcement", time.Minute, 50*time.Second, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		PhoneService.runEnforcement(ctx)
		return nil
	})
	enforceRunner.Start()
}
```
`OnStop` 停 `enforceRunner`。module.go 加 `enforceRunner *framework.PeriodicRunner` 包级变量 + import `framework`/`time`/`context`。

- [ ] **Step 4：写测试（`enforce_test.go` 或追加 service_test）**

> 真实 destroy 调中台，单测用 `ops==nil` 跳过。为测 enforcement 逻辑，用一个 **fake ops**（midplat port 接口的测试替身，记录 Shutdown/Destroy 调用），或验证「无 ops 时 runEnforcement 安全返回」+ 用 billing 状态机 + repo 计数验证 reconcile。实现者据 phone 现有测试替身能力选择：
> - 若 phone 已有 midplat port 接口可注入 fake（检查 `op_api.go`/`midplat.go`），写 fake 记录 destroy 的 cpIds，断言 recycled 用户的超量实例被销毁、席位释放、reconcile 生效。
> - 若无现成替身，至少写：recycled 用户 + N 台实例 + 容量 C，调 `runEnforcement`（需 ops 非 nil → 用 fake），断言销毁 N-C 台、本地档案删除、`SeatService.repo.usage` 校正。

最小可行测试（fake ops）：
```go
func TestEnforcementRecyclesOverQuota(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	const u = 9601
	fake := &fakeOps{} // 实现 op port，记录 Destroy/Shutdown
	svc := newService(newRepository(framework.DB), fake)
	// 造 3 台实例(带 cpId)，容量 1，状态置 recycled
	for i := 0; i < 3; i++ {
		require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "p", CpID: "cp" + itoaT(i), Status: StatusStopped}).Error)
	}
	_, _ = billing.GrantInstanceSeatsForTest(u, 1)
	require.NoError(t, billing.SetDunningForTest(u, "recycled")) // 测试助手
	svc.runEnforcement(context.Background())
	// 销毁 2 台(3-1)，剩 1 台
	var n int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ?", u).Count(&n)
	assert.Equal(t, int64(1), n)
	assert.Len(t, fake.destroyed, 2)
}
```
为此 billing 公开包加测试助手 `SetDunningForTest(userID int, state string) error`（委托 `internal.SeatService.repo.upsertDunning`，经 internal 导出一个 `SetDunningForTest`）。`fakeOps` 实现 phone 的 op 接口（实现者按 `op_api.go` 接口补齐其余方法为空实现）。`itoaT` 用 `strconv.Itoa`。

- [ ] **Step 5：全量回归 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./... 2>&1 | tail -20 && go test ./framework/ -run TestModuleBoundaries -v 2>&1 | tail -5 && gofmt -l modules/ framework/ && go vet ./... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend
git commit -m "feat(phone): 欠费执行 worker(冻结强制关机/回收销毁超量实例)+席位校正

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 6 验收对照（PRD §4.2/§10.1）

| 需求 | 覆盖 |
|---|---|
| HA 多实例 cron 单实例触发（无额外基建） | Task 1 租约锁 + phone worker 改造 |
| 实例创建门禁（席位不足拦截） | Task 2 TryOccupy + Task 4 Create 接入（AC-2） |
| 开机门禁（冻结禁开机） | Task 2 IsFrozen + Task 4 Power 接入 |
| 欠费保护状态机 active→grace→frozen→recycled（cron, X/Y 可配） | Task 2 runDunning + Task 3 cron 装配（AC-10） |
| 冻结强制关机 | Task 5 runEnforcement frozen 分支 |
| 回收销毁超量实例（不可逆） | Task 5 runEnforcement recycled 分支 |
| 续费/补容量后自动恢复 active | Task 2 runDunning（over==false → active） |
| 占用漂移自愈 | Task 5 ReconcileInstanceSeats |

> 单向依赖 phone→billing 全程保持；arch_test 边界须绿。真实回收销毁不可逆——Task 5 需加强对抗审查（fake ops 验证销毁的是「超量且正确的那几台」、幂等、不误删容量内实例）。

## Self-Review 记录

- **Spec 覆盖**：覆盖 PRD §10.1 欠费状态机 + §4.2 席位、HA cron 机制。
- **占位扫描**：Task 3 `dunning.go` 给了「占位说明 + 最终实现」两段，实现者取最终实现段；其余完整。Task 4/5 含「实现者据现有代码判定」处（Power 开机判定、op 接口 fake、repo 方法是否存在）——这些是必要的代码勘探点，已明确指引。
- **类型一致**：`SeatUsage/DunningState/EnforcementTarget`、`SeatService` 方法、`framework.TryRunLocked/PeriodicRunner/CronLock/InstanceID`、billing 公开门面（TryOccupy/Release/Reconcile/IsFrozen/InstanceSeatCapacity/ListDunningEnforcement + Dunning* 常量 + 测试助手）贯穿一致；状态常量 active/grace/frozen/recycled 统一。
- **依赖方向**：phone→billing 单向（billing 公开包导出门面；billing 不 import phone）。arch_test 须验证。
- **事务/幂等/并发**：占用 occupy 守卫式原子 UPDATE(used<capacity)；release 守卫(used>0)；dunning 转换幂等(按 state+时间)；cron 走租约锁单实例；enforcement 幂等(按状态/数量)。
- **不可逆**：回收销毁 Task 5 单列 + 加强审查；victims 取「最近创建的超量部分」，保留容量内最早创建实例。
