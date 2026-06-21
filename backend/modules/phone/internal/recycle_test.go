package phone

import (
	"context"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recycleCleanup 清理本组测试涉及的表（不截断共享 seed 表）。
func recycleCleanup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "run_sessions",
			"billing_license_units", "billing_ledger_entries",
			"billing_seat_usages", "billing_dunning_states",
			"billing_entitlement_batches", "billing_runtime_minute_wallets")
	})
}

// TestReconcileSeatsRecyclesOverflow：席位不足时最新创建的超额实例进回收站。
func TestReconcileSeatsRecyclesOverflow(t *testing.T) {
	recycleCleanup(t)
	const u = 9701
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	base := time.Now().Add(-time.Hour)
	for i, cp := range []string{"cpO1", "cpO2", "cpO3"} {
		require.NoError(t, framework.DB.Create(&CloudPhone{
			UserID: u, Name: "p", CpID: cp, Status: StatusStopped,
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}
	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1)) // 仅 1 席位 → 回收 2 个最新

	require.NoError(t, svc.reconcileSeats(context.Background(), u))

	// 最早 cpO1 保留 STOPPED；cpO2/cpO3 RECYCLED。
	var kept, recycled int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND status = ?", u, StatusStopped).Count(&kept)
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND status = ?", u, StatusRecycled).Count(&recycled)
	assert.Equal(t, int64(1), kept)
	assert.Equal(t, int64(2), recycled)

	// 回收实例带 recycled_at。
	var recs []CloudPhone
	framework.DB.Where("user_id = ? AND status = ?", u, StatusRecycled).Find(&recs)
	for _, r := range recs {
		assert.NotNil(t, r.RecycledAt)
	}
}

// TestReconcileForceStopsRunningOverflow：运行中的超额实例先强制关机再回收。
func TestReconcileForceStopsRunningOverflow(t *testing.T) {
	recycleCleanup(t)
	const u = 9702
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	base := time.Now().Add(-time.Hour)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "keep", CpID: "cpKeep", Status: StatusStopped, CreatedAt: base}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "run", CpID: "cpRun", Status: StatusRunning, CreatedAt: base.Add(time.Minute)}).Error)
	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1))

	require.NoError(t, svc.reconcileSeats(context.Background(), u))

	assert.Contains(t, fake.shutdown, "cpRun") // 运行中的被强制关机
	var status string
	framework.DB.Model(&CloudPhone{}).Where("cp_id = ?", "cpRun").Pluck("status", &status)
	assert.Equal(t, StatusRecycled, status)
}

// TestReconcileExcludesRecycled：回收态实例不参与 reconcile 计数。
func TestReconcileExcludesRecycled(t *testing.T) {
	recycleCleanup(t)
	const u = 9703
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	now := time.Now()
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "live", CpID: "cpLive", Status: StatusStopped, CreatedAt: now.Add(-time.Hour)}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "rec", CpID: "cpRec", Status: StatusRecycled, RecycledAt: &now, CreatedAt: now}).Error)
	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1)) // 1 席位，仅 1 非回收实例 → 无溢出

	require.NoError(t, svc.reconcileSeats(context.Background(), u))

	var liveStatus string
	framework.DB.Model(&CloudPhone{}).Where("cp_id = ?", "cpLive").Pluck("status", &liveStatus)
	assert.Equal(t, StatusStopped, liveStatus) // 未被回收（回收态实例没占席位）
}

// TestRecycleBinList：列出回收站实例并带剩余清理天数。
func TestRecycleBinList(t *testing.T) {
	recycleCleanup(t)
	const u = 9704
	svc := newService(newRepository(framework.DB), &fakeOps{})

	rt := time.Now().Add(-5 * 24 * time.Hour) // 5 天前回收
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "r1", CpID: "cpR1", Status: StatusRecycled, RecycledAt: &rt, RecycleReason: "席位不足"}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "live", CpID: "cpL", Status: StatusStopped}).Error)

	items, err := svc.RecycleBinList(u)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "r1", items[0].Name)
	assert.Equal(t, "席位不足", items[0].RecycleReason)
	// 保留 30 天，5 天前回收 → 剩余约 25 天。
	assert.InDelta(t, 25, items[0].DaysRemaining, 1)
}

