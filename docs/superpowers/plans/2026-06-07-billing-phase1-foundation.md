# 计费系统 Phase 1 · 计划 1：账户 / 钱包余额 / 统一流水 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭起 `billing` 后端模块，落地「一用户一计费账户 + 钱包余额（分）+ 统一流水账本」，支持充值（桩）、运营手动赠送/扣减（带理由）、前台查账户与流水、后台查任意用户账户并调整余额。

**Architecture:** 沿用仓库「`framework` + `modules/<name>/internal`」模块化单体约定：模块在 `init()` 自注册并登记幂等建表，`Init(db)` 装配 `repository → service`，路由前台用 `user.AuthMiddleware()`、后台用 `staff.PermissionMiddleware(...)`。余额读改写用**单条带条件的原子 `UPDATE`**（`balance_cents + ? >= 0`）防丢失更新与扣成负数，并在同一事务内写一条流水快照。金额一律以**整数分（cents, int64）**存储，前端展示再换算为元。

**Tech Stack:** Go 1.24 / Gin / GORM（sqlite 测试，mysql/pg 生产）/ testify。模块名 `manager-backend`。

---

## Phase 1 计划路线图（本文件＝计划 1）

| # | 计划 | 产出（各自可测的工作软件） | 依赖 |
|---|---|---|---|
| **1** | **账户 / 钱包余额 / 统一流水**（本文件） | billing 模块、账户、余额原子增减、流水、充值桩、前台查询、后台余额调整、`billing` 权限组 | — |
| 2 | 商品目录与定价 | SKU + 规格 + 阶梯折扣 + 上下架；前台 `GET /billing/skus`、后台 CRUD | 1 |
| 3 | 权益批次 + 资源科目 | `instance_seat`/`boot_seat`/`runtime_minute` 批次、可用容量查询；流水扩展到资源科目；后台调整扩展到资源包 | 1 |
| 4 | 订单 / 收银台 / 支付桩 + 发放 | 下单、余额支付、网关回调桩、支付成功发放权益/入账（事务）；前台订单、后台订单 | 1,2,3 |
| 5 | 试用与发放 | 试用配置（4 维资格）、领取去重、发放入账；前后台页 | 2,3 |
| 6 | 实例创建门禁 + 欠费保护 cron | `phone.Create` 校验实例席位；席位池 active→grace→frozen→recycled（OnStart cron） | 3 |
| 7 | 前端接真（my + admin） | 收银台/订单/费用日志/账户概览 + 后台定价/折扣/订单/账户调整/试用配置 | 2–6 |

> 时长费计量（覆盖优先级、按分钟扣减、自动关机）= **Phase 2**，前置依赖中台开机时长接口（见 PRD §14.1），不在 Phase 1。

---

## 计划 1 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `backend/modules/billing/billing.go` | 公开包入口，供 main blank-import 触发自注册 | 创建 |
| `backend/modules/billing/internal/model.go` | `Account`、`LedgerEntry` 及科目/类型常量、请求 DTO | 创建 |
| `backend/modules/billing/internal/repository.go` | 持久化：get-or-create 账户、原子余额变更+写流水、流水查询 | 创建 |
| `backend/modules/billing/internal/service.go` | 业务：`GetAccount`/`Topup`/`AdjustBalance`/`ListLedger` | 创建 |
| `backend/modules/billing/internal/api.go` | 前台 + 后台 HTTP handler（薄转发） | 创建 |
| `backend/modules/billing/internal/module.go` | 自注册、建表、`Init`、路由 | 创建 |
| `backend/modules/billing/internal/main_test.go` | 测试库装配（`TestMain`） | 创建 |
| `backend/modules/billing/internal/service_test.go` | 服务层测试（账户/余额/流水/隔离/调整） | 创建 |
| `backend/main.go` | 增加 `_ "manager-backend/modules/billing"` 导入 | 修改 |
| `backend/modules/staff/internal/permissions.go` | 增加 `billing` 权限分组 | 修改 |

