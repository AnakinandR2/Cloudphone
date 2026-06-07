# 计费系统 Phase 1 · 计划 5：试用与发放 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 运营按策略配置免费试用（发放某资源权益），用户在满足资格时领取；4 维资格：新用户(从未付费) / 运营手动授予 / 每用户限领次数 / 活动·邀请码。领取去重 + 原子发放（复用权益 Grant，流水类型 `trial`）。

**Architecture:** 复用 `billing/internal` 单包（`trial_*.go` 前缀分子域）。`TrialPolicy`（发放科目/数量/有效天数 + 资格规则）、`TrialGrant`（领取记录，去重/历史）、`TrialEligibility`（运营手动授予的领取资格）。**领取 claim** 在 `trialRepo` 单事务内：再次校验未超限领（守卫）→ 写 `TrialGrant` → 发放权益（tx 绑定 `gormEntitlementRepository`：createBatch+capacity+insertLedger(type=trial)），与订单 settle 同范式。「新用户」= billing 本地「该用户已付订单数==0」（自包含，无需跨模块）。试用**仅发放资源**（instance_seat/boot_seat/runtime_minute）；余额赠送由 [[Plan1]] 后台 AdjustBalance 覆盖。金额/单位沿用既有。

**Tech Stack:** Go 1.26 / Gin / GORM（sqlite 测试）/ testify。模块名 `manager-backend`，分支 `feature/billing-trial`。

---

## 领域模型

- **TrialPolicy** `billing_trial_policies`：`{code(uniqueIndex), name, enabled, grant_subject(资源科目), grant_quantity, grant_expire_days(0=永久), per_user_limit(默认1), allow_new_user, invite_code(""=不需要), 时间戳}`。
- **TrialGrant** `billing_trial_grants`：`{policy_id, user_id, subject, quantity, claimed_at, created_at}`（去重：`count(policy_id,user_id) < per_user_limit`）。
- **TrialEligibility** `billing_trial_eligibilities`：`{policy_id, user_id, granted_by, created_at}`（运营手动授予领取资格）。
- 新增流水类型 `LedgerTrial = "trial"`。

## 资格判定

领取资格（满足任一路径，且未超限领）：
1. **新用户**：`policy.AllowNewUser` 且 该用户已付订单数==0。
2. **手动授予**：`TrialEligibility` 存在 `(policy, user)`。
3. **活动/邀请码**：`policy.InviteCode != ""` 且领取时提供的 `inviteCode == policy.InviteCode`。
限领：`countClaims(policy, user) < policy.PerUserLimit`。

## 计划 5 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `internal/trial_model.go` | `TrialPolicy/TrialGrant/TrialEligibility`、DTO | 创建 |
| `internal/trial_repository.go` | `trialRepository`：策略 CRUD、countClaims、manualEligible、countPaidOrders、`claim`(事务) | 创建 |
| `internal/trial_service.go` | `TrialService`：ListClaimable/ClaimTrial/策略 CRUD/GrantEligibility/ListGrants | 创建 |
| `internal/trial_api.go` | 前台 可领/领取；后台 策略 CRUD/授予资格/发放记录 | 创建 |
| `internal/trial_service_test.go` | 服务层测试 | 创建 |
| `internal/model.go` | 加 `LedgerTrial` 常量 | 修改 |
| `internal/module.go` | Init 装配 `TrialService`；建表加 3 表；路由 | 修改 |
| `internal/main_test.go` | AutoMigrate 加 3 表 | 修改 |

**约定速查**：`framework.OK*/Fail/FailErr`；`apperr.Validation/NotFound/Conflict`；前台 `user.AuthMiddleware()`(`currentUserID`)；后台 `staff.PermissionMiddleware("billing:view"|"billing:manage")`；Go 可用 `time.Now()`；测试 `framework.CleanTable`。已有：`gormEntitlementRepository{db}`(`createBatch`/`capacity(uid,subject,now)`/`insertLedger`)；资源科目常量 `SubjectInstanceSeat/BootSeat/RuntimeMinute` + `resourceSubjects` map；`SourceTrial`；订单表 `billing_orders`(status 'paid')。

