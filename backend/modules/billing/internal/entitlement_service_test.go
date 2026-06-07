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