**约定速查（实现时照抄）**
- 统一响应：`framework.OK(c)` / `framework.OKWithData(c, data)` / `framework.OKWithPage(c, list, total)` / `framework.Fail(c, http.StatusXxx, msg)` / `framework.FailErr(c, err)`。
- 领域错误：`apperr.Validation(msg)` / `apperr.NotFound(msg)` / `apperr.BadRequest(msg)`（由 `FailErr` 翻译 HTTP 码：Validation→422、NotFound→404）。
- 当前用户：前台 `c.Get("userID")`→int（`user.AuthMiddleware` 写入）；后台同样 `c.Get("userID")`→int（staff id）。
- 测试库：`framework.SetupTestDB(m)` + `framework.DB.AutoMigrate(...)` + `framework.CleanTable("表名")`；默认 sqlite 临时文件。

---

## Task 1：模块骨架 + 账户模型 + get-or-create

**Files:**
- Create: `backend/modules/billing/billing.go`
- Create: `backend/modules/billing/internal/model.go`
- Create: `backend/modules/billing/internal/repository.go`
- Create: `backend/modules/billing/internal/service.go`
- Create: `backend/modules/billing/internal/module.go`
- Create: `backend/modules/billing/internal/main_test.go`
- Test: `backend/modules/billing/internal/service_test.go`
- Modify: `backend/main.go`

- [ ] **Step 1：写失败测试（账户自动创建且幂等）**

创建 `backend/modules/billing/internal/service_test.go`：

```go
package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = 7001
	userB = 7002
	staffOp = "staff:1"
)

func TestGetAccountAutoCreateIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	acc, err := BillingService.GetAccount(userA)
	require.NoError(t, err)
	assert.NotZero(t, acc.ID)
	assert.Equal(t, uint(userA), acc.UserID)
	assert.Equal(t, int64(0), acc.BalanceCents)

	// 再取一次：同一账户，不重复创建
	again, err := BillingService.GetAccount(userA)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, again.ID)
}
```

- [ ] **Step 2：创建 `main_test.go`（测试库装配）**

```go
package billing

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}); err != nil {
		panic(err)
	}
	if err := (&billingModule{}).Init(framework.DB); err != nil {
		panic(err)
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
```

- [ ] **Step 3：创建 `model.go`（账户 + 流水 + 常量）**

```go
package billing

import "time"

// Account 计费账户：一用户一份，持有钱包余额（分）。
type Account struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_billing_account_user" json:"user_id"`
	BalanceCents int64     `gorm:"not null;default:0" json:"balance_cents"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string { return "billing_accounts" }

// LedgerEntry 统一流水：余额与（后续计划的）资源包每一次增减的不可变记录。
// 既是「费用日志」的底座，也是「手动调整/退款」的载体。
type LedgerEntry struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint      `gorm:"not null;index:idx_billing_ledger_user" json:"user_id"`
	Subject           string    `gorm:"type:varchar(20);not null" json:"subject"`   // 科目：balance（计划1）；资源科目后续计划加
	Type              string    `gorm:"type:varchar(20);not null" json:"type"`      // topup/consume/adjust_grant/adjust_deduct/...
	DeltaCents        int64     `gorm:"not null" json:"delta_cents"`                // 正=增 负=减（科目=balance 时单位为分）
	BalanceAfterCents int64     `gorm:"not null" json:"balance_after_cents"`        // 变更后余额快照（审计/对账）
	Reason            string    `gorm:"type:varchar(255)" json:"reason"`            // 调整理由（adjust 必填）
	OrderID           uint      `gorm:"default:0;index" json:"order_id"`            // 关联订单（计划4）
	Operator          string    `gorm:"type:varchar(64)" json:"operator"`           // user:<id> / staff:<id> / system
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (LedgerEntry) TableName() string { return "billing_ledger_entries" }

// 科目
const SubjectBalance = "balance"

// 流水类型
const (
	LedgerTopup        = "topup"
	LedgerConsume      = "consume"
	LedgerAdjustGrant  = "adjust_grant"
	LedgerAdjustDeduct = "adjust_deduct"
)

// TopupRequest 充值请求（计划1 为桩：直接入账）。
type TopupRequest struct {
	AmountCents int64 `json:"amount_cents" binding:"required"`
}

// AdjustRequest 后台余额调整（正=赠送 负=扣减），理由必填。
type AdjustRequest struct {
	DeltaCents int64  `json:"delta_cents" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
}
```

- [ ] **Step 4：创建 `repository.go`（get-or-create）**

```go
package billing