---

## Task 1：试用模型 + 表 + 仓储 + LedgerTrial

**Files:** Create `trial_model.go`, `trial_repository.go`; Modify `model.go`, `module.go`, `main_test.go`; Test `trial_service_test.go`（仓储层）。

- [ ] **Step 1：创建 `trial_model.go`**

```go
package billing

import "time"

// TrialPolicy 试用策略（运营配置）：满足资格的用户可领取一份资源权益。
type TrialPolicy struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_billing_trial_code" json:"code"`
	Name            string    `gorm:"type:varchar(100);not null" json:"name"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	GrantSubject    string    `gorm:"type:varchar(20);not null" json:"grant_subject"`  // 资源科目
	GrantQuantity   int64     `gorm:"not null" json:"grant_quantity"`                  // 台/分钟
	GrantExpireDays int       `gorm:"not null;default:0" json:"grant_expire_days"`     // 0=永久
	PerUserLimit    int       `gorm:"not null;default:1" json:"per_user_limit"`
	AllowNewUser    bool      `gorm:"not null;default:false" json:"allow_new_user"`
	InviteCode      string    `gorm:"type:varchar(64)" json:"invite_code"` // ""=不需要
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TrialPolicy) TableName() string { return "billing_trial_policies" }

// TrialGrant 领取记录（去重 + 历史）。
type TrialGrant struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID  uint      `gorm:"not null;index:idx_billing_trialgrant_policy_user" json:"policy_id"`
	UserID    uint      `gorm:"not null;index:idx_billing_trialgrant_policy_user" json:"user_id"`
	Subject   string    `gorm:"type:varchar(20);not null" json:"subject"`
	Quantity  int64     `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TrialGrant) TableName() string { return "billing_trial_grants" }

// TrialEligibility 运营手动授予的领取资格。
type TrialEligibility struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID  uint      `gorm:"not null;index:idx_billing_trialelig_policy_user" json:"policy_id"`
	UserID    uint      `gorm:"not null;index:idx_billing_trialelig_policy_user" json:"user_id"`
	GrantedBy string    `gorm:"type:varchar(64)" json:"granted_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TrialEligibility) TableName() string { return "billing_trial_eligibilities" }

// DTO
type TrialPolicyCreate struct {
	Code            string `json:"code" binding:"required"`
	Name            string `json:"name" binding:"required"`
	GrantSubject    string `json:"grant_subject" binding:"required"`
	GrantQuantity   int64  `json:"grant_quantity" binding:"required"`
	GrantExpireDays int    `json:"grant_expire_days"`
	PerUserLimit    int    `json:"per_user_limit"`
	AllowNewUser    bool   `json:"allow_new_user"`
	InviteCode      string `json:"invite_code"`
	Enabled         *bool  `json:"enabled"`
}

type TrialPolicyUpdate struct {
	Name            string `json:"name"`
	GrantQuantity   *int64 `json:"grant_quantity"`
	GrantExpireDays *int   `json:"grant_expire_days"`
	PerUserLimit    *int   `json:"per_user_limit"`
	AllowNewUser    *bool  `json:"allow_new_user"`
	InviteCode      *string `json:"invite_code"`
	Enabled         *bool  `json:"enabled"`
}

type ClaimRequest struct {
	InviteCode string `json:"invite_code"`
}

type EligibilityGrantRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

// ClaimableItem 前台「可领」列表项。
type ClaimableItem struct {
	Policy        TrialPolicy `json:"policy"`
	Claimable     bool        `json:"claimable"`
	NeedInvite    bool        `json:"need_invite"`
	ClaimedCount  int         `json:"claimed_count"`
	Reason        string      `json:"reason"`
}
```

- [ ] **Step 2：`model.go` 加 `LedgerTrial` 常量**

在流水类型常量块追加：
```go
	LedgerTrial = "trial" // 试用发放
```

- [ ] **Step 3：`main_test.go` + `module.go` AutoMigrate 加 3 表**