// TestRecycleBinRestoreNeedsFreeSeat：恢复需有空闲席位，不足 → 422。
func TestRecycleBinRestoreNeedsFreeSeat(t *testing.T) {
	recycleCleanup(t)
	const u = 9705
	svc := newService(newRepository(framework.DB), &fakeOps{})

	now := time.Now()
	var live = CloudPhone{UserID: u, Name: "live", CpID: "cpL", Status: StatusStopped, CreatedAt: now.Add(-time.Hour)}
	require.NoError(t, framework.DB.Create(&live).Error)
	rec := CloudPhone{UserID: u, Name: "rec", CpID: "cpR", Status: StatusRecycled, RecycledAt: &now}
	require.NoError(t, framework.DB.Create(&rec).Error)

	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1)) // 1 席位已被 live 占用 → 无空闲

	err := svc.RecycleBinRestore(context.Background(), u, int(rec.ID))
	require.Error(t, err) // 席位不足
	var st string
	framework.DB.Model(&CloudPhone{}).Where("id = ?", rec.ID).Pluck("status", &st)
	assert.Equal(t, StatusRecycled, st) // 仍在回收站

	// 再发 1 席位 → 有空闲 → 可恢复。
	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1))
	require.NoError(t, svc.RecycleBinRestore(context.Background(), u, int(rec.ID)))
	framework.DB.Model(&CloudPhone{}).Where("id = ?", rec.ID).Pluck("status", &st)
	assert.Equal(t, StatusStopped, st) // 恢复为 STOPPED
}

// TestRecycleBinRestoreIsolation：不能恢复他人的回收实例。
func TestRecycleBinRestoreIsolation(t *testing.T) {
	recycleCleanup(t)
	const owner, other = 9706, 9707
	svc := newService(newRepository(framework.DB), &fakeOps{})
	now := time.Now()
	rec := CloudPhone{UserID: owner, Name: "rec", CpID: "cpR", Status: StatusRecycled, RecycledAt: &now}
	require.NoError(t, framework.DB.Create(&rec).Error)
	require.NoError(t, billing.GrantSeatLicensesForTest(other, 5))

	err := svc.RecycleBinRestore(context.Background(), other, int(rec.ID))
	require.Error(t, err) // 非属主 → 不存在
}

// TestRecycleCleanupExpired：超保留天数的回收实例被销毁并硬删。
func TestRecycleCleanupExpired(t *testing.T) {
	recycleCleanup(t)
	const u = 9708
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	old := time.Now().Add(-40 * 24 * time.Hour)  // 40 天前，超 30 天保留
	fresh := time.Now().Add(-2 * 24 * time.Hour) // 2 天前，未超
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "old", CpID: "cpOld", Status: StatusRecycled, RecycledAt: &old}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "fresh", CpID: "cpFresh", Status: StatusRecycled, RecycledAt: &fresh}).Error)

	svc.runRecycleCleanup(context.Background())

	assert.Equal(t, []string{"cpOld"}, fake.destroyed) // 仅过期的被中台销毁
	var n int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND cp_id = ?", u, "cpOld").Count(&n)
	assert.Equal(t, int64(0), n) // 硬删
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND cp_id = ?", u, "cpFresh").Count(&n)
	assert.Equal(t, int64(1), n) // 保留
}

// TestCreateGatedBySeatCapacity：创建受新 seat 容量约束（SeatCapacity vs 当前非回收实例数）。
func TestCreateGatedBySeatCapacity(t *testing.T) {
	recycleCleanup(t)
	const u = 9710
	svc := newService(newRepository(framework.DB), nil) // 无中台：本地降级直接落 CREATED

	_, err := svc.Create(u, &CloudPhoneCreate{Name: "x"})
	require.Error(t, err) // 无席位 → 拒

	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1))
	_, err = svc.Create(u, &CloudPhoneCreate{Name: "a"})
	require.NoError(t, err) // 1 席位 → 可
	_, err = svc.Create(u, &CloudPhoneCreate{Name: "b"})
	require.Error(t, err) // 超席位 → 拒
}

// TestCanBootGatesPower：开机前置校验——无空闲包月名额且无临时时长 → 拒绝。
func TestCanBootGate(t *testing.T) {
	recycleCleanup(t)
	const u = 9709
	can, err := billing.CanBoot(u)
	require.NoError(t, err)
	assert.False(t, can) // 无资源 → 不可开机

	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(u, 100))
	can, err = billing.CanBoot(u)
	require.NoError(t, err)
	assert.True(t, can) // 有临时时长 → 可开机
}