import "gorm.io/gorm"

type repository interface {
	getOrCreateAccount(userID int) (*Account, error)
	applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error)
	listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error)
	countLedger(userID int, subject, typ string) (int64, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// getOrCreateAccount 取当前用户账户，不存在则建（余额 0）。
func (r *gormRepository) getOrCreateAccount(userID int) (*Account, error) {
	var acc Account
	err := r.db.Where(Account{UserID: uint(userID)}).
		Attrs(Account{BalanceCents: 0}).
		FirstOrCreate(&acc).Error
	if err != nil {
		return nil, err
	}
	return &acc, nil
}
```

> `applyBalance`、`listLedger`、`countLedger` 在 Task 2 / Task 3 补全；本 Task 先放接口签名即可编译（Go 允许接口方法暂未被调用，但实现体必须存在）。**因此本 Task 同时给出三者的最小占位实现**（见下），Task 2/3 再替换为真实逻辑。

`repository.go` 末尾追加占位实现（Task 2/3 替换）：

```go
func (r *gormRepository) applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error) {
	return nil, nil // TODO(Task 2)
}

func (r *gormRepository) listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error) {
	return nil, 0, nil // TODO(Task 3)
}

func (r *gormRepository) countLedger(userID int, subject, typ string) (int64, error) {
	return 0, nil // TODO(Task 3)
}
```

- [ ] **Step 5：创建 `service.go`（GetAccount）**

```go
package billing

// serviceImpl 计费服务，依赖注入 repository（不接触全局 DB）。
type serviceImpl struct{ repo repository }

// BillingService 模块内服务实例，由 module.Init 注入 DB 后装配。
var BillingService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// GetAccount 取（或自动创建）当前用户账户。
func (s *serviceImpl) GetAccount(userID int) (*Account, error) {
	return s.repo.getOrCreateAccount(userID)
}
```

- [ ] **Step 6：创建 `module.go`（注册 + 建表 + Init + 空路由）**

```go
package billing

import (
	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type billingModule struct{}

func (m *billingModule) Name() string { return "billing" }

func (m *billingModule) Init(db *gorm.DB) error {
	BillingService = newService(newRepository(db))
	return nil
}

func (m *billingModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台/后台路由在 Task 4 / Task 5 补全。
}

func (m *billingModule) OnStart() error { return nil }
func (m *billingModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&billingModule{})

	// 建表（幂等）：计费账户 + 统一流水。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Account{}, &LedgerEntry{})
	})
}
```

- [ ] **Step 7：创建 `billing.go`（公开包）**

```go
// Package billing 是计费业务模块的公开入口。
//
// 模块实现位于 modules/billing/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。如需对外发布契约（供其他模块依赖），在此再导出。
package billing

import _ "manager-backend/modules/billing/internal"
```

- [ ] **Step 8：在 `backend/main.go` 注册模块导入**

在 import 块中 `_ "manager-backend/modules/app"`（第 19 行）之后、`_ "manager-backend/modules/cloudphone"` 之前插入一行（保持字母序）：

```go
	_ "manager-backend/modules/billing"
```

- [ ] **Step 9：跑测试，确认通过**

Run: `cd backend && go test ./modules/billing/internal/ -run TestGetAccountAutoCreateIdempotent -v`
Expected: PASS（`ok manager-backend/modules/billing/internal`）。

- [ ] **Step 10：构建全量，确认 main 装配无误**

Run: `cd backend && go build ./...`
Expected: 无输出、退出码 0。

- [ ] **Step 11：提交**

```bash
git add backend/modules/billing backend/main.go
git commit -m "feat(billing): 模块骨架 + 计费账户 + get-or-create

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2：余额原子增减 + 流水（充值 / 调整）

**Files:**
- Modify: `backend/modules/billing/internal/repository.go`（`applyBalance` 真实实现）
- Modify: `backend/modules/billing/internal/service.go`（`Topup` / `AdjustBalance`）
- Test: `backend/modules/billing/internal/service_test.go`（追加）

- [ ] **Step 1：写失败测试（充值/扣减/余额不足/理由必填）**

