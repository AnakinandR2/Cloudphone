package automation

import (
	"context"
	"testing"

	"manager-backend/framework/midplat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userRW = 8501

// ---- repository 直测 ----

func TestRepoActivePlansFilter(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo

	// 活跃：plan_uid 非空 且 status != FINISHED
	require.NoError(t, repo.createPlan(&AutomationPlan{UserID: userRW, PlanUID: "u-active", Status: PlanEnabling}))
	// FINISHED → 排除
	require.NoError(t, repo.createPlan(&AutomationPlan{UserID: userRW, PlanUID: "u-fin", Status: PlanFinished}))
	// plan_uid 空 → 排除
	require.NoError(t, repo.createPlan(&AutomationPlan{UserID: userRW, PlanUID: "", Status: PlanEnabling}))

	active, err := repo.activePlans()
	require.NoError(t, err)
	uids := map[string]bool{}
	for _, p := range active {
		uids[p.PlanUID] = true
	}
	assert.True(t, uids["u-active"])
	assert.False(t, uids["u-fin"])
	assert.False(t, uids[""])
}

func TestRepoUpsertTaskByMidID(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo

	// insert 分支
	require.NoError(t, repo.upsertTaskByMidID(&AutomationTask{
		UserID: userRW, MidTaskID: 700001, TaskNo: "T1", LastStatus: "WAITING_PUBLISH", CpID: "cp1",
	}))
	got, err := repo.taskByMidID(userRW, 700001)
	require.NoError(t, err)
	assert.Equal(t, "WAITING_PUBLISH", got.LastStatus)

	// update 分支：同 mid_task_id 刷新可变字段
	require.NoError(t, repo.upsertTaskByMidID(&AutomationTask{
		UserID: userRW, MidTaskID: 700001, TaskNo: "T1b", LastStatus: "COMPLETED", RunStart: "s", RunEnd: "e",
	}))
	got, err = repo.taskByMidID(userRW, 700001)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", got.LastStatus)
	assert.Equal(t, "T1b", got.TaskNo)
	assert.Equal(t, "s", got.RunStart)
}

func TestRepoNonTerminalTasks(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo

	require.NoError(t, repo.insertTasks([]AutomationTask{
		{UserID: userRW, MidTaskID: 710001, LastStatus: "RUNNING"},
		{UserID: userRW, MidTaskID: 710002, LastStatus: "COMPLETED"},
		{UserID: userRW, MidTaskID: 710003, LastStatus: "WAITING_PUBLISH"},
	}))
	tasks, err := repo.nonTerminalTasks()
	require.NoError(t, err)
	ids := map[int64]bool{}
	for _, x := range tasks {
		ids[x.MidTaskID] = true
	}
	assert.True(t, ids[710001])
	assert.False(t, ids[710002], "终态应排除")
	assert.True(t, ids[710003])
}

func TestRepoInsertTasksEmpty(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	require.NoError(t, Service.repo.insertTasks(nil))
}

func TestRepoUpdateTaskStatus(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo
	require.NoError(t, repo.insertTasks([]AutomationTask{
		{UserID: userRW, MidTaskID: 720001, LastStatus: "RUNNING"},
	}))
	require.NoError(t, repo.updateTaskStatus(720001, "COMPLETED", "rs", "re"))
	got, err := repo.taskByMidID(userRW, 720001)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", got.LastStatus)
	assert.Equal(t, "rs", got.RunStart)
	assert.Equal(t, "re", got.RunEnd)
}

func TestRepoListUserTasksStatusFilter(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo
	require.NoError(t, repo.insertTasks([]AutomationTask{
		{UserID: userRW, MidTaskID: 730001, LastStatus: "RUNNING"},
		{UserID: userRW, MidTaskID: 730002, LastStatus: "COMPLETED"},
	}))
	rows, total, err := repo.listUserTasks(userRW, 0, 20, "COMPLETED")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(730002), rows[0].MidTaskID)
}

func TestRepoPlanByIDAndUpdateStatus(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	repo := Service.repo
	p := &AutomationPlan{UserID: userRW, PlanID: 9, PlanUID: "uu", Status: PlanNotStarted}
	require.NoError(t, repo.createPlan(p))
	got, err := repo.planByID(p.ID)
	require.NoError(t, err)
	assert.Equal(t, PlanNotStarted, got.Status)
	require.NoError(t, repo.updatePlanStatus(p.ID, PlanPaused))
	got, err = repo.planByID(p.ID)
	require.NoError(t, err)
	assert.Equal(t, PlanPaused, got.Status)
}

// ---- worker：syncTasks / discoverPlanTasks / refreshTaskStatuses ----

// fakeWorkerOps 让 TasksByPlan 返回派生任务，TaskStatuses 返回终态。
type fakeWorkerOps struct {
	fakeOps
	planTasks map[string][]midplat.ScriptTaskVO
}

func (f *fakeWorkerOps) TasksByPlan(_ context.Context, planUID string) ([]midplat.ScriptTaskVO, error) {
	return f.planTasks[planUID], nil
}
func (f *fakeWorkerOps) TaskStatuses(_ context.Context, ids []int64) ([]midplat.ScriptTaskVO, error) {
	out := make([]midplat.ScriptTaskVO, 0, len(ids))
	for _, id := range ids {
		out = append(out, midplat.ScriptTaskVO{ID: id, TaskStatus: "COMPLETED", RunStartTime: "rs", RunEndTime: "re"})
	}
	return out, nil
}

func TestSyncTasksDiscoverAndRefresh(t *testing.T) {
	f := &fakeWorkerOps{
		planTasks: map[string][]midplat.ScriptTaskVO{
			"plan-uid-w": {
				{ID: 740001, TaskID: "TW1", CpID: "cpw1", TaskName: "派生", TaskStatus: "RUNNING"},
			},
		},
	}
	withFakeOps(t, f)

	// 活跃计划
	require.NoError(t, Service.repo.createPlan(&AutomationPlan{
		UserID: userRW, PlanID: 1, PlanUID: "plan-uid-w", ScriptLocalID: 1,
		ScriptName: "脚本", Status: PlanEnabling,
	}))

	Service.syncTasks(context.Background())

	// discoverPlanTasks upsert 了派生任务，refreshTaskStatuses 把它刷成 COMPLETED
	got, err := Service.repo.taskByMidID(userRW, 740001)
	require.NoError(t, err)
	assert.Equal(t, TriggerPlan, got.Trigger)
	assert.Equal(t, "COMPLETED", got.LastStatus)
	assert.Equal(t, "rs", got.RunStart)
}

func TestSyncTasksNilOps(t *testing.T) {
	prev := Service.ops
	Service.ops = nil
	t.Cleanup(func() { Service.ops = prev })
	// 不应 panic
	Service.syncTasks(context.Background())
}
