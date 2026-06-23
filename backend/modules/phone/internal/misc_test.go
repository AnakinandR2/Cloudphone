package phone

import (
	"context"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
	"manager-backend/modules/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func miscCleanup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "run_sessions", "cp_tasks",
			"billing_license_units", "billing_ledger_entries",
			"billing_seat_usages", "billing_dunning_states",
			"billing_entitlement_batches", "billing_runtime_minute_wallets")
	})
}

// runReconcilePatrol：对所有有「非回收」实例的用户跑 reconcile，溢出实例回收。
func TestRunReconcilePatrol(t *testing.T) {
	miscCleanup(t)
	const u1, u2 = 940101, 940102
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	base := time.Now().Add(-time.Hour)
	// u1：2 台但仅 1 席位 → 回收 1 台。
	for i, cp := range []string{"cp-r1", "cp-r2"} {
		require.NoError(t, framework.DB.Create(&CloudPhone{
			UserID: u1, Name: "p", CpID: cp, Status: StatusStopped,
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}
	require.NoError(t, billing.GrantSeatLicensesForTest(u1, 1))
	// u2：1 台 1 席位 → 不回收。
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u2, Name: "p", CpID: "cp-r3", Status: StatusStopped, CreatedAt: base}).Error)
	require.NoError(t, billing.GrantSeatLicensesForTest(u2, 1))

	svc.runReconcilePatrol(context.Background())

	var recycled int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND status = ?", u1, StatusRecycled).Count(&recycled)
	assert.Equal(t, int64(1), recycled, "u1 超额 1 台应被回收")
	var u2stopped int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ? AND status = ?", u2, StatusStopped).Count(&u2stopped)
	assert.Equal(t, int64(1), u2stopped, "u2 不超额不回收")
}

// runReconcilePatrol：无中台直接返回，不动数据。
func TestRunReconcilePatrolNoOps(t *testing.T) {
	miscCleanup(t)
	svc := newService(newRepository(framework.DB), nil)
	require.NotPanics(t, func() { svc.runReconcilePatrol(context.Background()) })
}

// syncRunSessions：拉中台运行日志并 upsert 到本地（仅我方用户的入库）。
func TestSyncRunSessions(t *testing.T) {
	miscCleanup(t)
	const u = 940103
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "s", CpID: "cp-sync", Status: StatusRunning}).Error)

	on := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	f := &fakePort{runLogPage: &midplat.RunLogPage{
		PageNum: 1, PageSize: 100, TotalSize: 1,
		Data: []midplat.RunLogEntry{
			{LogNo: "RL-1", CpID: "cp-sync", PowerOnTime: on, PowerOffTime: "运行中", SessionStatus: "运行中"},
			{LogNo: "RL-2", CpID: "cp-foreign", PowerOnTime: on, PowerOffTime: "运行中"}, // 非我方用户 → 丢弃
		},
	}}
	svc := newService(newRepository(framework.DB), f)
	svc.syncRunSessions(context.Background())

	var n int64
	framework.DB.Model(&RunSession{}).Where("cp_id = ?", "cp-sync").Count(&n)
	assert.Equal(t, int64(1), n, "我方实例的会话应入库")
	framework.DB.Model(&RunSession{}).Where("cp_id = ?", "cp-foreign").Count(&n)
	assert.Equal(t, int64(0), n, "非我方实例不入库")
}

// syncRunSessions：无中台直接返回。
func TestSyncRunSessionsNoOps(t *testing.T) {
	miscCleanup(t)
	svc := newService(newRepository(framework.DB), nil)
	require.NotPanics(t, func() { svc.syncRunSessions(context.Background()) })
}

// OwnedCpIDs / OwnsCpID：只返回本人已开通(cpId 非空)的实例，越权返回 false。
func TestOwnedCpIDsAndOwnsCpID(t *testing.T) {
	miscCleanup(t)
	const u = 940104
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "a", CpID: "cp-mine", Status: StatusRunning}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "b", CpID: "", Status: StatusCreated}).Error) // 未开通
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: userB, Name: "c", CpID: "cp-other", Status: StatusRunning}).Error)

	svc := newService(newRepository(framework.DB), &fakeOps{})

	ids, err := svc.OwnedCpIDs(u)
	require.NoError(t, err)
	assert.Equal(t, []string{"cp-mine"}, ids)

	owns, err := svc.OwnsCpID(u, "cp-mine")
	require.NoError(t, err)
	assert.True(t, owns)

	owns, err = svc.OwnsCpID(u, "cp-other")
	require.NoError(t, err)
	assert.False(t, owns, "他人实例不归本人")

	owns, err = svc.OwnsCpID(u, "")
	require.NoError(t, err)
	assert.False(t, owns, "空 cpId 直接 false")
}

// AdminListTags：跨用户标签去重。
func TestAdminListTags(t *testing.T) {
	miscCleanup(t)
	a := CloudPhone{UserID: 940105, Name: "a", Status: StatusCreated}
	b := CloudPhone{UserID: 940106, Name: "b", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&a).Error)
	require.NoError(t, framework.DB.Create(&b).Error)

	svc := newService(newRepository(framework.DB), &fakeOps{})
	require.NoError(t, svc.SetTags(940105, []int{int(a.ID)}, []Tag{{Name: "游戏"}}))
	require.NoError(t, svc.SetTags(940106, []int{int(b.ID)}, []Tag{{Name: "游戏"}, {Name: "测试"}}))

	tags, err := svc.AdminListTags()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, tg := range tags {
		names[tg.Name] = true
	}
	assert.True(t, names["游戏"])
	assert.True(t, names["测试"])
	assert.Len(t, tags, 2, "跨用户去重后应为 2 个标签")
}
