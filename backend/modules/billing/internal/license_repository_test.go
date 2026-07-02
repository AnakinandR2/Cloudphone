package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupLicenseUnitsByUser 只删除本测试自造的行（按唯一 uid），
// 不截断共享表（RunSetup 未 seed 授权单元，但整库测试共用一个 DB，需保持隔离）。
// Task 11（同包 fulfill_service_test.go）复用此函数，签名统一为 int。
func cleanupLicenseUnitsByUser(t *testing.T, uid int) {
	t.Helper()
	require.NoError(t, framework.DB.
		Where("user_id = ?", uid).
		Delete(&LicenseUnit{}).Error)
}

// TestActiveUnits_ExpiredFilteredAtReadTime 验证到期在「读时」被过滤：
// 即便不调 markExpired，activeUnits/Capacity 也只返回 expire_at>now 的 active 单元。
func TestActiveUnits_ExpiredFilteredAtReadTime(t *testing.T) {
	const uid = 920910
	t.Cleanup(func() { cleanupLicenseUnitsByUser(t, uid) })

	now := time.Now()
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(24 * time.Hour)},
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(-1 * time.Second)}, // 刚过期
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(30 * 24 * time.Hour)},
	}))

	// 传构造 now：只应返回 now+24h 与 now+30d 两条，now-1s 被 expire_at>now 排除。
	units, err := LicenseService.repo.activeUnits(uid, KindSeat, now)
	require.NoError(t, err)
	require.Len(t, units, 2, "expire_at<=now must be excluded at read time")
	for _, u := range units {
		assert.True(t, u.ExpireAt.After(now), "returned unit must not be expired: id=%d expireAt=%s", u.ID, u.ExpireAt)
	}

	// Capacity 走 activeUnits(time.Now())：两条远期单元（+24h/+30d）在真实 wall-clock 下仍未过期，
	// now-1s 那条早已过期，故 Capacity 同样只数到 2。
	capSeat, err := LicenseService.Capacity(uid, KindSeat)
	require.NoError(t, err)
	assert.Equal(t, 2, capSeat, "Capacity counts only non-expired active seats")
}

// TestMarkExpired_FlipsExpiredAndClearsInstance 验证 markExpired 的物化翻转语义：
// active 且 expire_at<=now 的单元被置为 expired 并清空 current_instance_id；
// 翻转后 activeUnits 结果不变（本就已被读时过滤）。
func TestMarkExpired_FlipsExpiredAndClearsInstance(t *testing.T) {
	const uid = 920911
	t.Cleanup(func() { cleanupLicenseUnitsByUser(t, uid) })

	now := time.Now()
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(24 * time.Hour)},
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, CurrentInstanceID: "cp-expired", ExpireAt: now.Add(-1 * time.Second)}, // 刚过期且占用中
		{UserID: uid, Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(30 * 24 * time.Hour)},
	}))

	// 翻转前：读时已过滤，activeUnits 只见 2 条未过期。
	before, err := LicenseService.repo.activeUnits(uid, KindSeat, now)
	require.NoError(t, err)
	require.Len(t, before, 2)

	// markExpired 目前无路由/定时任务触发（全仓仅接口声明+实现），此处直调 repo 验证其行为。
	affected, err := LicenseService.repo.markExpired(now)
	require.NoError(t, err)
	// markExpired 是全表操作，其它测试残留可能一并被翻转，故只断言"至少翻了本用例这一条"，
	// 具体行的状态用下面按 uid 的精确查询来确证。
	assert.GreaterOrEqual(t, affected, int64(1), "the just-expired seat must be affected")

	// 精确断言：本用户仅那条 now-1s 单元被翻成 expired 且 current_instance_id 清空。
	var mine []LicenseUnit
	require.NoError(t, framework.DB.
		Where("user_id = ?", uid).
		Order("expire_at DESC").
		Find(&mine).Error)
	require.Len(t, mine, 3)

	var expiredCount, activeCount int
	for _, u := range mine {
		switch {
		case u.ExpireAt.After(now):
			assert.Equal(t, LicenseActive, u.Status, "future-dated unit stays active: id=%d", u.ID)
			activeCount++
		default:
			assert.Equal(t, LicenseExpired, u.Status, "past-dated unit flipped to expired: id=%d", u.ID)
			assert.Equal(t, "", u.CurrentInstanceID, "expired unit must release its instance: id=%d", u.ID)
			expiredCount++
		}
	}
	assert.Equal(t, 1, expiredCount, "exactly one seat was expired")
	assert.Equal(t, 2, activeCount, "two future seats untouched")

	// 翻转后再读 activeUnits：结果不变（读时过滤与物化翻转口径一致）。
	after, err := LicenseService.repo.activeUnits(uid, KindSeat, now)
	require.NoError(t, err)
	require.Len(t, after, 2, "activeUnits unchanged after markExpired")
}