两处 AutoMigrate 末尾追加 `&TrialPolicy{}, &TrialGrant{}, &TrialEligibility{}`。

- [ ] **Step 4：创建 `trial_repository.go`（claim 占位，Task 2 实现）**

```go
package billing

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type trialRepository interface {
	createPolicy(p *TrialPolicy) error
	updatePolicy(id int, fields map[string]interface{}) error
	deletePolicy(id int) error
	getPolicy(id int) (*TrialPolicy, error)
	getPolicyByCode(code string) (*TrialPolicy, error)
	listPolicies(enabledOnly bool) ([]TrialPolicy, error)

	countClaims(policyID, userID int) (int, error)
	manualEligible(policyID, userID int) (bool, error)
	countPaidOrders(userID int) (int, error)
	createEligibility(e *TrialEligibility) error
	listGrants(policyID int) ([]TrialGrant, error)

	// claim 单事务：守卫式限领 + 写 TrialGrant + 发放权益。Task 2 实现。
	claim(policy *TrialPolicy, userID int, expireAt *time.Time) error
}

type gormTrialRepository struct{ db *gorm.DB }

func newTrialRepository(db *gorm.DB) trialRepository { return &gormTrialRepository{db: db} }

func (r *gormTrialRepository) createPolicy(p *TrialPolicy) error { return r.db.Create(p).Error }

func (r *gormTrialRepository) updatePolicy(id int, fields map[string]interface{}) error {
	return r.db.Model(&TrialPolicy{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormTrialRepository) deletePolicy(id int) error {
	return r.db.Where("id = ?", id).Delete(&TrialPolicy{}).Error
}

func (r *gormTrialRepository) getPolicy(id int) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) getPolicyByCode(code string) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Where("code = ?", code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) listPolicies(enabledOnly bool) ([]TrialPolicy, error) {
	q := r.db.Model(&TrialPolicy{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	var items []TrialPolicy
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormTrialRepository) countClaims(policyID, userID int) (int, error) {
	var n int64
	err := r.db.Model(&TrialGrant{}).Where("policy_id = ? AND user_id = ?", policyID, userID).Count(&n).Error
	return int(n), err
}

func (r *gormTrialRepository) manualEligible(policyID, userID int) (bool, error) {
	var n int64
	err := r.db.Model(&TrialEligibility{}).Where("policy_id = ? AND user_id = ?", policyID, userID).Count(&n).Error
	return n > 0, err
}

func (r *gormTrialRepository) countPaidOrders(userID int) (int, error) {
	var n int64
	err := r.db.Model(&Order{}).Where("user_id = ? AND status = ?", userID, OrderPaid).Count(&n).Error
	return int(n), err
}

func (r *gormTrialRepository) createEligibility(e *TrialEligibility) error { return r.db.Create(e).Error }

func (r *gormTrialRepository) listGrants(policyID int) ([]TrialGrant, error) {
	var items []TrialGrant
	err := r.db.Where("policy_id = ?", policyID).Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormTrialRepository) claim(policy *TrialPolicy, userID int, expireAt *time.Time) error {
	return errors.New("not implemented") // TODO(Task 2)
}

func isNotFoundTrial(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
```

- [ ] **Step 5：创建 `trial_service_test.go`（仓储 helper 测试）**

```go
package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trialUser = 9201

func TestTrialPolicyCRUDAndHelpers(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_trial_policies", "billing_trial_grants", "billing_trial_eligibilities", "billing_orders")
	})
	repo := newTrialRepository(framework.DB)

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, GrantSubject: SubjectInstanceSeat, GrantQuantity: 1, PerUserLimit: 1, AllowNewUser: true}))
	p, err := repo.getPolicyByCode("newbie")
	require.NoError(t, err)
	assert.Equal(t, "新人礼", p.Name)

	// 限领计数 0
	n, err := repo.countClaims(int(p.ID), trialUser)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	// 手动资格：默认无
	ok, err := repo.manualEligible(int(p.ID), trialUser)
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, repo.createEligibility(&TrialEligibility{PolicyID: p.ID, UserID: uint(trialUser), GrantedBy: "staff:1"}))
	ok, _ = repo.manualEligible(int(p.ID), trialUser)
	assert.True(t, ok)

	// 已付订单数：0（新用户）
	cnt, err := repo.countPaidOrders(trialUser)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)
	require.NoError(t, framework.DB.Create(&Order{OrderNo: "BILX1", UserID: uint(trialUser), Status: OrderPaid, PayMethod: PayBalance, TotalCents: 1}).Error)
	cnt, _ = repo.countPaidOrders(trialUser)
	assert.Equal(t, 1, cnt) // 已付费 → 非新用户

	list, err := repo.listPolicies(true)
	require.NoError(t, err)
	require.Len(t, list, 1)
}
```

