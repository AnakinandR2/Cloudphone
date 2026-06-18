package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trialUser = 9201

// trialTables 试用相关全部表（清理用）。
var trialTables = []string{
	"billing_trial_policies", "billing_trial_policy_items", "billing_trial_claims",
	"billing_trial_grants", "billing_trial_eligibilities",
	"billing_orders", "billing_entitlement_batches", "billing_ledger_entries",
}

// oneItem 便捷构造单发放项策略（测试用）。
func oneItem(subject string, qty int64, expireDays int) []TrialPolicyItem {
	return []TrialPolicyItem{{Subject: subject, Quantity: qty, ExpireDays: expireDays}}
}

func TestTrialPolicyCRUDAndHelpers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, PerUserLimit: 1, AllowNewUser: true, Items: oneItem(SubjectInstanceSeat, 1, 0)}))
	p, err := repo.getPolicyByCode("newbie")
	require.NoError(t, err)
	assert.Equal(t, "新人礼", p.Name)
	require.Len(t, p.Items, 1)
	assert.Equal(t, SubjectInstanceSeat, p.Items[0].Subject)

	n, err := repo.countClaims(int(p.ID), trialUser)
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	ok, err := repo.manualEligible(int(p.ID), trialUser)
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, repo.createEligibility(&TrialEligibility{PolicyID: p.ID, UserID: uint(trialUser), GrantedBy: "staff:1"}))
	ok, _ = repo.manualEligible(int(p.ID), trialUser)
	assert.True(t, ok)

	cnt, err := repo.countPaidOrders(trialUser)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)
	require.NoError(t, framework.DB.Create(&Order{OrderNo: "BILX1", UserID: uint(trialUser), Status: OrderPaid, PayMethod: PayBalance, TotalCents: 1}).Error)
	cnt, _ = repo.countPaidOrders(trialUser)
	assert.Equal(t, 1, cnt)

	list, err := repo.listPolicies(true)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestClaimTrialNewUserAndDedup(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, PerUserLimit: 1, AllowNewUser: true, Items: oneItem(SubjectInstanceSeat, 1, 30)}))

	require.NoError(t, TrialService.ClaimTrial(trialUser, "newbie", ""))
	cap, err := EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cap)

	// 限领按 TrialClaim 计：第 2 次被拒，容量不变。
	err = TrialService.ClaimTrial(trialUser, "newbie", "")
	assert.Error(t, err)
	cap, _ = EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	assert.Equal(t, int64(1), cap)

	_, total, err := BillingService.ListLedger(trialUser, 1, 50, SubjectInstanceSeat, LedgerTrial)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

// 一次领取同时发放三类资源：三类容量均到账、写 1 个 TrialClaim + 3 条 TrialGrant；每项独立到期。
func TestClaimTrialMultiResource(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{
		Code: "gift", Name: "大礼包", Enabled: true, PerUserLimit: 1, AllowNewUser: true,
		Items: []TrialPolicyItem{
			{Subject: SubjectInstanceSeat, Quantity: 2, ExpireDays: 30},
			{Subject: SubjectRuntimeMinute, Quantity: 600, ExpireDays: 0}, // 永久
			{Subject: SubjectBootSeat, Quantity: 1, ExpireDays: 30},
		},
	}))

	require.NoError(t, TrialService.ClaimTrial(trialUser, "gift", ""))

	inst, _ := EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	mins, _ := EntitlementService.Capacity(trialUser, SubjectRuntimeMinute)
	boot, _ := EntitlementService.Capacity(trialUser, SubjectBootSeat)
	assert.Equal(t, int64(2), inst)
	assert.Equal(t, int64(600), mins)
	assert.Equal(t, int64(1), boot)

	// 1 个领取头 + 3 条发放明细。
	p, _ := repo.getPolicyByCode("gift")
	var claims int64
	framework.DB.Model(&TrialClaim{}).Where("policy_id = ?", p.ID).Count(&claims)
	assert.Equal(t, int64(1), claims)
	grants, _ := repo.listGrants(int(p.ID))
	assert.Len(t, grants, 3)

	// 每项独立到期：runtime_minute 批次永久(nil)，instance_seat 批次有到期。
	var permMins int64
	framework.DB.Model(&EntitlementBatch{}).Where("user_id = ? AND subject = ? AND expire_at IS NULL", trialUser, SubjectRuntimeMinute).Count(&permMins)
	assert.Equal(t, int64(1), permMins)
	var datedSeat int64
	framework.DB.Model(&EntitlementBatch{}).Where("user_id = ? AND subject = ? AND expire_at IS NOT NULL", trialUser, SubjectInstanceSeat).Count(&datedSeat)
	assert.Equal(t, int64(1), datedSeat)

	// 限领 1：再领被拒。
	assert.Error(t, TrialService.ClaimTrial(trialUser, "gift", ""))
}

