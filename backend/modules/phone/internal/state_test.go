package phone

import (
	"context"
	"errors"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"
	"manager-backend/modules/proxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertPhone 直接落一台指定状态/ cpId 的云手机（默认带 ProxyID=1，满足开机门禁「必须绑代理」）。
func insertPhone(t *testing.T, userID int, status, cpID string) *CloudPhone {
	t.Helper()
	p := CloudPhone{UserID: uint(userID), Name: "st机", Status: status, CpID: cpID, ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	return &p
}

func insertTask(t *testing.T, p *CloudPhone, taskType string, deadline time.Time) {
	t.Helper()
	require.NoError(t, framework.DB.Create(&CpTask{
		UserID: p.UserID, CloudPhoneID: p.ID, CpID: p.CpID,
		Type: taskType, ExpectedState: MidplatReady, Status: TaskPending, Deadline: deadline,
	}).Error)
}

func statusOf(t *testing.T, id uint) string {
	t.Helper()
	var p CloudPhone
	require.NoError(t, framework.DB.First(&p, id).Error)
	return p.Status
}

// 创建：调中台受理 → 落 CREATING + cpId + 创建任务。
func TestCreateProvisionsViaMidplat(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units")
	})
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	withFakeOps(t, &fakePort{createCpID: "cp-123"})

	p, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "新机"})
	require.NoError(t, err)
	assert.Equal(t, StatusCreating, p.Status)
	assert.Equal(t, "cp-123", p.CpID)

	var tasks []CpTask
	require.NoError(t, framework.DB.Where("cloud_phone_id = ?", p.ID).Find(&tasks).Error)
	require.Len(t, tasks, 1)
	assert.Equal(t, TaskTypeCreate, tasks[0].Type)
	assert.Equal(t, TaskPending, tasks[0].Status)
}

// 无中台（降级）：仅落本地档案，直接 CREATED，不建任务。
func TestCreateDegradedWithoutMidplat(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units")
	})
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	withFakeOps(t, nil)

	p, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "本地机"})
	require.NoError(t, err)
	assert.Equal(t, StatusCreated, p.Status)
	assert.Empty(t, p.CpID)

	var count int64
	framework.DB.Model(&CpTask{}).Where("cloud_phone_id = ?", p.ID).Count(&count)
	assert.Zero(t, count)
}

// worker：needStart=false，创建完成（中台 STOPPED）收敛到 CREATED（不自动开机）。
func TestWorkerCreateSuccess(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatStopped}})

	p := insertPhone(t, userA, StatusCreating, "cp-x")
	insertTask(t, p, TaskTypeCreate, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	assert.Equal(t, StatusCreated, statusOf(t, p.ID))
}

// worker：创建任务超时（中台迟迟未 ONLINE）→ CREATE_FAILED。
func TestWorkerCreateTimeout(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{}}) // cp 未就绪

	p := insertPhone(t, userA, StatusCreating, "cp-x")
	insertTask(t, p, TaskTypeCreate, time.Now().Add(-time.Minute)) // 已过期

	PhoneService.runDueTasks(context.Background())
	assert.Equal(t, StatusCreateFailed, statusOf(t, p.ID))
}

// worker：开机任务在中台 ONLINE 后收敛到 RUNNING。
func TestWorkerStartSuccess(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatReady}})

	p := insertPhone(t, userA, StatusStarting, "cp-x")
	insertTask(t, p, TaskTypeStart, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	assert.Equal(t, StatusRunning, statusOf(t, p.ID))
}

// worker：开机时中台报销毁 → 回滚 STOPPED。
func TestWorkerStartDestroyed(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatDestroyed}})

	p := insertPhone(t, userA, StatusStarting, "cp-x")
	insertTask(t, p, TaskTypeStart, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	assert.Equal(t, StatusStopped, statusOf(t, p.ID))
}

// worker：关机任务在中台 STOPPED 后收敛到业务 STOPPED。
func TestWorkerStopSuccess(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatStopped}})

	p := insertPhone(t, userA, StatusStopping, "cp-x")
	insertTask(t, p, TaskTypeStop, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	assert.Equal(t, StatusStopped, statusOf(t, p.ID))
}

