# 计费系统 Phase 1 · 计划 3：权益批次（资源包） Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `billing` 模块加入「资源包/权益」：`instance_seat`（实例席位）/`boot_seat`（开机席位）/`runtime_minute`（时长分钟）三类**批次**（带到期、可用容量），统一流水扩展到资源科目，运营手动赠送/扣减扩展到资源包，前台账户/资源概览可见容量与批次。

**Architecture:** 复用 `billing/internal` 单包（`entitlement_*.go` 前缀分子域）。资源以**批次 EntitlementBatch** 持有：`可用容量(subject) = Σ(quantity − used) 于 未过期且 quantity>used 的批次`。席位类（instance_seat/boot_seat）为容量型（`used` 恒 0，占用由实例/运行态实时判定，见 Plan 6）；时长分钟为消耗型（Phase 2 计量增 `used`）。统一流水沿用 Plan 1 `LedgerEntry`，本计划把其 `DeltaCents/BalanceAfterCents` **泛化重命名为 `Delta/BalanceAfter`**（balance 科目=分；资源科目=台/分钟），使一张流水表覆盖货币与资源。Grant/Deduct 与流水写入同事务。

**Tech Stack:** Go 1.26 / Gin / GORM（sqlite 测试）/ testify。模块名 `manager-backend`，分支 `feature/billing-entitlement`。

---

## 领域模型

- **EntitlementBatch** `billing_entitlement_batches`：`{user_id, subject(instance_seat/boot_seat/runtime_minute), quantity, used, expire_at(*time, nil=永久), source(order/grant/adjust/trial), source_ref(订单号/理由), 时间戳}`。
- **可用容量** `Capacity(userID, subject) = Σ(quantity − used)`，仅计 `未过期(expire_at IS NULL OR expire_at > now)` 且 `quantity > used` 的批次。
- **Grant**（赠送/购买/试用）：新增一条批次 + 写流水（`Delta=+quantity, BalanceAfter=容量`）。
- **Deduct**（扣减/退款/Phase2 消耗）：按**临近到期优先 FIFO** 跨批次累加 `used` 直至扣满；容量不足报错 + 写流水（`Delta=-amount`）。
- 统一流水 `LedgerEntry`（Plan 1）：本计划字段改名 `Delta`/`BalanceAfter`；资源操作写 `Subject=资源科目`。

## 计划 3 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `internal/model.go` | `LedgerEntry` 字段 `DeltaCents/BalanceAfterCents`→`Delta/BalanceAfter` | 修改 |
| `internal/repository.go` | `applyBalance` 写流水字段改名 | 修改 |
| `internal/entitlement_model.go` | `EntitlementBatch`、资源科目常量、`CapacitySnapshot` | 创建 |
| `internal/entitlement_repository.go` | `entitlementRepository`：建批次/查活跃批次/容量/列批次/消耗/写流水 | 创建 |
| `internal/entitlement_service.go` | `EntitlementService`：Grant/Deduct/Capacity/Capacities/ListBatches/AdjustResource | 创建 |
| `internal/entitlement_api.go` | 前台 entitlements；后台资源调整 | 创建 |
| `internal/entitlement_service_test.go` | 服务层测试 | 创建 |
| `internal/api.go` | `AdminGetAccount` handler 追加 capacities | 修改 |
| `internal/module.go` | Init 装配 `EntitlementService`；建表加批次表；后台/前台路由 | 修改 |
| `internal/main_test.go` | AutoMigrate 加 `EntitlementBatch` | 修改 |
| `internal/service_test.go` | Plan1 断言 `BalanceAfterCents`→`BalanceAfter` | 修改 |

**约定速查**：`framework.OK*/Fail/FailErr`；`apperr.Validation/NotFound`；前台 `user.AuthMiddleware()`；后台 `staff.PermissionMiddleware("billing:view"|"billing:manage")`；Go 代码可用 `time.Now()`；测试 `framework.CleanTable`。

