package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFulfillNew_CreatesNSeatsWithMonthExpiry(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 930101
	require.NoError(t, FulfillService.FulfillNew(uid, KindSeat, 3, 2, SourceOrder, "order-1"))

	cap, err := LicenseService.Capacity(uid, KindSeat)
	require.NoError(t, err)
	assert.Equal(t, 3, cap)

	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 3)
	// 到期约 2 个月后。
	want := time.Now().AddDate(0, 2, 0)
	assert.WithinDuration(t, want, units[0].ExpireAt, 24*time.Hour)
}

func TestFulfillNew_BootSlotUsesDayExpiry(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 930102
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 1, 30, SourceOrder, "order-2"))
	units, err := LicenseService.repo.listByUserKind(uid, KindBootSlot)
	require.NoError(t, err)
	require.Len(t, units, 1)
	want := time.Now().AddDate(0, 0, 30)
	assert.WithinDuration(t, want, units[0].ExpireAt, 2*time.Hour)
}

func TestFulfillRenew_ExtendsSelectedUnits(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 930103
	require.NoError(t, FulfillService.FulfillNew(uid, KindSeat, 2, 1, SourceOrder, "order-3"))
	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 2)

	before := units[0].ExpireAt
	require.NoError(t, FulfillService.FulfillRenew(uid, KindSeat, []uint{units[0].ID}, 3))

	after, err := LicenseService.repo.getByIDs(uid, []uint{units[0].ID}, KindSeat)
	require.NoError(t, err)
	// 续费 3 个月，新到期 ≈ before + 3 月。
	assert.WithinDuration(t, before.AddDate(0, 3, 0), after[0].ExpireAt, 24*time.Hour)
}

func TestFulfillRuntimePack_AddsMinutes(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_runtime_minute_wallets") })
	uid := 930104
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 600, SourceOrder, "order-4"))
	rem, err := RuntimeWalletService.Remaining(uid)
	require.NoError(t, err)
	assert.Equal(t, int64(600), rem)

	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 100, SourceTrial, "trial-x"))
	rem, _ = RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(700), rem)
}