// 开机门禁：按中台实时态判（STOPPED 可开机 → STARTING；关机仅 NORMAL/运行中）。
func TestPowerGating(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_runtime_minute_wallets", "billing_ledger_entries")
	})
	// 开机前置校验（CanBoot）：给 userA 备一份临时时长，使开机门禁放行，专注测状态机门禁。
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(userA, 1000))
	// fake 中台实时态：开关机门禁现按此判（与 UI 显示同源）。
	f := &fakePort{statuses: map[string]string{
		"cp-a": "STOPPED",      // 已停止 → 可开机
		"cp-b": "NORMAL",       // 运行中 → 可关机
		"cp-c": "INITIALIZING", // 创建中 → 不可开机
		"cp-d": "NORMAL",       // 运行中 → 不可再开机
		"cp-e": "STOPPED",      // 已停止 → 不可关机
	}}
	withFakeOps(t, f)

	// 中台 STOPPED → 开机 → 本地置 STARTING
	a := insertPhone(t, userA, StatusStopped, "cp-a")
	require.NoError(t, PhoneService.Power(userA, int(a.ID), "开机"))
	assert.Equal(t, StatusStarting, statusOf(t, a.ID))
	assert.Equal(t, "power:开机", f.lastOp)

	// 中台 NORMAL → 关机 → 本地置 STOPPING
	b := insertPhone(t, userA, StatusRunning, "cp-b")
	require.NoError(t, PhoneService.Power(userA, int(b.ID), "关机"))
	assert.Equal(t, StatusStopping, statusOf(t, b.ID))
	assert.Equal(t, "power:关机", f.lastOp)

	// 非法过渡（按实时态）
	c := insertPhone(t, userA, StatusCreating, "cp-c")
	assert.Error(t, PhoneService.Power(userA, int(c.ID), "开机"), "中台创建中不可开机")
	d := insertPhone(t, userA, StatusRunning, "cp-d")
	assert.Error(t, PhoneService.Power(userA, int(d.ID), "开机"), "中台运行中不可再开机")
	e := insertPhone(t, userA, StatusStopped, "cp-e")
	assert.Error(t, PhoneService.Power(userA, int(e.ID), "关机"), "中台已停止不可关机")

	// 中台查不到（UNKNOWN）→ 拒绝
	u := insertPhone(t, userA, StatusRunning, "cp-unknown")
	assert.Error(t, PhoneService.Power(userA, int(u.ID), "关机"), "实时态未知应拒绝")
}

// 开机前置校验（CanBoot，新模型 §3.3）：无空闲包月名额且无临时时长 → 拒绝；
// 有临时时长 或 有包月开机数 → 放行。
func TestPowerRuntimeGate(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "run_sessions",
			"billing_license_units", "billing_runtime_minute_wallets", "billing_ledger_entries")
	})
	f := &fakePort{statuses: map[string]string{
		"cp-rt1": "STOPPED", "cp-rt2": "STOPPED",
	}}
	withFakeOps(t, f)

	// 用户1：无包月名额无临时时长 → 拒绝；发放临时时长后 → 放行。
	const u1 = 970201
	p1 := insertPhone(t, u1, StatusStopped, "cp-rt1")
	assert.Error(t, PhoneService.Power(u1, int(p1.ID), "开机"), "无名额无时长应拒绝开机")
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(u1, 60))
	require.NoError(t, PhoneService.Power(u1, int(p1.ID), "开机"))
	assert.Equal(t, StatusStarting, statusOf(t, p1.ID))

	// 用户2：仅有包月开机数（boot_slot）→ 放行。
	const u2 = 970202
	require.NoError(t, billing.GrantBootSlotLicensesForTest(u2, 1))
	p2 := insertPhone(t, u2, StatusStopped, "cp-rt2")
	require.NoError(t, PhoneService.Power(u2, int(p2.ID), "开机"), "有包月开机数应可开机")
	assert.Equal(t, StatusStarting, statusOf(t, p2.ID))
}

// 开机硬约束：必须已绑定代理（proxy_id>0）才能开机；未绑代理一律拒绝，且不触达中台。
// 绑定代理后即可开机。代理门禁排在 CanBoot 之前，这里备足时长以证明唯一拦截原因是「未绑代理」。
func TestPowerOnRequiresProxy(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_runtime_minute_wallets", "billing_ledger_entries", "proxies")
	})
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(userA, 1000))
	f := &fakePort{statuses: map[string]string{"cp-noproxy": "STOPPED"}} // 实时态可开机
	withFakeOps(t, f)

	// 未绑代理（proxy_id=0）的已开通云手机。
	p := CloudPhone{UserID: userA, Name: "未绑代理机", Status: StatusStopped, CpID: "cp-noproxy", ProxyID: 0}
	require.NoError(t, framework.DB.Create(&p).Error)

	err := PhoneService.Power(userA, int(p.ID), "开机")
	require.Error(t, err, "未绑代理应拒绝开机")
	assert.Contains(t, err.Error(), "代理")
	assert.Equal(t, 0, f.calls, "被代理门禁拦下，不应触达中台")

	// 绑定代理后可开机（I2 修复后 Update 绑代理需属主校验：先给 userA 建一条真实代理）。
	px, err := proxy.Create(userA, proxy.ProxyInput{Name: "px-boot", Host: "203.0.113.10", Port: 1080})
	require.NoError(t, err)
	_, err = PhoneService.Update(userA, int(p.ID), &CloudPhoneUpdate{ProxyID: uint(px.ID)})
	require.NoError(t, err)
	require.NoError(t, PhoneService.Power(userA, int(p.ID), "开机"))
	assert.Equal(t, StatusStarting, statusOf(t, p.ID))
}

