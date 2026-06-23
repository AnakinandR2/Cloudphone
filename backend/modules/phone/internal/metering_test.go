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

func meteringCleanup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "run_sessions",
			"billing_license_units", "billing_ledger_entries",
			"billing_runtime_minute_wallets", "billing_seat_usages",
			"billing_dunning_states", "billing_entitlement_batches",
			"billing_runtime_settlements")
	})
}

// runSettlement：取近 6h 有交集的会话，按用户调 billing.SettleRuntime 结算（best-effort，不 panic）。
func TestRunSettlement(t *testing.T) {
	meteringCleanup(t)
	const u = 960101
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(u, 1000))

	on := time.Now().Add(-30 * time.Minute)
	off := time.Now().Add(-5 * time.Minute)
	// 一条已关机会话 + 一条运行中会话。
	require.NoError(t, framework.DB.Create(&RunSession{
		LogNo: "S-set-1", CpID: "cp-set", UserID: u,
		PowerOnAt: on, PowerOffAt: &off, SessionStatus: "SHUTDOWN",
	}).Error)
	require.NoError(t, framework.DB.Create(&RunSession{
		LogNo: "S-set-2", CpID: "cp-set2", UserID: u,
		PowerOnAt: time.Now().Add(-10 * time.Minute), SessionStatus: "RUNNING",
	}).Error)

	svc := newService(newRepository(framework.DB), &fakeOps{})
	// 不应 panic；结算逻辑跑通即覆盖。
	require.NotPanics(t, func() { svc.runSettlement(context.Background()) })
}

// runSettlement：仓库错误时静默返回（这里用空数据验证无会话也安全）。
func TestRunSettlementEmpty(t *testing.T) {
	meteringCleanup(t)
	svc := newService(newRepository(framework.DB), &fakeOps{})
	require.NotPanics(t, func() { svc.runSettlement(context.Background()) })
}

// runRuntimeGuard：用户运行中实例数超过开机名额(bootCap)且无临时时长覆盖 → 关停溢出的（后开先关）。
func TestRunRuntimeGuardShutsDownOverflow(t *testing.T) {
	meteringCleanup(t)
	const u = 960102
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	// 两台运行中实例，但用户 0 开机名额、0 临时时长 → 两台都无覆盖 → 全关停。
	base := time.Now().Add(-time.Hour)
	for i, cp := range []string{"cp-g1", "cp-g2"} {
		require.NoError(t, framework.DB.Create(&CloudPhone{
			UserID: u, Name: "g", CpID: cp, Status: StatusRunning, ProxyID: 1,
		}).Error)
		require.NoError(t, framework.DB.Create(&RunSession{
			LogNo: "S-g-" + cp, CpID: cp, UserID: u,
			PowerOnAt: base.Add(time.Duration(i) * time.Minute), SessionStatus: "RUNNING",
		}).Error)
	}

	svc.runRuntimeGuard(context.Background())

	// 无覆盖 → 两台都被请求关机。
	assert.ElementsMatch(t, []string{"cp-g1", "cp-g2"}, fake.shutdown)
	// 本地状态置 STOPPING。
	for _, cp := range []string{"cp-g1", "cp-g2"} {
		var st string
		framework.DB.Model(&CloudPhone{}).Where("cp_id = ?", cp).Pluck("status", &st)
		assert.Equal(t, StatusStopping, st)
	}
}

// runRuntimeGuard：有足够开机名额(bootCap)覆盖全部运行台 → 不关停。
func TestRunRuntimeGuardKeepsWhenCovered(t *testing.T) {
	meteringCleanup(t)
	const u = 960103
	require.NoError(t, billing.GrantBootSlotLicensesForTest(u, 2)) // 2 开机名额覆盖 1 台
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	require.NoError(t, framework.DB.Create(&CloudPhone{
		UserID: u, Name: "k", CpID: "cp-keep", Status: StatusRunning, ProxyID: 1,
	}).Error)
	require.NoError(t, framework.DB.Create(&RunSession{
		LogNo: "S-keep", CpID: "cp-keep", UserID: u,
		PowerOnAt: time.Now().Add(-time.Hour), SessionStatus: "RUNNING",
	}).Error)

	svc.runRuntimeGuard(context.Background())
	assert.Empty(t, fake.shutdown, "有名额覆盖不应关停")
	var st string
	framework.DB.Model(&CloudPhone{}).Where("cp_id = ?", "cp-keep").Pluck("status", &st)
	assert.Equal(t, StatusRunning, st)
}

// runRuntimeGuard：临时时长足以覆盖溢出台 → 不关停。
func TestRunRuntimeGuardCoveredByMinutes(t *testing.T) {
	meteringCleanup(t)
	const u = 960104
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(u, 100)) // 有临时时长覆盖溢出
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	require.NoError(t, framework.DB.Create(&CloudPhone{
		UserID: u, Name: "m", CpID: "cp-min", Status: StatusRunning, ProxyID: 1,
	}).Error)
	require.NoError(t, framework.DB.Create(&RunSession{
		LogNo: "S-min", CpID: "cp-min", UserID: u,
		PowerOnAt: time.Now().Add(-time.Hour), SessionStatus: "RUNNING",
	}).Error)

	svc.runRuntimeGuard(context.Background())
	assert.Empty(t, fake.shutdown, "有临时时长覆盖不应关停")
}

// runRuntimeGuard：无中台(ops=nil)直接返回。
func TestRunRuntimeGuardNoOps(t *testing.T) {
	meteringCleanup(t)
	svc := newService(newRepository(framework.DB), nil)
	require.NotPanics(t, func() { svc.runRuntimeGuard(context.Background()) })
}

// shutdownSessions：仅关停状态为 RUNNING 且 cpId 非空的实例；过渡态/不存在跳过。
func TestShutdownSessionsSkipsNonRunning(t *testing.T) {
	meteringCleanup(t)
	const u = 960105
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)

	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "run", CpID: "cp-run", Status: StatusRunning, ProxyID: 1}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "stop", CpID: "cp-stop", Status: StatusStopping, ProxyID: 1}).Error)

	sessions := []RunSession{
		{CpID: "cp-run", UserID: u},
		{CpID: "cp-stop", UserID: u},  // 非 RUNNING → 跳过
		{CpID: "cp-ghost", UserID: u}, // 无对应实例 → 跳过
	}
	svc.shutdownSessions(context.Background(), u, sessions)

	assert.Equal(t, []string{"cp-run"}, fake.shutdown)
	var st string
	framework.DB.Model(&CloudPhone{}).Where("cp_id = ?", "cp-run").Pluck("status", &st)
	assert.Equal(t, StatusStopping, st)
}