- [ ] **Step 6：测试 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run TestTrialPolicyCRUDAndHelpers -v && go test ./modules/billing/internal/ -v && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 试用模型(策略/领取/资格)+仓储+LedgerTrial

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> 注：`claim` 占位返回 not implemented，Task 2 实现。`TrialService` 尚未装配（Task 2 在 Init 加），本 Task 测试直接用 `newTrialRepository`。

---

## Task 2：TrialService 领取（资格 + 原子发放 + 去重）+ ListClaimable

**Files:** Modify `trial_repository.go`(`claim` 真实现), `module.go`(Init 装配 TrialService); Create `trial_service.go`; Test 追加。

- [ ] **Step 1：写失败测试（追加 `trial_service_test.go`）**

```go
func TestClaimTrialNewUserAndDedup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_trial_policies", "billing_trial_grants", "billing_trial_eligibilities",
			"billing_orders", "billing_entitlement_batches", "billing_ledger_entries")
	})
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, GrantSubject: SubjectInstanceSeat, GrantQuantity: 1, GrantExpireDays: 30, PerUserLimit: 1, AllowNewUser: true}))

	// 新用户领取成功，发放 1 台实例席位
	require.NoError(t, TrialService.ClaimTrial(trialUser, "newbie", ""))
	cap, err := EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cap)

	// 再领 → 超限
	err = TrialService.ClaimTrial(trialUser, "newbie", "")
	assert.Error(t, err)
	cap, _ = EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	assert.Equal(t, int64(1), cap) // 未二次发放

	// 流水含一条 trial
	_, total, err := BillingService.ListLedger(trialUser, 1, 50, SubjectInstanceSeat, LedgerTrial)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestClaimTrialEligibilityPaths(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_trial_policies", "billing_trial_grants", "billing_trial_eligibilities",
			"billing_orders", "billing_entitlement_batches", "billing_ledger_entries")
	})
	repo := newTrialRepository(framework.DB)

	// 仅邀请码策略
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, GrantSubject: SubjectRuntimeMinute, GrantQuantity: 600, PerUserLimit: 1, InviteCode: "VIP2026"}))
	// 不给码 → 失败
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", ""))
	// 错码 → 失败
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", "WRONG"))
	// 对码 → 成功
	require.NoError(t, TrialService.ClaimTrial(trialUser, "promo", "VIP2026"))
	cap, _ := EntitlementService.Capacity(trialUser, SubjectRuntimeMinute)
	assert.Equal(t, int64(600), cap)

	// 已付费用户 + 仅新用户策略 → 失败；手动授予后 → 成功
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newonly", Name: "仅新人", Enabled: true, GrantSubject: SubjectBootSeat, GrantQuantity: 2, PerUserLimit: 1, AllowNewUser: true}))
	require.NoError(t, framework.DB.Create(&Order{OrderNo: "BILY1", UserID: uint(trialUser), Status: OrderPaid, PayMethod: PayBalance, TotalCents: 1}).Error)
	assert.Error(t, TrialService.ClaimTrial(trialUser, "newonly", "")) // 已付费非新用户
	pol, _ := repo.getPolicyByCode("newonly")
	require.NoError(t, repo.createEligibility(&TrialEligibility{PolicyID: pol.ID, UserID: uint(trialUser), GrantedBy: "staff:1"}))
	require.NoError(t, TrialService.ClaimTrial(trialUser, "newonly", "")) // 手动授予 → 成功
	cap, _ = EntitlementService.Capacity(trialUser, SubjectBootSeat)
	assert.Equal(t, int64(2), cap)
}

func TestListClaimable(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_trial_policies", "billing_trial_grants", "billing_trial_eligibilities",
			"billing_orders", "billing_entitlement_batches", "billing_ledger_entries")
	})
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, GrantSubject: SubjectInstanceSeat, GrantQuantity: 1, PerUserLimit: 1, AllowNewUser: true}))
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, GrantSubject: SubjectRuntimeMinute, GrantQuantity: 600, PerUserLimit: 1, InviteCode: "VIP"}))

	items, err := TrialService.ListClaimable(trialUser)
	require.NoError(t, err)
	require.Len(t, items, 2)
	byCode := map[string]ClaimableItem{}
	for _, it := range items {
		byCode[it.Policy.Code] = it
	}
	assert.True(t, byCode["newbie"].Claimable)   // 新用户可领
	assert.False(t, byCode["promo"].Claimable)   // 需邀请码
	assert.True(t, byCode["promo"].NeedInvite)
}
```

