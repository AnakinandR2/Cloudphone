package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const entUser = 8101

func TestEntitlementGrantDeductWithLedger(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })

	_, err := EntitlementService.Grant(entUser, SubjectInstanceSeat, 10, nil, SourceAdjust, "运营赠送", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)
	capacity, err := EntitlementService.Capacity(entUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(10), capacity)

	_, err = EntitlementService.Grant(entUser, SubjectRuntimeMinute, 600, nil, SourceAdjust, "补偿", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)

	require.NoError(t, EntitlementService.Deduct(entUser, SubjectInstanceSeat, 3, "回收", LedgerAdjustDeduct, "staff:1"))
	capacity, err = EntitlementService.Capacity(entUser, SubjectInstanceSeat)
	require.NoError(t, err)
	assert.Equal(t, int64(7), capacity)

	err = EntitlementService.Deduct(entUser, SubjectInstanceSeat, 999, "越扣", LedgerAdjustDeduct, "staff:1")
	assert.Error(t, err)

	_, total, err := BillingService.ListLedger(entUser, 1, 50, SubjectInstanceSeat, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total) // instance_seat: 1 grant + 1 deduct (the over-deduct rolled back, no ledger)

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

	_, err := EntitlementService.Grant(entUser, SubjectRuntimeMinute, 100, &soon, SourceAdjust, "批A", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)
	_, err = EntitlementService.Grant(entUser, SubjectRuntimeMinute, 100, &later, SourceAdjust, "批B", LedgerAdjustGrant, "staff:1")
	require.NoError(t, err)

	require.NoError(t, EntitlementService.Deduct(entUser, SubjectRuntimeMinute, 150, "消耗", LedgerConsume, "system"))

	batches, err := EntitlementService.ListBatches(entUser, SubjectRuntimeMinute)
	require.NoError(t, err)
	require.Len(t, batches, 2)
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
	assert.Error(t, err)
	_, err = EntitlementService.Grant(entUser, SubjectInstanceSeat, 0, nil, SourceAdjust, "x", LedgerAdjustGrant, "staff:1")
	assert.Error(t, err)
	err = EntitlementService.Deduct(entUser, SubjectInstanceSeat, 0, "x", LedgerAdjustDeduct, "staff:1")
	assert.Error(t, err)
}

func TestEntitlementCapacityExcludesExpiredAndUsed(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_entitlement_batches", "billing_ledger_entries") })
	repo := newEntitlementRepository(framework.DB)
	now := time.Now()
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 5, ExpireAt: &future}))
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 3}))
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 10, ExpireAt: &past}))
	require.NoError(t, repo.createBatch(&EntitlementBatch{UserID: entUser, Subject: SubjectInstanceSeat, Quantity: 2, Used: 2, ExpireAt: &future}))

	capacity, err := repo.capacity(entUser, SubjectInstanceSeat, now)
	require.NoError(t, err)
	assert.Equal(t, int64(8), capacity)

	capacity, err = repo.capacity(entUser, SubjectRuntimeMinute, now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), capacity)
}