---

## Task 1：统一流水字段泛化（Delta / BalanceAfter）

**Files:** Modify `model.go`, `repository.go`, `service_test.go`。

- [ ] **Step 1：改 `model.go` 的 `LedgerEntry`**

把 `LedgerEntry` 的两行字段：
```go
	DeltaCents        int64     `gorm:"not null" json:"delta_cents"`                // 正=增 负=减（科目=balance 时单位为分）
	BalanceAfterCents int64     `gorm:"not null" json:"balance_after_cents"`        // 变更后余额快照（审计/对账）
```
改为：
```go
	Delta       int64 `gorm:"not null" json:"delta"`        // 增减量：balance 科目=分；资源科目=台/分钟（正=增 负=减）
	BalanceAfter int64 `gorm:"not null" json:"balance_after"` // 变更后余量快照：balance=分；资源=可用容量
```

- [ ] **Step 2：改 `repository.go` 的 `applyBalance` 写流水**

把构造 `LedgerEntry` 的：
```go
		entry := LedgerEntry{
			UserID: uint(userID), Subject: SubjectBalance, Type: typ,
			DeltaCents: delta, BalanceAfterCents: acc.BalanceCents,
			Reason: reason, OrderID: orderID, Operator: operator,
		}
```
改为：
```go
		entry := LedgerEntry{
			UserID: uint(userID), Subject: SubjectBalance, Type: typ,
			Delta: delta, BalanceAfter: acc.BalanceCents,
			Reason: reason, OrderID: orderID, Operator: operator,
		}
```

- [ ] **Step 3：改 `service_test.go` 断言**

把 `TestTopupAndAdjustBalanceWithLedger` 里：
```go
	assert.Equal(t, int64(12000), list[0].BalanceAfterCents) // id DESC，最新在前
```
改为：
```go
	assert.Equal(t, int64(12000), list[0].BalanceAfter) // id DESC，最新在前
```

- [ ] **Step 4：验证（无其他引用残留）**

Run: `cd /home/root/workspace005/gloryphone-code/backend && grep -rn "DeltaCents\|BalanceAfterCents" modules/billing/`
Expected: 无输出（全部已改名）。

- [ ] **Step 5：测试 + 质量门**

Run: `cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...`
Expected: 全 PASS、无格式/vet 问题、build 0。

- [ ] **Step 6：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "refactor(billing): 统一流水字段泛化 Delta/BalanceAfter(覆盖货币与资源)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2：EntitlementBatch 模型 + 仓储（容量）

**Files:** Create `entitlement_model.go`, `entitlement_repository.go`; Modify `module.go`(建表), `main_test.go`(AutoMigrate); Test `entitlement_service_test.go`（仓储层经服务暂未就绪，本 Task 直接测仓储）。

- [ ] **Step 1：创建 `entitlement_model.go`**

```go
package billing

import "time"

// 资源科目（统一流水 Subject 取值之一；与 SubjectBalance 并列）
const (
	SubjectInstanceSeat  = "instance_seat"  // 实例席位（容量型，台）
	SubjectBootSeat      = "boot_seat"      // 并发开机席位（容量型，台）
	SubjectRuntimeMinute = "runtime_minute" // 时长包余额（消耗型，分钟）
)

// 批次来源
const (
	SourceOrder = "order"
	SourceGrant = "grant"
	SourceAdjust = "adjust"
	SourceTrial = "trial"
)

// resourceSubjects 合法的资源科目集合。
var resourceSubjects = map[string]bool{
	SubjectInstanceSeat: true, SubjectBootSeat: true, SubjectRuntimeMinute: true,
}

// EntitlementBatch 资源批次：一次发放的一份额度（带到期）。
// 可用容量(subject) = Σ(Quantity - Used) 于 未过期且 Quantity>Used 的批次。
type EntitlementBatch struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint       `gorm:"not null;index:idx_billing_ent_user_subject" json:"user_id"`
	Subject   string     `gorm:"type:varchar(20);not null;index:idx_billing_ent_user_subject" json:"subject"`
	Quantity  int64      `gorm:"not null" json:"quantity"`              // 台数 或 分钟数
	Used      int64      `gorm:"not null;default:0" json:"used"`        // 已消耗（席位类恒0；时长 Phase2 增）
	ExpireAt  *time.Time `json:"expire_at"`                             // nil=永久
	Source    string     `gorm:"type:varchar(20)" json:"source"`        // order/grant/adjust/trial
	SourceRef string     `gorm:"type:varchar(255)" json:"source_ref"`   // 订单号 或 理由
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (EntitlementBatch) TableName() string { return "billing_entitlement_batches" }

// CapacitySnapshot 某用户三类资源的可用容量概览。
type CapacitySnapshot struct {
	InstanceSeat  int64 `json:"instance_seat"`
	BootSeat      int64 `json:"boot_seat"`
	RuntimeMinute int64 `json:"runtime_minute"`
}
```