- [ ] **Step 2：测试 → FAIL（TrialService 未定义）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestClaimTrial|TestListClaimable' -v`

- [ ] **Step 3：实现 `claim`（替换 `trial_repository.go` 占位）**

```go
// claim 单事务：守卫式限领 + 写 TrialGrant + 发放权益(type=trial)。
func (r *gormTrialRepository) claim(policy *TrialPolicy, userID int, expireAt *time.Time) error {
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 守卫：事务内再次校验未超限领
		var claimed int64
		if err := tx.Model(&TrialGrant{}).Where("policy_id = ? AND user_id = ?", policy.ID, userID).Count(&claimed).Error; err != nil {
			return err
		}
		if int(claimed) >= policy.PerUserLimit {
			return apperr.Conflict("已达领取上限")
		}
		if err := tx.Create(&TrialGrant{PolicyID: policy.ID, UserID: uint(userID), Subject: policy.GrantSubject, Quantity: policy.GrantQuantity}).Error; err != nil {
			return err
		}
		entRepo := &gormEntitlementRepository{db: tx}
		batch := EntitlementBatch{UserID: uint(userID), Subject: policy.GrantSubject, Quantity: policy.GrantQuantity, Source: SourceTrial, SourceRef: "trial:" + policy.Code, ExpireAt: expireAt}
		if err := entRepo.createBatch(&batch); err != nil {
			return err
		}
		capacity, err := entRepo.capacity(userID, policy.GrantSubject, now)
		if err != nil {
			return err
		}
		return entRepo.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: policy.GrantSubject, Type: LedgerTrial,
			Delta: policy.GrantQuantity, BalanceAfter: capacity, Reason: "trial:" + policy.Code, Operator: "user:" + itoa(userID),
		})
	})
}
```
（`apperr`、`itoa` 已在包内可用；`trial_repository.go` import 需含 `manager-backend/framework/apperr`。）

- [ ] **Step 4：创建 `trial_service.go`**