在 `service_test.go` 追加：

```go
func TestTopupAndAdjustBalanceWithLedger(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	// 充值 100.00 元（10000 分）
	acc, err := BillingService.Topup(userA, 10000, "首次充值", "user:7001")
	require.NoError(t, err)
	assert.Equal(t, int64(10000), acc.BalanceCents)

	// 运营赠送 50.00 元
	acc, err = BillingService.AdjustBalance(userA, 5000, "活动补偿", staffOp)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), acc.BalanceCents)

	// 运营扣减 30.00 元（退款实现）
	acc, err = BillingService.AdjustBalance(userA, -3000, "误充退款", staffOp)
	require.NoError(t, err)
	assert.Equal(t, int64(12000), acc.BalanceCents)

	// 流水应有 3 条，且最新一条余额快照正确
	list, total, err := BillingService.repo.listLedger(userA, 0, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(12000), list[0].BalanceAfterCents) // id DESC，最新在前
	assert.Equal(t, LedgerAdjustDeduct, list[0].Type)
}

func TestAdjustBalanceGuards(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	// 扣成负数被拒
	_, err := BillingService.AdjustBalance(userA, -100, "越扣", staffOp)
	assert.Error(t, err)

	// 理由必填
	_, err = BillingService.AdjustBalance(userA, 100, "", staffOp)
	assert.Error(t, err)

	// 充值金额必须 > 0
	_, err = BillingService.Topup(userA, 0, "零充值", "user:7001")
	assert.Error(t, err)
}
```

- [ ] **Step 2：跑测试，确认失败**

Run: `cd backend && go test ./modules/billing/internal/ -run 'TestTopupAndAdjustBalanceWithLedger|TestAdjustBalanceGuards' -v`
Expected: FAIL（`applyBalance` 占位返回 nil → 解引用 panic 或断言不符）。

- [ ] **Step 3：实现 `applyBalance`（替换 Task 1 的占位）**

把 `repository.go` 里 `applyBalance` 占位体替换为：

```go
// applyBalance 原子地变更余额并写一条流水（同一事务）。
// 用「带条件的 UPDATE」防止丢失更新与扣成负数：余额不足时 RowsAffected=0 → 报错。
func (r *gormRepository) applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error) {
	var acc Account
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(Account{UserID: uint(userID)}).
			Attrs(Account{BalanceCents: 0}).
			FirstOrCreate(&acc).Error; err != nil {
			return err
		}
		res := tx.Model(&Account{}).
			Where("user_id = ? AND balance_cents + ? >= 0", userID, delta).
			UpdateColumn("balance_cents", gorm.Expr("balance_cents + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.Validation("余额不足")
		}
		if err := tx.Where("user_id = ?", userID).First(&acc).Error; err != nil {
			return err
		}
		entry := LedgerEntry{
			UserID: uint(userID), Subject: SubjectBalance, Type: typ,
			DeltaCents: delta, BalanceAfterCents: acc.BalanceCents,
			Reason: reason, OrderID: orderID, Operator: operator,
		}
		return tx.Create(&entry).Error
	})
	if err != nil {
		return nil, err
	}
	return &acc, nil
}
```

在 `repository.go` 顶部 import 加入 `"manager-backend/framework/apperr"`：

```go
import (
	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)
```

- [ ] **Step 4：实现 `Topup` / `AdjustBalance`（service.go 追加）**

```go
import "manager-backend/framework/apperr"

// Topup 充值入账（计划1 为桩：直接增加余额；计划4 改由支付网关回调驱动）。
func (s *serviceImpl) Topup(userID int, amountCents int64, reason, operator string) (*Account, error) {
	if amountCents <= 0 {
		return nil, apperr.Validation("充值金额必须大于0")
	}
	return s.repo.applyBalance(userID, amountCents, LedgerTopup, reason, 0, operator)
}

// AdjustBalance 运营手动赠送(正)/扣减(负)余额，理由必填；扣减不可越扣为负。退款=负向扣减。
func (s *serviceImpl) AdjustBalance(userID int, deltaCents int64, reason, operator string) (*Account, error) {
	if deltaCents == 0 {
		return nil, apperr.Validation("调整金额不能为0")
	}
	if reason == "" {
		return nil, apperr.Validation("调整理由必填")
	}
	typ := LedgerAdjustGrant
	if deltaCents < 0 {
		typ = LedgerAdjustDeduct
	}
	return s.repo.applyBalance(userID, deltaCents, typ, reason, 0, operator)
}
```

