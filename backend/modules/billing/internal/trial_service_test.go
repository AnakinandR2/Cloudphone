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
	t.Cleanup(func() {
		framework.CleanTable("billing_trial_policies", "billing_trial_grants", "billing_trial_eligibilities",
			"billing_orders", "billing_entitlement_batches", "billing_ledger_entries")
	})
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newbie", Name: "新人礼", Enabled: true, GrantSubject: SubjectInstanceSeat, GrantQuantity: 1, GrantExpireDays: 30, PerUserLimit: 1, AllowNewUser: true}))

	require.NoError(t, TrialService.ClaimTrial(trialUser, "newbie", ""))
	cap, err := EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(1), cap)

	err = TrialService.ClaimTrial(trialUser, "newbie", "")
	assert.Error(t, err)
	cap, _ = EntitlementService.Capacity(trialUser, SubjectInstanceSeat)
	assert.Equal(t, int64(1), cap)

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

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "promo", Name: "活动", Enabled: true, GrantSubject: SubjectRuntimeMinute, GrantQuantity: 600, PerUserLimit: 1, InviteCode: "VIP2026"}))
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", ""))
	assert.Error(t, TrialService.ClaimTrial(trialUser, "promo", "WRONG"))
	require.NoError(t, TrialService.ClaimTrial(trialUser, "promo", "VIP2026"))
	cap, _ := EntitlementService.Capacity(trialUser, SubjectRuntimeMinute)
	assert.Equal(t, int64(600), cap)

	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "newonly", Name: "仅新人", Enabled: true, GrantSubject: SubjectBootSeat, GrantQuantity: 2, PerUserLimit: 1, AllowNewUser: true}))
	require.NoError(t, framework.DB.Create(&Order{OrderNo: "BILY1", UserID: uint(trialUser), Status: OrderPaid, PayMethod: PayBalance, TotalCents: 1}).Error)
	assert.Error(t, TrialService.ClaimTrial(trialUser, "newonly", ""))
	pol, _ := repo.getPolicyByCode("newonly")
	require.NoError(t, repo.createEligibility(&TrialEligibility{PolicyID: pol.ID, UserID: uint(trialUser), GrantedBy: "staff:1"}))
	require.NoError(t, TrialService.ClaimTrial(trialUser, "newonly", ""))
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
	assert.True(t, byCode["newbie"].Claimable)
	assert.False(t, byCode["promo"].Claimable)
	assert.True(t, byCode["promo"].NeedInvite)
}
