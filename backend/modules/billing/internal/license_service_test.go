package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseCapacity_CountsOnlyNonExpiredActive(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 920001
	now := time.Now()
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(48 * time.Hour)},
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(72 * time.Hour)},
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(-1 * time.Hour)}, // 已过期
		{UserID: uint(uid), Kind: KindBootSlot, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(48 * time.Hour)},
	}))

	cap, err := LicenseService.Capacity(uid, KindSeat)
	require.NoError(t, err)
	assert.Equal(t, 2, cap, "expired seat excluded")

	bootCap, err := LicenseService.Capacity(uid, KindBootSlot)
	require.NoError(t, err)
	assert.Equal(t, 1, bootCap)
}

func TestReconcileSeats_OverflowRecycledAndOccupancyMaterialized(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 920002
	now := time.Now()
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(48 * time.Hour)},
	}))
	instances := []InstanceRef{
		{CpID: "cp-a", CreatedAt: now.Add(-72 * time.Hour)},
		{CpID: "cp-b", CreatedAt: now.Add(-24 * time.Hour)},
	}

	recycle, err := LicenseService.ReconcileSeats(uid, instances)
	require.NoError(t, err)
	// 只有1个席位，2台实例 → 最新的 cp-b 溢出回收。
	require.Equal(t, []string{"cp-b"}, recycle)

	// 席位上应坐着最老的 cp-a。
	units, err := LicenseService.repo.activeUnits(uid, KindSeat, now)
	require.NoError(t, err)
	require.Len(t, units, 1)
	assert.Equal(t, "cp-a", units[0].CurrentInstanceID)
}