- [ ] **Step 2：`main_test.go` AutoMigrate 加批次表**

把 AutoMigrate 行改为：
```go
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}); err != nil {
```

- [ ] **Step 3：`module.go` 建表加批次表**

把 `RegisterSetup` 内 AutoMigrate 改为：
```go
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}); err != nil {
			return err
		}
```

- [ ] **Step 4：创建 `entitlement_repository.go`**

```go
package billing

import (
	"time"

	"gorm.io/gorm"
)

type entitlementRepository interface {
	createBatch(b *EntitlementBatch) error
	capacity(userID int, subject string, now time.Time) (int64, error)
	listActiveBatches(userID int, subject string, now time.Time) ([]EntitlementBatch, error)
	listBatches(userID int, subject string) ([]EntitlementBatch, error)
	addUsed(id int, delta int64) error
	insertLedger(e *LedgerEntry) error
	txWith(fn func(txRepo entitlementRepository) error) error
}

type gormEntitlementRepository struct{ db *gorm.DB }

func newEntitlementRepository(db *gorm.DB) entitlementRepository {
	return &gormEntitlementRepository{db: db}
}

func (r *gormEntitlementRepository) createBatch(b *EntitlementBatch) error { return r.db.Create(b).Error }

// activeScope 未过期且仍有余量的批次。
func (r *gormEntitlementRepository) activeScope(userID int, subject string, now time.Time) *gorm.DB {
	return r.db.Model(&EntitlementBatch{}).
		Where("user_id = ? AND subject = ? AND quantity > used AND (expire_at IS NULL OR expire_at > ?)", userID, subject, now)
}

func (r *gormEntitlementRepository) capacity(userID int, subject string, now time.Time) (int64, error) {
	var cap int64
	err := r.activeScope(userID, subject, now).
		Select("COALESCE(SUM(quantity - used), 0)").Scan(&cap).Error
	return cap, err
}

// listActiveBatches 临近到期优先（永久批次最后），用于 FIFO 消耗。
func (r *gormEntitlementRepository) listActiveBatches(userID int, subject string, now time.Time) ([]EntitlementBatch, error) {
	var items []EntitlementBatch
	err := r.activeScope(userID, subject, now).
		Order("expire_at IS NULL, expire_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormEntitlementRepository) listBatches(userID int, subject string) ([]EntitlementBatch, error) {
	q := r.db.Model(&EntitlementBatch{}).Where("user_id = ?", userID)
	if subject != "" {
		q = q.Where("subject = ?", subject)
	}
	var items []EntitlementBatch
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormEntitlementRepository) addUsed(id int, delta int64) error {
	return r.db.Model(&EntitlementBatch{}).Where("id = ?", id).
		UpdateColumn("used", gorm.Expr("used + ?", delta)).Error
}

func (r *gormEntitlementRepository) insertLedger(e *LedgerEntry) error { return r.db.Create(e).Error }

// txWith 在事务内执行 fn，fn 收到一个绑定到事务的 repo。
func (r *gormEntitlementRepository) txWith(fn func(txRepo entitlementRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&gormEntitlementRepository{db: tx})
	})
}
```

