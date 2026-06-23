package automation

import (
	"context"
	"errors"
	"testing"

	"manager-backend/framework/midplat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userE = 8401

// ---- 工具函数 ----

func TestParseStrList(t *testing.T) {
	assert.Equal(t, []string{}, parseStrList(""))
	assert.Equal(t, []string{}, parseStrList("{not json"))
	assert.Equal(t, []string{"a", "b"}, parseStrList(`["a","b"]`))
}

func TestMarshalStrList(t *testing.T) {
	assert.Equal(t, `["x","y"]`, marshalStrList([]string{"x", "y"}))
	assert.Equal(t, `[]`, marshalStrList([]string{}))
}

func TestBumpVersion(t *testing.T) {
	assert.Equal(t, "1.0.1", bumpVersion("1.0.0"))
	assert.Equal(t, "2.3.5", bumpVersion("2.3.4"))
	assert.Equal(t, "abc.1", bumpVersion("abc"))     // 非标准格式回退追加
	assert.Equal(t, "1.0.x.1", bumpVersion("1.0.x")) // 末位非数字回退
}

func TestExtractScriptResult(t *testing.T) {
	assert.Equal(t, "", extractScriptResult("no marker here"))
	// 双标记
	assert.Equal(t, `{"ok":1}`, extractScriptResult(`log #RESULT#{"ok":1}#RESULT# tail`))
	// 仅起始标记 → 取后续全部
	assert.Equal(t, "rest content", extractScriptResult("pre #RESULT# rest content"))
}

func TestRenderScriptLog(t *testing.T) {
	// 空列表 → 回退原始 runLog
	rep := &midplat.ScriptTaskReport{RunLog: "raw log"}
	assert.Equal(t, "raw log", renderScriptLog(rep))

	// 有 runLogList → 渲染时间/级别/内容
	rep2 := &midplat.ScriptTaskReport{
		RunLogList: []midplat.ScriptTaskReportLog{
			{Timestamp: "2026-01-01", Level: "INFO", Content: "hello"},
			{Content: "bare"},
		},
	}
	out := renderScriptLog(rep2)
	assert.Contains(t, out, "2026-01-01 [INFO] hello")
	assert.Contains(t, out, "bare")
}

func TestUniqueMidName(t *testing.T) {
	a := uniqueMidName(5)
	b := uniqueMidName(5)
	assert.NotEqual(t, a, b, "含纳秒应唯一")
	assert.Contains(t, a, "glory-5-")
}

func TestBeijingNow(t *testing.T) {
	s := beijingNow()
	assert.Len(t, s, len("2006-01-02T15:04:05"))
}

// ---- requireOps：未配置中台 ----

func TestRequireOpsUnconfigured(t *testing.T) {
	// 临时把 ops 置 nil 模拟未配置
	prev := Service.ops
	Service.ops = nil
	t.Cleanup(func() { Service.ops = prev })

	_, err := Service.CreateUserScript(userE, ScriptInput{Name: "x", LuaContent: "log(1)"})
	require.Error(t, err)
	_, err = Service.CreatePlan(userE, PlanInput{Frequency: "INTERVAL", IntervalValue: 1})
	require.Error(t, err)
	_, err = Service.RunNow(userE, 1, []string{"cp"}, "")
	require.Error(t, err)
	_, err = Service.TaskDetail(userE, 1)
	require.Error(t, err)
	err = Service.StartPlan(userE, 1)
	require.Error(t, err)
}

// ---- toggle / delete best-effort ----

func TestDeleteUserScriptBestEffort(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userE, ScriptInput{Name: "d", LuaContent: "log(1)"})
	require.NoError(t, err)
	scriptID := rec.ScriptID
	require.True(t, scriptID > 0)

	require.NoError(t, Service.DeleteUserScript(userE, rec.ID))
	assert.Contains(t, f.deletedTpl, scriptID, "应 best-effort 删中台模板")

	// 不属于本人 → NotFound
	rec2, _ := Service.CreateUserScript(userE, ScriptInput{Name: "d2", LuaContent: "log(1)"})
	err = Service.DeleteUserScript(99999, rec2.ID)
	require.Error(t, err)
}