// 销毁门禁（按中台实时态）：过渡/运行态不可销毁；INIT_FAILED/STOPPED 可销毁；UNKNOWN 拒绝。
func TestDestroyGating(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	f := &fakePort{statuses: map[string]string{
		"cp-INITIALIZING": "INITIALIZING",
		"cp-STARTING":     "STARTING",
		"cp-NORMAL":       "NORMAL",
		"cp-DESTROYING":   "DESTROYING",
		"cp-ok-failed":    "INIT_FAILED",
		"cp-ok-stopped":   "STOPPED",
	}}
	withFakeOps(t, f)

	for _, raw := range []string{"INITIALIZING", "STARTING", "NORMAL", "DESTROYING"} {
		p := insertPhone(t, userA, StatusRunning, "cp-"+raw)
		assert.Error(t, PhoneService.Delete(userA, int(p.ID)), raw+" 不应可销毁")
	}

	// 可销毁（中台 INIT_FAILED / STOPPED）：调中台 destroy + 置 DESTROYING + 建销毁任务（异步，不立即删本地）。
	for _, cp := range []string{"cp-ok-failed", "cp-ok-stopped"} {
		p := insertPhone(t, userA, StatusStopped, cp)
		require.NoError(t, PhoneService.Delete(userA, int(p.ID)), cp+" 应可销毁")
		assert.Equal(t, "destroy", f.lastOp, "应调用中台销毁")
		assert.Equal(t, StatusDestroying, statusOf(t, p.ID), "销毁后应进入 DESTROYING")
	}

	// 中台查不到（UNKNOWN）→ 拒绝。
	u := insertPhone(t, userA, StatusStopped, "cp-unknown")
	assert.Error(t, PhoneService.Delete(userA, int(u.ID)), "实时态未知应拒绝销毁")
}

// 销毁 worker：中台确认实例消失（查不到）后删本地档案。
func TestWorkerDestroyRemovesRecord(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{}}) // cp 已不在中台列表 = 已销毁

	p := insertPhone(t, userA, StatusDestroying, "cp-gone")
	insertTask(t, p, TaskTypeDestroy, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	_, err := PhoneService.GetByID(userA, int(p.ID))
	assert.Error(t, err, "中台确认消失后本地档案应被删除")
}

// 无中台实例（cp_id 空）直接删本地，不进 DESTROYING。
func TestDestroyWithoutCpIDDeletesNow(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{})

	p := insertPhone(t, userA, StatusCreateFailed, "") // 无 cpId
	require.NoError(t, PhoneService.Delete(userA, int(p.ID)))
	_, err := PhoneService.GetByID(userA, int(p.ID))
	assert.Error(t, err, "无中台实例应直接删本地")
}

// 展示纯实时：中台查不到/查询失败 → UNKNOWN；未开通(无 cpId) → 保留本地。
func TestResolveLiveStatusesUnknown(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })

	// 中台返回 cp-1=NORMAL；cp-2 不在返回里 → UNKNOWN。
	f := &fakePort{statuses: map[string]string{"cp-1": "NORMAL"}}
	withFakeOps(t, f)
	items := []CloudPhone{
		{CpID: "cp-1", Status: StatusStopped}, // 本地陈旧，应被实时 NORMAL→RUNNING 覆盖
		{CpID: "cp-2", Status: StatusRunning}, // 中台查不到 → UNKNOWN
		{CpID: "", Status: StatusCreated},     // 未开通 → 保留本地
	}
	PhoneService.resolveLiveStatuses(items)
	assert.Equal(t, StatusRunning, items[0].Status)
	assert.Equal(t, StatusUnknown, items[1].Status)
	assert.Equal(t, StatusCreated, items[2].Status)

	// 中台查询失败 → 有 cpId 的全部 UNKNOWN。
	fe := &fakePort{err: errors.New("中台炸了")}
	withFakeOps(t, fe)
	items2 := []CloudPhone{{CpID: "cp-9", Status: StatusRunning}}
	PhoneService.resolveLiveStatuses(items2)
	assert.Equal(t, StatusUnknown, items2[0].Status)
}