- [ ] **Step 5：创建 `entitlement_service_test.go`（仓储容量测试）**

```go
package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const entUser = 8101

func TestEntitlementCapacityExcludesExpiredAndUsed(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })
	repo := newEntitlementRepository(framework.DB)
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	// 活跃：5 台（未过期）
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 5, ExpireAt: &future}))
	// 永久：3 台
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 3}))
	// 已过期：10 台（不计）
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 10, ExpireAt: &past}))
	// 已用尽：2 台 quantity=used（不计）
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 2, Used: 2, ExpireAt: &future}))

	cap, err := repo.capacity(entUser, SubjectInstanceSeat, now)
	require.NoError(t, err)
	assert.Equal(t, int64(8), cap) // 5 + 3

	// 其它科目为 0
	cap, err = repo.capacity(entUser, SubjectRuntimeMinute, now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), cap)
}
```

- [ ] **Step 6：测试 → PASS**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run TestEntitlementCapacityExcludesExpiredAndUsed -v`

- [ ] **Step 7：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./... && go test ./modules/billing/internal/ -v
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 权益批次模型 + 仓储(容量/活跃批次/FIFO 基础)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3：EntitlementService Grant / Deduct（FIFO）+ 流水

**Files:** Create `entitlement_service.go`; Modify `module.go`(Init 装配); Test 追加。

- [ ] **Step 1：写失败测试（追加到 `entitlement_service_test.go`）**

```go
func TestEntitlementGrantDeductWithLedger(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })

	// 赠送 10 台实例席位（永久）
	_, err := EntitlementService.Grant(entUser, SubjectInstanceSeat, 10, nil, SourceAdjust, "运营赠送", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)
	cap, err := EntitlementService.Capacity(entUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(10), cap)

	// 再赠送 600 分钟时长
	_, err = EntitlementService.Grant(entUser, SubjectRuntimeMinute, 600, nil, SourceAdjust, "补偿", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)

	// 扣减 3 台
	require.NoError(t, EntitlementService.Deduct(entUser, SubjectInstanceSeat, 3, "回收", LedgerAdjustDeduct, "staff:1"))
	cap, err = EntitlementService.Capacity(entUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(7), cap)

	// 扣减超额 → 报错
	err = EntitlementService.Deduct(entUser, SubjectInstanceSeat, 999, "越扣", LedgerAdjustDeduct, "staff:1")
	assert.Error(t, err)

	// 流水：2 笔 grant + 1 笔 deduct（资源科目）
	_, total, err := BillingService.ListLedger(entUser, 1, 50, SubjectInstanceSeat, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total) // instance_seat：1 grant + 1 deduct

	// 容量快照
	snap, err := EntitlementService.Capacities(entUser)
	require.NoError(t, err)
	assert.Equal(t, int64(7), snap.InstanceSeat)
	assert.Equal(t, int64(600), snap.RuntimeMinute)
	assert.Equal(t, int64(0), snap.BootSeat)
}

func TestEntitlementFifoConsumeAcrossBatches(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })
	now := time.Now()
	soon := now.Add(1 * time.Hour)
	later := now.Add(48 * time.Hour)

	// 两批时长：先到期的 100，后到期的 100
	_, err := EntitlementService.Grant(entUser, SubjectRuntimeMinute, 100, &soon, SourceAdjust, "批A", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)
	_, err = EntitlementService.Grant(entUser, SubjectRuntimeMinute, 100, &later, SourceAdjust, "批B", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)

	// 扣 150 → 先扣光临近到期的 100，再从后到期的扣 50
	require.NoError(t, EntitlementService.Deduct(entUser, SubjectRuntimeMinute, 150, "消耗", LedgerConsume, "system"))

	batches, err := EntitlementService.ListBatches(entUser, SubjectRuntimeMinute)
	require.NoError(t, err)
	require.Len(t, batches, 2)
	// 找到 soon 批应被扣满
	var soonUsed, laterUsed int64
	for _, b := range batches {
		if b.SourceRef == "批A" {
			soonUsed = b.Used
		} else if b.SourceRef == "批B" {
			laterUsed = b.Used
		}
	}
	assert.Equal(t, int64(100), soonUsed)
	assert.Equal(t, int64(50), laterUsed)
}