func TestToggleUserScriptNotFound(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	err := Service.ToggleUserScript(userE, 999999, true)
	require.Error(t, err)
}

// fakeOpsErr 让中台调用全部失败，验证错误传播。
type fakeOpsErr struct{ fakeOps }

func (f *fakeOpsErr) ToggleTemplate(_ context.Context, _ int64, _ bool) error {
	return errors.New("boom")
}

func TestToggleScriptMidplatError(t *testing.T) {
	f := &fakeOpsErr{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userE, ScriptInput{Name: "t", LuaContent: "log(1)"})
	require.NoError(t, err)
	err = Service.ToggleUserScript(userE, rec.ID, false)
	require.Error(t, err, "ToggleTemplate 失败应传播错误")
}

// ---- admin 变体错误路径 ----

func TestAdminStoreScriptNotFound(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	_, err := Service.AdminUpdateStoreScript(999999, ScriptInput{LuaContent: "log(1)"})
	require.Error(t, err)
	err = Service.AdminToggleStoreScript(999999, true)
	require.Error(t, err)
	err = Service.AdminDeleteStoreScript(999999)
	require.Error(t, err)
}

func TestAdminUserScriptNotFound(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	err := Service.AdminToggleUserScript(999999, true)
	require.Error(t, err)
	err = Service.AdminDeleteUserScript(999999)
	require.Error(t, err)
}

func TestAdminUpdateAndDeleteStore(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.AdminCreateStoreScript(ScriptInput{Name: "s", LuaContent: "log(1)"})
	require.NoError(t, err)

	f.scriptID = 700
	upd, err := Service.AdminUpdateStoreScript(rec.ID, ScriptInput{Name: "s2", LuaContent: "log(2)"})
	require.NoError(t, err)
	assert.Equal(t, int64(700), upd.ScriptID)

	require.NoError(t, Service.AdminToggleStoreScript(rec.ID, false))
	require.NoError(t, Service.AdminDeleteStoreScript(rec.ID))
}

// ---- CreatePlan 校验分支 ----

func TestCreatePlanValidation(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userE, "cp-v1")
	rec, err := Service.CreateUserScript(userE, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)

	// INTERVAL 间隔 <=0
	_, err = Service.CreatePlan(userE, PlanInput{
		ScriptLocalID: rec.ID, Frequency: "INTERVAL", IntervalValue: 0, CpIDs: []string{"cp-v1"},
	})
	require.Error(t, err)

	// 脚本不存在
	_, err = Service.CreatePlan(userE, PlanInput{
		ScriptLocalID: 999999, Frequency: "INTERVAL", IntervalValue: 5, CpIDs: []string{"cp-v1"},
	})
	require.Error(t, err)

	// 没有有效 cpId（全是别人的）
	_, err = Service.CreatePlan(userE, PlanInput{
		ScriptLocalID: rec.ID, Frequency: "INTERVAL", IntervalValue: 5, CpIDs: []string{"cp-foreign"},
	})
	require.Error(t, err)

	// 缺 startTime/endTime
	_, err = Service.CreatePlan(userE, PlanInput{
		ScriptLocalID: rec.ID, Frequency: "INTERVAL", IntervalValue: 5, CpIDs: []string{"cp-v1"},
	})
	require.Error(t, err)
}

func TestRunNowScriptNotReady(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userE, "cp-nr1")
	// 直接造一条 scriptId=0 的脚本（未就绪）
	rec := &AutomationScript{UserID: userE, Name: "nr", Status: ScriptEnabled, ScriptID: 0}
	require.NoError(t, Service.repo.createScript(rec))

	_, err := Service.RunNow(userE, rec.ID, []string{"cp-nr1"}, "")
	require.Error(t, err, "scriptId=0 应拒绝")

	_, err = Service.CreatePlan(userE, PlanInput{
		ScriptLocalID: rec.ID, Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-17T00:00:00", EndTime: "2027-06-17T00:00:00", CpIDs: []string{"cp-nr1"},
	})
	require.Error(t, err, "scriptId=0 计划应拒绝")
}