> 注意 `service.go` 顶部需有 `import "manager-backend/framework/apperr"`（与现有 import 合并为一个 import 块）。

- [ ] **Step 5：跑测试，确认通过**

Run: `cd backend && go test ./modules/billing/internal/ -run 'TestTopupAndAdjustBalanceWithLedger|TestAdjustBalanceGuards' -v`
Expected: PASS。

- [ ] **Step 6：提交**

```bash
git add backend/modules/billing/internal
git commit -m "feat(billing): 余额原子增减+流水，充值桩与手动赠送/扣减(带理由)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3：流水查询 + 跨用户隔离

**Files:**
- Modify: `backend/modules/billing/internal/repository.go`（`listLedger`/`countLedger` 真实实现）
- Modify: `backend/modules/billing/internal/service.go`（`ListLedger`）
- Test: `backend/modules/billing/internal/service_test.go`（追加）

- [ ] **Step 1：写失败测试（分页/筛选/隔离）**

```go
func TestListLedgerPagingFilterIsolation(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	_, _ = BillingService.Topup(userA, 10000, "充值", "user:7001")
	_, _ = BillingService.AdjustBalance(userA, -2000, "退款", staffOp)
	_, _ = BillingService.Topup(userB, 9999, "B充值", "user:7002")

	// userA 仅见自己的 2 条
	list, total, err := BillingService.ListLedger(userA, 1, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, list, 2)

	// 按类型筛选
	_, totalTopup, err := BillingService.ListLedger(userA, 1, 10, "", LedgerTopup)
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalTopup)

	// userB 看不到 userA 的流水
	_, totalB, err := BillingService.ListLedger(userB, 1, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalB)
}
```

- [ ] **Step 2：跑测试，确认失败**

Run: `cd backend && go test ./modules/billing/internal/ -run TestListLedgerPagingFilterIsolation -v`
Expected: FAIL（占位返回空 → total=0 不符）。

- [ ] **Step 3：实现 `listLedger` / `countLedger`（替换占位）**

```go
func (r *gormRepository) ledgerScope(userID int, subject, typ string) *gorm.DB {
	q := r.db.Model(&LedgerEntry{}).Where("user_id = ?", userID)
	if subject != "" {
		q = q.Where("subject = ?", subject)
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	return q
}

func (r *gormRepository) countLedger(userID int, subject, typ string) (int64, error) {
	var total int64
	err := r.ledgerScope(userID, subject, typ).Count(&total).Error
	return total, err
}

func (r *gormRepository) listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error) {
	total, err := r.countLedger(userID, subject, typ)
	if err != nil {
		return nil, 0, err
	}
	var items []LedgerEntry
	err = r.ledgerScope(userID, subject, typ).Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}