func TestEntitlementGuards(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })
	_, err := EntitlementService.Grant(entUser, "nope", 1, nil, SourceAdjust, "x", LedgerAdjustGrant, "staff:1")
	assert.Error(t, err) // 非法科目
	_, err = EntitlementService.Grant(entUser, SubjectInstanceSeat, 0, nil, SourceAdjust, "x", LedgerAdjustGrant, "staff:1")
	assert.Error(t, err) // 数量<=0
	err = EntitlementService.Deduct(entUser, SubjectInstanceSeat, 0, "x", LedgerAdjustDeduct, "staff:1")
	assert.Error(t, err) // 扣减<=0
}
```

- [ ] **Step 2：测试 → FAIL（EntitlementService 未定义）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestEntitlement' -v`

- [ ] **Step 3：创建 `entitlement_service.go`**

```go
package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

type entitlementServiceImpl struct{ repo entitlementRepository }

// EntitlementService 模块内实例，由 module.Init 注入 DB 后装配。
var EntitlementService *entitlementServiceImpl

func newEntitlementService(repo entitlementRepository) *entitlementServiceImpl {
	return &entitlementServiceImpl{repo: repo}
}

// Grant 发放一批资源额度 + 写流水。expireAt=nil 表示永久。
// ledgerType ∈ {purchase, trial, adjust_grant}。ref=订单号/理由。
func (s *entitlementServiceImpl) Grant(userID int, subject string, quantity int64, expireAt *time.Time, source, ref, ledgerType, operator string) (*EntitlementBatch, error) {
	if !resourceSubjects[subject] {
		return nil, apperr.Validation("非法的资源科目")
	}
	if quantity <= 0 {
		return nil, apperr.Validation("发放数量必须大于0")
	}
	var batch EntitlementBatch
	err := s.repo.txWith(func(tx entitlementRepository) error {
		batch = EntitlementBatch{UserID: uint(userID), Subject: subject, Quantity: quantity, Source: source, SourceRef: ref, ExpireAt: expireAt}
		if err := tx.createBatch(&batch); err != nil {
			return err
		}
		cap, err := tx.capacity(userID, subject, time.Now())
		if err != nil {
			return err
		}
		return tx.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: subject, Type: ledgerType,
			Delta: quantity, BalanceAfter: cap, Reason: ref, Operator: operator,
		})
	})
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// Deduct 扣减资源额度（临近到期优先 FIFO）+ 写流水。容量不足报错。
// ledgerType ∈ {consume, adjust_deduct}。
func (s *entitlementServiceImpl) Deduct(userID int, subject string, amount int64, reason, ledgerType, operator string) error {
	if !resourceSubjects[subject] {
		return apperr.Validation("非法的资源科目")
	}
	if amount <= 0 {
		return apperr.Validation("扣减数量必须大于0")
	}
	return s.repo.txWith(func(tx entitlementRepository) error {
		now := time.Now()
		batches, err := tx.listActiveBatches(userID, subject, now)
		if err != nil {
			return err
		}
		remaining := amount
		for _, b := range batches {
			if remaining <= 0 {
				break
			}
			avail := b.Quantity - b.Used
			if avail <= 0 {
				continue
			}
			take := avail
			if take > remaining {
				take = remaining
			}
			if err := tx.addUsed(int(b.ID), take); err != nil {
				return err
			}
			remaining -= take
		}
		if remaining > 0 {
			return apperr.Validation("额度不足")
		}
		cap, err := tx.capacity(userID, subject, now)
		if err != nil {
			return err
		}
		return tx.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: subject, Type: ledgerType,
			Delta: -amount, BalanceAfter: cap, Reason: reason, Operator: operator,
		})
	})
}

// AdjustResource 运营手动赠送(delta>0)/扣减(delta<0)资源，理由必填。退款=赠送对应资源。
func (s *entitlementServiceImpl) AdjustResource(userID int, subject string, delta int64, reason, operator string) error {
	if delta == 0 {
		return apperr.Validation("调整数量不能为0")
	}
	if reason == "" {
		return apperr.Validation("调整理由必填")
	}
	if delta > 0 {
		_, err := s.Grant(userID, subject, delta, nil, SourceAdjust, reason, LedgerAdjustGrant, operator)
		return err
	}
	return s.Deduct(userID, subject, -delta, reason, LedgerAdjustDeduct, operator)
}

// Capacity 某科目可用容量。
func (s *entitlementServiceImpl) Capacity(userID int, subject string) (int64, error) {
	if !resourceSubjects[subject] {
		return 0, apperr.Validation("非法的资源科目")
	}
	return s.repo.capacity(userID, subject, time.Now())
}

// Capacities 三类资源容量概览。
func (s *entitlementServiceImpl) Capacities(userID int) (*CapacitySnapshot, error) {
	now := time.Now()
	inst, err := s.repo.capacity(userID, SubjectInstanceSeat, now)
	if err != nil {
		return nil, err
	}
	boot, err := s.repo.capacity(userID, SubjectBootSeat, now)
	if err != nil {
		return nil, err
	}
	mins, err := s.repo.capacity(userID, SubjectRuntimeMinute, now)
	if err != nil {
		return nil, err
	}
	return &CapacitySnapshot{InstanceSeat: inst, BootSeat: boot, RuntimeMinute: mins}, nil
}

// ListBatches 列某用户某科目（空=全部）的批次。
func (s *entitlementServiceImpl) ListBatches(userID int, subject string) ([]EntitlementBatch, error) {
	if subject != "" && !resourceSubjects[subject] {
		return nil, apperr.Validation("非法的资源科目")
	}
	return s.repo.listBatches(userID, subject)
}
```