```go
package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

type trialServiceImpl struct{ repo trialRepository }

var TrialService *trialServiceImpl

func newTrialService(repo trialRepository) *trialServiceImpl { return &trialServiceImpl{repo: repo} }

// eligible 计算资格（不含邀请码主动校验）：返回 (可领, 需邀请码, 已领次数, 原因)。
func (s *trialServiceImpl) eligible(userID int, p *TrialPolicy) (bool, bool, int, string) {
	claimed, _ := s.repo.countClaims(int(p.ID), userID)
	if claimed >= p.PerUserLimit {
		return false, false, claimed, "已达领取上限"
	}
	if p.AllowNewUser {
		if paid, _ := s.repo.countPaidOrders(userID); paid == 0 {
			return true, false, claimed, ""
		}
	}
	if ok, _ := s.repo.manualEligible(int(p.ID), userID); ok {
		return true, false, claimed, ""
	}
	if p.InviteCode != "" {
		return false, true, claimed, "需邀请码"
	}
	return false, false, claimed, "不符合领取条件"
}

// ListClaimable 列出启用策略 + 当前用户可领状态。
func (s *trialServiceImpl) ListClaimable(userID int) ([]ClaimableItem, error) {
	policies, err := s.repo.listPolicies(true)
	if err != nil {
		return nil, err
	}
	out := make([]ClaimableItem, 0, len(policies))
	for _, p := range policies {
		ok, needInvite, claimed, reason := s.eligible(userID, &p)
		out = append(out, ClaimableItem{Policy: p, Claimable: ok, NeedInvite: needInvite, ClaimedCount: claimed, Reason: reason})
	}
	return out, nil
}

// ClaimTrial 领取试用：校验资格(含邀请码) → 原子发放。
func (s *trialServiceImpl) ClaimTrial(userID int, code, inviteCode string) error {
	p, err := s.repo.getPolicyByCode(code)
	if err != nil {
		if isNotFoundTrial(err) {
			return apperr.NotFound("试用不存在")
		}
		return err
	}
	if !p.Enabled {
		return apperr.Validation("试用已停用")
	}
	claimed, err := s.repo.countClaims(int(p.ID), userID)
	if err != nil {
		return err
	}
	if claimed >= p.PerUserLimit {
		return apperr.Conflict("已达领取上限")
	}
	// 资格：新用户 / 手动授予 / 邀请码 任一
	ok := false
	if p.AllowNewUser {
		if paid, _ := s.repo.countPaidOrders(userID); paid == 0 {
			ok = true
		}
	}
	if !ok {
		if m, _ := s.repo.manualEligible(int(p.ID), userID); m {
			ok = true
		}
	}
	if !ok && p.InviteCode != "" && inviteCode == p.InviteCode {
		ok = true
	}
	if !ok {
		return apperr.Validation("不符合领取条件")
	}
	var expireAt *time.Time
	if p.GrantExpireDays > 0 {
		exp := time.Now().AddDate(0, 0, p.GrantExpireDays)
		expireAt = &exp
	}
	return s.repo.claim(p, userID, expireAt)
}
```

- [ ] **Step 5：`module.go` Init 装配 TrialService**

```go
	OrderService = newOrderService(newOrderRepository(db), CatalogService)
	TrialService = newTrialService(newTrialRepository(db))
```

- [ ] **Step 6：测试 → PASS（全部 billing）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v`

- [ ] **Step 7：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 试用领取(4维资格+原子发放+去重)+ListClaimable

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3：后台试用配置/发放记录 服务 + 前台/后台 API + 路由

**Files:** Modify `trial_service.go`(admin 方法); Create `trial_api.go`; Modify `module.go`(路由)。

- [ ] **Step 1：在 `trial_service.go` 追加 admin 方法**

```go
var trialGrantSubjects = resourceSubjects // 试用仅发资源