```

- [ ] **Step 4：实现 `ListLedger`（service.go 追加）**

```go
// ListLedger 费用日志：当前用户的余额/资源包流水（分页 + 科目/类型筛选）。
func (s *serviceImpl) ListLedger(userID, page, size int, subject, typ string) ([]LedgerEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	items, total, err := s.repo.listLedger(userID, (page-1)*size, size, subject, typ)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
```

- [ ] **Step 5：跑测试，确认通过**

Run: `cd backend && go test ./modules/billing/internal/ -run TestListLedgerPagingFilterIsolation -v`
Expected: PASS。

- [ ] **Step 6：提交**

```bash
git add backend/modules/billing/internal
git commit -m "feat(billing): 费用日志流水查询(分页/筛选/属主隔离)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4：前台 API（账户 / 流水 / 充值桩）+ 路由

> 说明：handler 为薄转发（与 `note`/`phone` 一致），业务逻辑已由 service 测试覆盖；本 Task 用 `go build` + 文档化 curl 验证装配，不额外写 handler 单测（遵循仓库现有约定）。

**Files:**
- Modify: `backend/modules/billing/internal/api.go`（创建）
- Modify: `backend/modules/billing/internal/module.go`（前台路由）

- [ ] **Step 1：创建 `api.go`（前台 handler）**

```go
package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录用户 ID（前台 user / 后台 staff 中间件均写入 "userID"）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// GetMyAccount 我的计费账户（余额等）
func GetMyAccount(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	acc, err := BillingService.GetAccount(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, acc)
}

// GetMyLedger 我的费用日志（流水，分页 + subject/type 筛选）
func GetMyLedger(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := BillingService.ListLedger(uid, page, size, c.Query("subject"), c.Query("type"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// Topup 充值（计划1 为桩：直接入账；计划4 改由支付网关回调驱动）
func Topup(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	acc, err := BillingService.Topup(uid, req.AmountCents, "充值", "user:"+strconv.Itoa(uid))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, acc)
}
```

- [ ] **Step 2：在 `module.go` 注册前台路由**

把 `RegisterRoutes` 替换为（前台段；后台段 Task 5 加）：

```go
func (m *billingModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：我的计费，按属主隔离，要求 user 登录。
	g := router.Group("/billing")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/account", GetMyAccount)
		g.GET("/ledger", GetMyLedger)
		g.POST("/topup", Topup)
	}
}
```

在 `module.go` import 加入 `"manager-backend/modules/user"`：

```go
import (
	"manager-backend/framework"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
```

- [ ] **Step 3：构建 + 跑全部 billing 测试，确认无回归**

Run: `cd backend && go build ./... && go test ./modules/billing/internal/ -v`
Expected: build 无输出；测试全 PASS。

- [ ] **Step 4：（可选）手动联调验证**

启动后端后（需 user 登录 Cookie/Bearer）：
```
GET  /api/v1/billing/account            → {code:0,data:{balance_cents:0,...}}
POST /api/v1/billing/topup {"amount_cents":10000} → 余额 10000
GET  /api/v1/billing/ledger             → {code:0,data:{list:[...],total:1}}
```

- [ ] **Step 5：提交**

```bash
git add backend/modules/billing/internal
git commit -m "feat(billing): 前台 API 账户/费用日志/充值(桩) + 路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5：后台 API（查账户 / 余额调整）+ `billing` 权限组

**Files:**
- Modify: `backend/modules/staff/internal/permissions.go`（新增 billing 分组）
- Modify: `backend/modules/billing/internal/api.go`（后台 handler）
- Modify: `backend/modules/billing/internal/service.go`（`AdminGetAccount` 组合账户+流水，可选）
- Modify: `backend/modules/billing/internal/module.go`（后台路由）
- Test: `backend/modules/billing/internal/service_test.go`（追加 1 条调整用例已在 Task 2 覆盖；此处加管理读取用例）

- [ ] **Step 1：在 `permissions.go` 增加 billing 权限分组**

在 `// scaffold:permission-groups` 标记行之前插入：

```go
	{
		Module: "计费管理", ModuleKey: "billing",
		Permissions: []Permission{
			{Key: "billing:view", Label: "查看计费（账户/订单/流水）"},
			{Key: "billing:manage", Label: "管理计费（调整余额/资源、配置定价与试用）"},
		},
	},
```

- [ ] **Step 2：写失败测试（管理读取任意用户账户+流水）**

在 `service_test.go` 追加：

```go
func TestAdminGetAccountView(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	_, _ = BillingService.Topup(userA, 8888, "充值", "user:7001")

	acc, ledger, err := BillingService.AdminGetAccount(userA, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(8888), acc.BalanceCents)
	require.Len(t, ledger, 1)
	assert.Equal(t, LedgerTopup, ledger[0].Type)
}
```

- [ ] **Step 3：跑测试，确认失败**

Run: `cd backend && go test ./modules/billing/internal/ -run TestAdminGetAccountView -v`
Expected: FAIL（`AdminGetAccount` 未定义 → 编译错误）。

- [ ] **Step 4：实现 `AdminGetAccount`（service.go 追加）**

```go
// AdminGetAccount 运营查看任意用户的账户 + 近期流水（属主由调用方按权限控制）。
func (s *serviceImpl) AdminGetAccount(userID, page, size int) (*Account, []LedgerEntry, error) {
	acc, err := s.repo.getOrCreateAccount(userID)
	if err != nil {
		return nil, nil, err
	}
	ledger, _, err := s.ListLedger(userID, page, size, "", "")
	if err != nil {
		return nil, nil, err
	}
	return acc, ledger, nil
}
```

- [ ] **Step 5：在 `api.go` 追加后台 handler**

```go
// AdminGetAccount 运营：查看某用户账户 + 流水
func AdminGetAccount(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	acc, ledger, err := BillingService.AdminGetAccount(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"account": acc, "ledger": ledger})
}

// AdminAdjustBalance 运营：手动赠送/扣减余额（理由必填）= 退款实现
func AdminAdjustBalance(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	staffID, _ := currentUserID(c)
	var req AdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	acc, err := BillingService.AdjustBalance(uid, req.DeltaCents, req.Reason, "staff:"+strconv.Itoa(staffID))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, acc)
}
```

- [ ] **Step 6：在 `module.go` 注册后台路由**

在 `RegisterRoutes` 的前台段之后追加：

```go
	// 后台：运营查看账户 + 调整余额（staff 登录 + 权限）。
	admin := router.Group("/admin/billing")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/accounts/:userId", staff.PermissionMiddleware("billing:view"), AdminGetAccount)
		admin.POST("/accounts/:userId/adjust", staff.PermissionMiddleware("billing:manage"), AdminAdjustBalance)
	}
```

在 `module.go` import 加入 `"manager-backend/modules/staff"`：

```go
import (
	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)
```

- [ ] **Step 7：跑测试 + 构建**

Run: `cd backend && go build ./... && go test ./modules/billing/internal/ -v`
Expected: build 无输出；billing 测试全 PASS。

- [ ] **Step 8：跑全仓测试，确认无回归（尤其 staff 权限目录）**

Run: `cd backend && go test ./... 2>&1 | tail -30`
Expected: 各包 `ok`（无 FAIL）；`staff` 包测试通过（权限组数量类断言若存在需同步——见下方排查）。

> 排查提示：若 `staff` 包有「权限总数/分组数」断言因新增 billing 组而失败，更新该断言的期望值即可（搜索 `PermissionGroups` 相关断言）。

- [ ] **Step 9：提交**

```bash
git add backend/modules/billing/internal backend/modules/staff/internal/permissions.go
git commit -m "feat(billing): 后台账户查看+余额调整 + billing 权限组

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 1 验收对照（PRD §16）

| AC | 覆盖任务 |
|---|---|
| AC-5（余额支付的扣减原语：充足扣、不足拒） | Task 2（`applyBalance` 守卫，余额支付编排在计划4） |
| AC-8（手动赠送/扣减需理由、写流水、不越扣为负） | Task 2 `AdjustBalance` + `TestAdjustBalanceGuards` |
| AC-9（费用日志：流水可见、可筛选、属主隔离） | Task 3 `ListLedger` + Task 4 前台 API |
| 退款（= 负向调整 + 理由） | Task 2 / Task 5（后台入口） |

> AC-1/2/3/4/6/7/10/11 由后续计划（2–7、Phase 2）覆盖；本计划是其账户/流水底座。

---

## Self-Review 记录

- **Spec 覆盖**：计划 1 对应 PRD §4.2（账户+流水）、§9（钱包/充值/手动调整）、§16 AC-5/8/9 与退款；其余 AC 明确指派给后续计划（路线图表）。无遗漏到「无计划承接」的本计划范围内需求。
- **占位扫描**：Task 1 Step 4 故意放 `applyBalance`/`listLedger`/`countLedger` 占位以保证可编译，并在 Task 2/3 明确替换——非计划占位，已标注替换位置。其余步骤均给出完整代码与命令。
- **类型一致**：`Account`/`LedgerEntry` 字段、`applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string)`、`ListLedger(userID,page,size int, subject,typ string)`、`AdminGetAccount(userID,page,size int)` 在各 Task 引用一致；常量 `SubjectBalance`/`LedgerTopup`/`LedgerAdjustGrant`/`LedgerAdjustDeduct` 在 model.go 统一定义。
- **金额单位**：全程整数分（`*Cents int64`），前端换算元，避免浮点误差。