- [ ] **Step 4：`module.go` Init 装配 EntitlementService**

```go
func (m *billingModule) Init(db *gorm.DB) error {
	BillingService = newService(newRepository(db))
	CatalogService = newCatalogService(newCatalogRepository(db))
	EntitlementService = newEntitlementService(newEntitlementRepository(db))
	return nil
}
```

- [ ] **Step 5：测试 → PASS（全部 billing）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v`

- [ ] **Step 6：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 权益 Grant/Deduct(FIFO)+资源调整+容量快照+统一流水

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4：前台资源概览 + 后台资源调整 + 路由

> handler 薄转发；build+vet 验证装配。

**Files:** Create `entitlement_api.go`; Modify `api.go`(AdminGetAccount 加 capacities), `module.go`(路由)。

- [ ] **Step 1：创建 `entitlement_api.go`**

```go
package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetMyEntitlements 前台：我的资源容量 + 批次明细
func GetMyEntitlements(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	caps, err := EntitlementService.Capacities(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	batches, err := EntitlementService.ListBatches(uid, "")
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"capacities": caps, "batches": batches})
}

// AdjustResourceRequest 后台资源调整请求
type AdjustResourceRequest struct {
	Subject string `json:"subject" binding:"required"`
	Delta   int64  `json:"delta" binding:"required"` // 正=赠送 负=扣减
	Reason  string `json:"reason" binding:"required"`
}

