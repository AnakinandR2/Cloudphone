package phone

import (
	"context"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertPhone 直接落一台指定状态/ cpId 的云手机。
func insertPhone(t *testing.T, userID int, status, cpID string) *CloudPhone {
	t.Helper()
	p := CloudPhone{UserID: uint(userID), Name: "st机", Status: status, CpID: cpID}
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
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{createCpID: "cp-123"})

	p, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "新机", Region: "上海"})
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
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, nil)

	p, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "本地机"})
	require.NoError(t, err)
	assert.Equal(t, StatusCreated, p.Status)
	assert.Empty(t, p.CpID)

	var count int64
	framework.DB.Model(&CpTask{}).Where("cloud_phone_id = ?", p.ID).Count(&count)
	assert.Zero(t, count)
}

// worker：创建任务在中台 ONLINE 后收敛到 CREATED。
func TestWorkerCreateSuccess(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatReady}})

	p := insertPhone(t, userA, StatusCreating, "cp-x")
	insertTask(t, p, TaskTypeCreate, time.Now().Add(time.Minute))

	PhoneService.runDueTasks(context.Background())
	// 中台 autoStart：创建完成（NORMAL）直接收敛到 RUNNING。
	assert.Equal(t, StatusRunning, statusOf(t, p.ID))
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

// 开机门禁：CREATED/STOPPED 可开机 → STARTING；其余拒绝。关机仅 RUNNING。
func TestPowerGating(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	f := &fakePort{}
	withFakeOps(t, f)

	// CREATED → 开机 → STARTING
	created := insertPhone(t, userA, StatusCreated, "cp-a")
	require.NoError(t, PhoneService.Power(userA, int(created.ID), "开机"))
	assert.Equal(t, StatusStarting, statusOf(t, created.ID))
	assert.Equal(t, "power:开机", f.lastOp)

	// RUNNING → 关机 → STOPPING（异步，worker 收敛到 STOPPED）
	running := insertPhone(t, userA, StatusRunning, "cp-b")
	require.NoError(t, PhoneService.Power(userA, int(running.ID), "关机"))
	assert.Equal(t, StatusStopping, statusOf(t, running.ID))
	assert.Equal(t, "power:关机", f.lastOp)

	// 非法过渡
	creating := insertPhone(t, userA, StatusCreating, "cp-c")
	assert.Error(t, PhoneService.Power(userA, int(creating.ID), "开机"), "CREATING 不可开机")
	run2 := insertPhone(t, userA, StatusRunning, "cp-d")
	assert.Error(t, PhoneService.Power(userA, int(run2.ID), "开机"), "RUNNING 不可再开机")
	created2 := insertPhone(t, userA, StatusCreated, "cp-e")
	assert.Error(t, PhoneService.Power(userA, int(created2.ID), "关机"), "CREATED 不可关机")
}

// 销毁门禁：CREATING/STARTING/RUNNING/DESTROYING 不可销毁；CREATE_FAILED/CREATED/STOPPED 可销毁。
func TestDestroyGating(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	f := &fakePort{}
	withFakeOps(t, f)

	for _, st := range []string{StatusCreating, StatusStarting, StatusRunning, StatusDestroying} {
		p := insertPhone(t, userA, st, "cp-"+st)
		assert.Error(t, PhoneService.Delete(userA, int(p.ID)), st+" 不应可销毁")
	}

	// 可销毁状态：调中台 destroy + 置 DESTROYING + 建销毁任务（异步，不立即删本地）。
	for _, st := range []string{StatusCreateFailed, StatusCreated, StatusStopped} {
		p := insertPhone(t, userA, st, "cp-ok-"+st)
		require.NoError(t, PhoneService.Delete(userA, int(p.ID)), st+" 应可销毁")
		assert.Equal(t, "destroy", f.lastOp, "应调用中台销毁")
		assert.Equal(t, StatusDestroying, statusOf(t, p.ID), "销毁后应进入 DESTROYING")
	}
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
