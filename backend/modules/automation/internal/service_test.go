package automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userA = 7001

// 新建脚本：上传中台 + 取回 scriptId 回写；属主隔离。
func TestCreateUserScriptUploadsAndBinds(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)

	rec, err := Service.CreateUserScript(userA, ScriptInput{
		Name: "签到", Description: "每日签到", LuaContent: "log(\"hi\")", FileName: "a.lua",
	})
	require.NoError(t, err)
	assert.True(t, f.uploaded, "应上传到中台")
	assert.Equal(t, int64(100), rec.ScriptID, "应回写取回的 scriptId")
	assert.False(t, rec.Store)
	assert.Equal(t, ScriptEnabled, rec.Status)

	mine, err := Service.ListUserScripts(userA)
	require.NoError(t, err)
	require.NotEmpty(t, mine)
	assert.Equal(t, "签到", mine[0].Name)

	// 空内容拒绝
	_, err = Service.CreateUserScript(userA, ScriptInput{Name: "x", LuaContent: "  "})
	require.Error(t, err)
}

// 编辑脚本：重传得新 scriptId，旧模板被删。
func TestUpdateUserScriptRebindsAndDeletesOld(t *testing.T) {
	f := &fakeOps{scriptID: 0}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userA, ScriptInput{Name: "s", LuaContent: "log(1)"})
	require.NoError(t, err)
	oldID := rec.ScriptID

	f.scriptID = 200 // 下次 TemplateScriptID 返回新 id
	updated, err := Service.UpdateUserScript(userA, rec.ID, ScriptInput{Name: "s2", LuaContent: "log(2)"})
	require.NoError(t, err)
	assert.Equal(t, int64(200), updated.ScriptID)
	assert.Contains(t, f.deletedTpl, oldID, "应删除旧模板")
}

// 一次性运行：仅本人拥有的 cpId 透传，未拥有的过滤掉，落任务索引。
func TestRunNowFiltersOwnership(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userA, "cp-own1")
	seedPhone(t, userA, "cp-own2")

	rec, err := Service.CreateUserScript(userA, ScriptInput{Name: "r", LuaContent: "log(1)"})
	require.NoError(t, err)

	rows, err := Service.RunNow(userA, rec.ID, []string{"cp-own1", "cp-foreign", "cp-own2"}, "测试")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"cp-own1", "cp-own2"}, f.lastTaskCps, "只应透传本人拥有的 cpId")
	assert.Len(t, rows, 2)

	// 任务日志含这两条
	logs, total, err := Service.LogList(userA, 1, 20, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	assert.NotEmpty(t, logs)

	// 全是别人的机器 → 拒绝
	_, err = Service.RunNow(userA, rec.ID, []string{"cp-foreign"}, "")
	require.Error(t, err)
}

// 任务详情：终态合并报告、抽取 #RESULT#。
func TestTaskDetailMergesReport(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userA, "cp-d1")
	rec, err := Service.CreateUserScript(userA, ScriptInput{Name: "d", LuaContent: "log(1)"})
	require.NoError(t, err)
	rows, err := Service.RunNow(userA, rec.ID, []string{"cp-d1"}, "")
	require.NoError(t, err)

	detail, err := Service.TaskDetail(userA, rows[0].MidTaskID)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", detail.Status)
	assert.True(t, detail.Terminal)
	assert.Equal(t, `{"ok":true}`, detail.Result)
	assert.Equal(t, "https://oss/x.png", detail.ScreenshotURL)
	assert.Equal(t, int64(1234), detail.RunDurationMs)
}

// 周期计划：频率映射 + 启停删 + 归属校验。
func TestCreatePlanMapsFrequencyAndActions(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userA, "cp-p1")
	rec, err := Service.CreateUserScript(userA, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)

	plan, err := Service.CreatePlan(userA, PlanInput{
		ScriptLocalID: rec.ID, Name: "每5分钟", Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-17T00:00:00", EndTime: "2027-06-17T00:00:00",
		CpIDs: []string{"cp-p1", "cp-foreign"},
	})
	require.NoError(t, err)
	assert.Equal(t, "INTERVAL", f.lastPlan.ExecutionFrequency)
	assert.Equal(t, 5, f.lastPlan.IntervalValue)
	assert.Equal(t, []string{"cp-p1"}, f.lastPlan.CpIDList, "只下发本人机器")
	assert.Equal(t, PlanNotStarted, plan.Status)

	require.NoError(t, Service.StartPlan(userA, plan.ID))
	require.NoError(t, Service.PausePlan(userA, plan.ID))
	require.NoError(t, Service.DeletePlan(userA, plan.ID))
	assert.Equal(t, []string{"start", "pause", "delete"}, f.planActions)

	// DAILY 缺执行时间 → 拒绝
	_, err = Service.CreatePlan(userA, PlanInput{
		ScriptLocalID: rec.ID, Name: "daily", Frequency: "DAILY", CpIDs: []string{"cp-p1"},
	})
	require.Error(t, err)
}