// AdminAdjustResource 运营：手动赠送/扣减资源包（理由必填）
func AdminAdjustResource(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	staffID, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req AdjustResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := EntitlementService.AdjustResource(uid, req.Subject, req.Delta, req.Reason, "staff:"+strconv.Itoa(staffID)); err != nil {
		framework.FailErr(c, err)
		return
	}
	caps, err := EntitlementService.Capacities(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, caps)
}
```

- [ ] **Step 2：`api.go` 的 `AdminGetAccount` handler 追加 capacities**

把 `AdminGetAccount` handler 中：
```go
	acc, ledger, total, err := BillingService.AdminGetAccount(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"account": acc, "ledger": ledger, "ledger_total": total})
```
改为（成功取到账户后再查容量）：
```go
	acc, ledger, total, err := BillingService.AdminGetAccount(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	caps, err := EntitlementService.Capacities(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"account": acc, "ledger": ledger, "ledger_total": total, "capacities": caps})
```

- [ ] **Step 3：`module.go` 加路由**

前台 `/billing` 组内追加：
```go
		g.GET("/entitlements", GetMyEntitlements)
```
后台 `/admin/billing` 组内追加：
```go
		admin.POST("/accounts/:userId/adjust-resource", staff.PermissionMiddleware("billing:manage"), AdminAdjustResource)
```

- [ ] **Step 4：全量回归 + 质量门（paste 输出）**

```bash
cd /home/root/workspace005/gloryphone-code/backend
gofmt -l modules/billing/
go vet ./...
go test ./... 2>&1 | tail -20
go test ./framework/ -run TestModuleBoundaries -v 2>&1 | tail -5
go build ./...
```
Expect 全 ok、边界 PASS、无 Gin 路由冲突。

- [ ] **Step 5：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 前台资源概览 + 后台资源包调整 + 账户容量 + 路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 3 验收对照（PRD §4.2 / §9.2）

| 需求 | 覆盖 |
|---|---|
| 三类资源包（实例席位/开机席位/时长分钟）以批次持有、带到期 | Task 2 |
| 可用容量 = 未过期批次 Σ(数量−已用) | Task 2 capacity |
| 统一流水覆盖余额与资源科目 | Task 1（字段泛化）+ Task 3（资源写流水） |
| 运营手动赠送/扣减资源包（带理由+审计）= 资源退款 | Task 3 AdjustResource + Task 4 后台端点 |
| 前台账户/资源概览（余额+三类容量） | Task 4 entitlements（余额见 Plan1 account） |
| FIFO 临近到期优先消耗（Phase2 计量复用） | Task 3 Deduct |

> 时长按分钟自动消耗（覆盖优先级 + 自动关机）= **Phase 2**，复用本计划 `Deduct`/容量；本计划不做自动消耗。实例创建消耗席位的门禁 = **计划 6**。

## Self-Review 记录

- **Spec 覆盖**：覆盖 PRD §4.2（账户+资源批次+统一流水）与 §9.2（手动调整扩展到资源包）；自动消耗/门禁明确归后续计划。
- **占位扫描**：无 TODO；每步完整代码与命令。Task 1 为字段重命名 prep，Step 4 用 grep 校验无残留。
- **类型一致**：`EntitlementBatch`/`CapacitySnapshot`、`Grant(userID,subject,quantity,expireAt,source,ref,ledgerType,operator)`、`Deduct(userID,subject,amount,reason,ledgerType,operator)`、`AdjustResource`、`Capacity`/`Capacities`/`ListBatches` 贯穿一致；`LedgerEntry.Delta/BalanceAfter` 改名后全模块统一（grep 校验）；资源科目常量与 `resourceSubjects` 一致。
- **事务/并发**：Grant/Deduct 经 `txWith` 单事务；Deduct FIFO 跨批次 + 容量不足回滚；sqlite 串行化，生产 mysql/pg 后续可加行锁（注释留痕，非本计划阻塞）。
- **金额/单位**：货币分、资源台/分钟；流水 `Delta` 按科目单位。