// CreatePolicy 运营新建试用策略。
func (s *trialServiceImpl) CreatePolicy(req *TrialPolicyCreate) (*TrialPolicy, error) {
	if !trialGrantSubjects[req.GrantSubject] {
		return nil, apperr.Validation("试用只能发放资源科目")
	}
	if req.GrantQuantity <= 0 {
		return nil, apperr.Validation("发放数量必须大于0")
	}
	if _, err := s.repo.getPolicyByCode(req.Code); err == nil {
		return nil, apperr.Conflict("策略编码已存在")
	} else if !isNotFoundTrial(err) {
		return nil, err
	}
	limit := req.PerUserLimit
	if limit < 1 {
		limit = 1
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := TrialPolicy{
		Code: req.Code, Name: req.Name, Enabled: enabled, GrantSubject: req.GrantSubject,
		GrantQuantity: req.GrantQuantity, GrantExpireDays: req.GrantExpireDays, PerUserLimit: limit,
		AllowNewUser: req.AllowNewUser, InviteCode: req.InviteCode,
	}
	if err := s.repo.createPolicy(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdatePolicy 更新策略。
func (s *trialServiceImpl) UpdatePolicy(id int, req *TrialPolicyUpdate) (*TrialPolicy, error) {
	if _, err := s.getPolicy(id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.GrantQuantity != nil {
		if *req.GrantQuantity <= 0 {
			return nil, apperr.Validation("发放数量必须大于0")
		}
		fields["grant_quantity"] = *req.GrantQuantity
	}
	if req.GrantExpireDays != nil {
		fields["grant_expire_days"] = *req.GrantExpireDays
	}
	if req.PerUserLimit != nil {
		if *req.PerUserLimit < 1 {
			return nil, apperr.Validation("限领次数必须≥1")
		}
		fields["per_user_limit"] = *req.PerUserLimit
	}
	if req.AllowNewUser != nil {
		fields["allow_new_user"] = *req.AllowNewUser
	}
	if req.InviteCode != nil {
		fields["invite_code"] = *req.InviteCode
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if len(fields) > 0 {
		if err := s.repo.updatePolicy(id, fields); err != nil {
			return nil, err
		}
	}
	return s.getPolicy(id)
}

func (s *trialServiceImpl) getPolicy(id int) (*TrialPolicy, error) {
	p, err := s.repo.getPolicy(id)
	if err != nil {
		if isNotFoundTrial(err) {
			return nil, apperr.NotFound("试用策略不存在")
		}
		return nil, err
	}
	return p, nil
}

// DeletePolicy 删除策略。
func (s *trialServiceImpl) DeletePolicy(id int) error {
	if _, err := s.getPolicy(id); err != nil {
		return err
	}
	return s.repo.deletePolicy(id)
}

// ListPolicies 运营列出全部策略。
func (s *trialServiceImpl) ListPolicies() ([]TrialPolicy, error) {
	return s.repo.listPolicies(false)
}

// GrantEligibility 运营给用户授予某策略的领取资格。
func (s *trialServiceImpl) GrantEligibility(policyID, userID int, operator string) error {
	if _, err := s.getPolicy(policyID); err != nil {
		return err
	}
	return s.repo.createEligibility(&TrialEligibility{PolicyID: uint(policyID), UserID: uint(userID), GrantedBy: operator})
}

// ListGrants 某策略的发放记录。
func (s *trialServiceImpl) ListGrants(policyID int) ([]TrialGrant, error) {
	if _, err := s.getPolicy(policyID); err != nil {
		return nil, err
	}
	return s.repo.listGrants(policyID)
}
```

- [ ] **Step 2：创建 `trial_api.go`**

```go
package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ListMyTrials 前台：可领试用列表
func ListMyTrials(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := TrialService.ListClaimable(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// ClaimTrial 前台：领取试用（:code）
func ClaimTrial(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	code := c.Param("code")
	var req ClaimRequest
	_ = c.ShouldBindJSON(&req) // 邀请码可选，body 可空
	if err := TrialService.ClaimTrial(uid, code, req.InviteCode); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListTrialPolicies 后台：全部试用策略
func AdminListTrialPolicies(c *gin.Context) {
	list, err := TrialService.ListPolicies()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminCreateTrialPolicy 后台：新建试用策略
func AdminCreateTrialPolicy(c *gin.Context) {
	var req TrialPolicyCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := TrialService.CreatePolicy(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, p)
}

// AdminUpdateTrialPolicy 后台：更新试用策略
func AdminUpdateTrialPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req TrialPolicyUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := TrialService.UpdatePolicy(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, p)
}

// AdminDeleteTrialPolicy 后台：删除试用策略
func AdminDeleteTrialPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := TrialService.DeletePolicy(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminGrantTrialEligibility 后台：给用户授予领取资格（:id=policyID）
func AdminGrantTrialEligibility(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	staffID, _ := currentUserID(c)
	var req EligibilityGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := TrialService.GrantEligibility(pid, req.UserID, "staff:"+strconv.Itoa(staffID)); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListTrialGrants 后台：某策略发放记录（:id=policyID）
func AdminListTrialGrants(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	list, err := TrialService.ListGrants(pid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}
```

- [ ] **Step 3：`module.go` 加路由**

前台 `/billing` 组内追加：
```go
		g.GET("/trials", ListMyTrials)
		g.POST("/trials/:code/claim", ClaimTrial)
```
后台 `/admin/billing` 组内追加：
```go
		admin.GET("/trials", staff.PermissionMiddleware("billing:view"), AdminListTrialPolicies)
		admin.POST("/trials", staff.PermissionMiddleware("billing:manage"), AdminCreateTrialPolicy)
		admin.PUT("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminUpdateTrialPolicy)
		admin.DELETE("/trials/:id", staff.PermissionMiddleware("billing:manage"), AdminDeleteTrialPolicy)
		admin.POST("/trials/:id/eligibility", staff.PermissionMiddleware("billing:manage"), AdminGrantTrialEligibility)
		admin.GET("/trials/:id/grants", staff.PermissionMiddleware("billing:view"), AdminListTrialGrants)
```

> Gin 冲突检查：前台 `/trials`、`/trials/:code/claim`（`:code` 命名参数）与既有 `/orders/:id`、`/skus` 等分属不同前缀；后台 `/trials`、`/trials/:id`、`/trials/:id/eligibility`、`/trials/:id/grants` 同前缀同参名 `:id`，无冲突。**装配后跑全量测试**（apptest 启动暴露任何 panic）。

- [ ] **Step 4：全量回归 + 质量门（paste 输出）**

```bash
cd /home/root/workspace005/gloryphone-code/backend
gofmt -l modules/billing/
go vet ./...
go test ./... 2>&1 | tail -20
go test ./framework/ -run TestModuleBoundaries -v 2>&1 | tail -5
go build ./...
```
Expect 全 ok、边界 PASS。

- [ ] **Step 5：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 后台试用配置/授予资格/发放记录 + 前台可领/领取 API与路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 5 验收对照（PRD §7）

| 需求 | 覆盖 |
|---|---|
| 每 SKU/策略 配置试用（发放科目/数量/有效期/限领） | Task 1 模型 + Task 3 CreatePolicy |
| 资格：新用户（从未付费） | Task 2 eligible/ClaimTrial（countPaidOrders==0） |
| 资格：运营手动授予 | Task 1 TrialEligibility + Task 3 GrantEligibility |
| 资格：每用户限领次数（去重） | Task 2 countClaims + claim 守卫 |
| 资格：活动/邀请码 | Task 2 InviteCode 校验 |
| 领取原子发放（复用 Grant，流水 trial） | Task 2 claim（tx + 权益批次 + ledger type=trial）（AC-7） |
| 前台「可领/领取」、后台配置 + 发放记录 | Task 3 |

> 「按 SKU 配置」以独立 TrialPolicy（含 grant_subject/quantity）实现，等价于按可购买项配置试用内容；不强绑 SKU 表外键，更灵活。余额型试用本期不做（后台 AdjustBalance 覆盖）。

## Self-Review 记录

- **Spec 覆盖**：覆盖 PRD §7 全部（4 维资格 + 发放 + 前后台）；余额型试用、注册N天内（需 user.created_at）明确延后。
- **占位扫描**：Task 1 `claim` 占位于 Task 2 替换，已标注；其余完整代码+命令。
- **类型一致**：`TrialPolicy/TrialGrant/TrialEligibility/ClaimableItem` 及 DTO、`ClaimTrial(userID,code,inviteCode)`/`ListClaimable`/`CreatePolicy/UpdatePolicy/DeletePolicy/ListPolicies/GrantEligibility/ListGrants`/`claim(policy,userID,expireAt)` 贯穿一致；`LedgerTrial`/`SourceTrial` 一致；复用 `gormEntitlementRepository`/`resourceSubjects`/`Order`(countPaidOrders)。
- **事务/去重/幂等**：claim 单事务（限领守卫 + TrialGrant + 权益发放）；服务层先校验资格、事务内再校验限领防并发双领。
- **新用户判定**：billing 本地「已付订单数==0」，自包含。
