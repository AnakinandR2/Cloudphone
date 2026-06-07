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
	err := SeatService.TryOccupyInstanceSeat(seatUser)              // 超容量 → 拒
	assert.Error(t, err)

	require.NoError(t, SeatService.ReleaseInstanceSeat(seatUser))   // 1/2
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