func TestClaimTrialEligibilityPaths(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, PerUserLimit: 1, InviteCode: "VIP2026", Items: oneItem(SubjectRuntimeMinute, 600, 0)}))
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", ""))
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", "WRONG"))
	require.NoError(t, TrialService.ClaimTrial(trialUser, "promo", "VIP2026"))
	cap, _ := EntitlementService.Capacity(trialUser, SubjectRuntimeMinute)
	assert.Equal(t, int64(600), cap)

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newonly", Name: "仅新人", Enabled: true, PerUserLimit: 1, AllowNewUser: true, Items: oneItem(SubjectBootSeat, 2, 0)}))
	require.NoError(t, framework.DB.Create(&Order{OrderNo: "BILY1", UserID: uint(trialUser), Status: OrderPaid, PayMethod: PayBalance, TotalCents: 1}).Error)
	assert.Error(t, TrialService.ClaimTrial(trialUser, "newonly", ""))
	pol, _ := repo.getPolicyByCode("newonly")
	require.NoError(t, repo.createEligibility(&TrialEligibility{PolicyID: pol.ID, UserID: uint(trialUser), GrantedBy: "staff:1"}))
	require.NoError(t, TrialService.ClaimTrial(trialUser, "newonly", ""))
	cap, _ = EntitlementService.Capacity(trialUser, SubjectBootSeat)
	assert.Equal(t, int64(2), cap)
}

func TestListClaimable(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, PerUserLimit: 1, AllowNewUser: true, Items: oneItem(SubjectInstanceSeat, 1, 0)}))
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, PerUserLimit: 1, InviteCode: "VIP", Items: oneItem(SubjectRuntimeMinute, 600, 0)}))

	items, err := TrialService.ListClaimable(trialUser)
	require.NoError(t, err)
	require.Len(t, items, 2)
	byCode := map[string]ClaimableItem{}
	for _, it := range items {
		byCode[it.Policy.Code] = it
	}
	assert.True(t, byCode["newbie"].Claimable)
	require.Len(t, byCode["newbie"].Policy.Items, 1) // 发放项随策略返回
	assert.False(t, byCode["promo"].Claimable)
	assert.True(t, byCode["promo"].NeedInvite)
}

func TestListClaimableHidesInviteCode(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, PerUserLimit: 1, InviteCode: "SECRET", Items: oneItem(SubjectRuntimeMinute, 600, 0)}))

	items, err := TrialService.ListClaimable(trialUser)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "", items[0].Policy.InviteCode) // 前台不泄露邀请码
	assert.True(t, items[0].NeedInvite)             // 但提示需要邀请码
}

func TestTrialAdminPolicyService(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })

	// 无发放项 → 拒绝
	_, err := TrialService.CreatePolicy(&TrialPolicyCreate{Code: "empty", Name: "x"})
	assert.Error(t, err)
	// 试用只能发资源科目 → balance 被拒
	_, err = TrialService.CreatePolicy(&TrialPolicyCreate{Code: "bad", Name: "x", Items: []TrialPolicyItemInput{{Subject: SubjectBalance, Quantity: 100}}})
	assert.Error(t, err)
	// 科目重复 → 拒绝
	_, err = TrialService.CreatePolicy(&TrialPolicyCreate{Code: "dupsub", Name: "x", Items: []TrialPolicyItemInput{{Subject: SubjectInstanceSeat, Quantity: 1}, {Subject: SubjectInstanceSeat, Quantity: 2}}})
	assert.Error(t, err)

	p, err := TrialService.CreatePolicy(&TrialPolicyCreate{Code: "newbie", Name: "新人礼", AllowNewUser: true, Items: []TrialPolicyItemInput{{Subject: SubjectInstanceSeat, Quantity: 1, ExpireDays: 30}}})
	require.NoError(t, err)
	assert.Equal(t, 1, p.PerUserLimit) // 默认 1
	assert.True(t, p.Enabled)          // 默认启用
	require.Len(t, p.Items, 1)

	// 重复 code → 冲突
	_, err = TrialService.CreatePolicy(&TrialPolicyCreate{Code: "newbie", Name: "dup", Items: []TrialPolicyItemInput{{Subject: SubjectInstanceSeat, Quantity: 1}}})
	assert.Error(t, err)

	// 更新：替换发放项 + 停用
	disabled := false
	upd, err := TrialService.UpdatePolicy(int(p.ID), &TrialPolicyUpdate{
		Items:   &[]TrialPolicyItemInput{{Subject: SubjectRuntimeMinute, Quantity: 500, ExpireDays: 0}},
		Enabled: &disabled,
	})
	require.NoError(t, err)
	assert.False(t, upd.Enabled)
	require.Len(t, upd.Items, 1)
	assert.Equal(t, SubjectRuntimeMinute, upd.Items[0].Subject)
	assert.Equal(t, int64(500), upd.Items[0].Quantity)

	// 授予资格 + 列出（无领取记录时为空）
	require.NoError(t, TrialService.GrantEligibility(int(p.ID), 9301, "staff:1"))
	grants, err := TrialService.ListGrants(int(p.ID))
	require.NoError(t, err)
	assert.Len(t, grants, 0)

	// ListPolicies 含全部（含已停用）
	all, err := TrialService.ListPolicies()
	require.NoError(t, err)
	require.Len(t, all, 1)

	// 删除
	require.NoError(t, TrialService.DeletePolicy(int(p.ID)))
	_, err = TrialService.UpdatePolicy(int(p.ID), &TrialPolicyUpdate{Name: "ghost"})
	assert.Error(t, err) // 已删 → NotFound
}
