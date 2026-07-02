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

// 注：cleanupLicenseUnitsByUser(t *testing.T, uid int) 已由 Task 10 在同包
// （modules/billing/internal，license_repository_test.go）定义，本文件直接复用，勿重复定义。

// TestFulfillRenew_StacksFromOriginalExpiryWhenActive 未过期单元续费：新到期从原到期叠加（非从 now）。
func TestFulfillRenew_StacksFromOriginalExpiryWhenActive(t *testing.T) {
	uid := 930111
	cleanupLicenseUnitsByUser(t, uid)

	now := time.Now()
	orig := now.AddDate(0, 0, 10) // 原到期 = now+10 天（未过期）
	unit := LicenseUnit{
		UserID: uint(uid), Kind: KindSeat, Status: LicenseActive,
		Source: SourceOrder, SourceRef: "renew-active", ExpireAt: orig,
	}
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{unit}))

	created, err := LicenseService.repo.activeUnits(uid, KindSeat, now)
	require.NoError(t, err)
	require.Len(t, created, 1)
	id := created[0].ID
	origSaved := created[0].ExpireAt // 以 DB 落库后的到期为锚点，规避时区/精度误差

	require.NoError(t, FulfillService.FulfillRenew(uid, KindSeat, []uint{id}, 1))

	after, err := LicenseService.repo.getByIDs(uid, []uint{id}, KindSeat)
	require.NoError(t, err)
	require.Len(t, after, 1)
	// 从原到期起算：新到期 ≈ 原到期 + 1 月，且明显晚于「从 now 起算」(now+1 月)。
	assert.WithinDuration(t, origSaved.AddDate(0, 1, 0), after[0].ExpireAt, 2*time.Hour)
	assert.True(t, after[0].ExpireAt.After(now.AddDate(0, 1, 0)),
		"未过期续费必须从原到期叠加，应晚于从 now 起算")
}

// TestFulfillRenew_StacksFromNowWhenExpired 已过期单元续费：从 now 起算（非从旧到期叠加）。
func TestFulfillRenew_StacksFromNowWhenExpired(t *testing.T) {
	uid := 930112
	cleanupLicenseUnitsByUser(t, uid)

	now := time.Now()
	stale := now.AddDate(0, 0, -5) // 原到期 = now-5 天（已过期）
	unit := LicenseUnit{
		UserID: uint(uid), Kind: KindSeat, Status: LicenseActive,
		Source: SourceOrder, SourceRef: "renew-expired", ExpireAt: stale,
	}
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{unit}))

	// activeUnits 过滤 expire_at>now，已过期单元查不到，改用按属主的一次性查回 ID。
	var all []LicenseUnit
	require.NoError(t, framework.DB.Where("user_id = ? AND kind = ?", uid, KindSeat).Find(&all).Error)
	require.Len(t, all, 1)
	id := all[0].ID

	renewAt := time.Now()
	require.NoError(t, FulfillService.FulfillRenew(uid, KindSeat, []uint{id}, 1))

	after, err := LicenseService.repo.getByIDs(uid, []uint{id}, KindSeat)
	require.NoError(t, err)
	require.Len(t, after, 1)
	// 从 now 起算：新到期 ≈ renewAt + 1 月；绝不是从旧到期(now-5d)叠加。
	assert.WithinDuration(t, renewAt.AddDate(0, 1, 0), after[0].ExpireAt, 24*time.Hour)
	assert.True(t, after[0].ExpireAt.After(stale.AddDate(0, 1, 0)),
		"已过期续费应从 now 起算，晚于从旧到期叠加")
}

// TestGrantTrialItem_ZeroExpireDaysIsPerpetual ExpireDays=0 视为永久：到期≈now+100 年，50 年后仍未过期。
func TestGrantTrialItem_ZeroExpireDaysIsPerpetual(t *testing.T) {
	uid := 930113
	cleanupLicenseUnitsByUser(t, uid)

	now := time.Now()
	it := TrialPolicyItem{Subject: KindSeat, Quantity: 1, ExpireDays: 0} // 0=永久
	balanceAfter, err := grantTrialItem(framework.DB, uid, it, now, "trial:perpetual")
	require.NoError(t, err)
	assert.Equal(t, int64(1), balanceAfter)

	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 1)
	// 到期 ≈ now + 100 年。
	assert.WithinDuration(t, now.AddDate(100, 0, 0), units[0].ExpireAt, 48*time.Hour)

	// 50 年后仍属未过期可用池（activeUnits 过滤 expire_at>now）。
	future := now.AddDate(50, 0, 0)
	active, err := LicenseService.repo.activeUnits(uid, KindSeat, future)
	require.NoError(t, err)
	assert.Len(t, active, 1, "永久单元 50 年后仍应 active")
}
