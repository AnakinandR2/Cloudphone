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
