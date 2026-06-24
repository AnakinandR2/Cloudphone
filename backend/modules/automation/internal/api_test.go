package automation

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// callH 用给定属主 / method / body / 路径参数调用一个前台 handler，返回 envelope。
// userID<0 表示不预置 userID（模拟未授权）。
func callH(userID int, method, body string, params gin.Params, h gin.HandlerFunc) (int, json.RawMessage, int) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if userID >= 0 {
		c.Set("userID", userID)
	}
	if params != nil {
		c.Params = params
	}
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	c.Request = httptest.NewRequest(method, "/x", reader)
	c.Request.Header.Set("Content-Type", "application/json")
	h(c)
	var env struct {
		Code  int             `json:"code"`
		Data  json.RawMessage `json:"data"`
		Total int             `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Data, env.Code
}

func idParams(id uint) gin.Params {
	return gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
}

const userH = 8201

// ---- 未授权：所有前台 handler 都应 401 ----

func TestHandlersUnauthorized(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	cases := []struct {
		name string
		h    gin.HandlerFunc
		m    string
	}{
		{"ListScripts", ListScripts, "GET"},
		{"ListStoreScripts", ListStoreScripts, "GET"},
		{"ListUsableScripts", ListUsableScripts, "GET"},
		{"CreateScript", CreateScript, "POST"},
		{"UpdateScript", UpdateScript, "PUT"},
		{"ToggleScript", ToggleScript, "POST"},
		{"DeleteScript", DeleteScript, "DELETE"},
		{"ListPlans", ListPlans, "GET"},
		{"CreatePlan", CreatePlan, "POST"},
		{"RunTask", RunTask, "POST"},
		{"ListTasks", ListTasks, "GET"},
		{"TaskDetail", TaskDetail, "GET"},
		{"planStart", planActionHandler("start"), "POST"},
	}
	for _, tc := range cases {
		status, _, _ := callH(-1, tc.m, "", nil, tc.h)
		assert.Equal(t, 401, status, tc.name+" 未授权应 401")
	}
}

// ---- 脚本 handler ----

func TestCreateScriptHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)

	// 正常创建
	status, data, _ := callH(userH, "POST", `{"name":"s","luaContent":"log(1)","fileName":"a.lua"}`, nil, CreateScript)
	require.Equal(t, 200, status)
	var rec AutomationScript
	require.NoError(t, json.Unmarshal(data, &rec))
	assert.Equal(t, "s", rec.Name)
	assert.True(t, f.uploaded)

	// 非法 JSON → 400
	status, _, _ = callH(userH, "POST", `{bad`, nil, CreateScript)
	assert.Equal(t, 400, status)

	// 空内容 → service 拒绝（422 校验错）
	status, _, _ = callH(userH, "POST", `{"name":"x","luaContent":""}`, nil, CreateScript)
	assert.NotEqual(t, 200, status)
}

func TestListScriptHandlers(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "mine", LuaContent: "log(1)"})
	require.NoError(t, err)
	_, err = Service.AdminCreateStoreScript(ScriptInput{Name: "store", LuaContent: "log(2)"})
	require.NoError(t, err)

	var mine []AutomationScript
	status, data, _ := callH(userH, "GET", "", nil, ListScripts)
	require.Equal(t, 200, status)
	require.NoError(t, json.Unmarshal(data, &mine))
	assert.Len(t, mine, 1)
	assert.Equal(t, rec.ID, mine[0].ID)

	var store []AutomationScript
	status, data, _ = callH(userH, "GET", "", nil, ListStoreScripts)
	require.Equal(t, 200, status)
	require.NoError(t, json.Unmarshal(data, &store))
	assert.Len(t, store, 1)

	var usable []AutomationScript
	status, data, _ = callH(userH, "GET", "", nil, ListUsableScripts)
	require.Equal(t, 200, status)
	require.NoError(t, json.Unmarshal(data, &usable))
	assert.Len(t, usable, 2, "我的(启用) ∪ 商店(启用)")
}

func TestUpdateScriptHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "s", LuaContent: "log(1)"})
	require.NoError(t, err)

	// 非法 id
	status, _, _ := callH(userH, "PUT", `{"luaContent":"log(2)"}`, gin.Params{{Key: "id", Value: "abc"}}, UpdateScript)
	assert.Equal(t, 400, status)

	// 非法 JSON
	status, _, _ = callH(userH, "PUT", `{bad`, idParams(rec.ID), UpdateScript)
	assert.Equal(t, 400, status)

	// 正常更新
	f.scriptID = 300
	status, data, _ := callH(userH, "PUT", `{"name":"s2","luaContent":"log(2)"}`, idParams(rec.ID), UpdateScript)
	require.Equal(t, 200, status)
	var updated AutomationScript
	require.NoError(t, json.Unmarshal(data, &updated))
	assert.Equal(t, int64(300), updated.ScriptID)

	// 不属于本人 → 404
	status, _, _ = callH(99999, "PUT", `{"luaContent":"log(3)"}`, idParams(rec.ID), UpdateScript)
	assert.Equal(t, 404, status)
}

func TestToggleScriptHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "s", LuaContent: "log(1)"})
	require.NoError(t, err)

	status, _, _ := callH(userH, "POST", `{"enabled":false}`, idParams(rec.ID), ToggleScript)
	require.Equal(t, 200, status)
	got, err := Service.repo.ownedScript(userH, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, ScriptDisabled, got.Status)

	// 非法 id
	status, _, _ = callH(userH, "POST", `{"enabled":true}`, gin.Params{{Key: "id", Value: "0"}}, ToggleScript)
	assert.Equal(t, 400, status)
}

func TestDeleteScriptHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "s", LuaContent: "log(1)"})
	require.NoError(t, err)

	status, _, _ := callH(userH, "DELETE", "", idParams(rec.ID), DeleteScript)
	require.Equal(t, 200, status)
	_, err = Service.repo.ownedScript(userH, rec.ID)
	assert.Error(t, err)

	// 非法 id
	status, _, _ = callH(userH, "DELETE", "", gin.Params{{Key: "id", Value: "x"}}, DeleteScript)
	assert.Equal(t, 400, status)
}

// ---- 计划 handler ----

func TestCreatePlanHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-ph1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)

	body := `{"scriptId":` + strconv.FormatUint(uint64(rec.ID), 10) +
		`,"name":"每日","frequency":"DAILY","executionTime":"08:00:00","startTime":"2026-06-17T00:00:00","endTime":"2027-06-17T00:00:00","cpIds":["cp-ph1"]}`
	status, data, _ := callH(userH, "POST", body, nil, CreatePlan)
	require.Equal(t, 200, status)
	var plan AutomationPlan
	require.NoError(t, json.Unmarshal(data, &plan))
	assert.Equal(t, "每日", plan.Name)

	// 非法 JSON
	status, _, _ = callH(userH, "POST", `{bad`, nil, CreatePlan)
	assert.Equal(t, 400, status)

	// 非法频率 → service 拒绝
	status, _, _ = callH(userH, "POST", `{"frequency":"WEEKLY"}`, nil, CreatePlan)
	assert.NotEqual(t, 200, status)
}

func TestListPlansHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-lp1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)
	_, err = Service.CreatePlan(userH, PlanInput{
		ScriptLocalID: rec.ID, Name: "n", Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-17T00:00:00", EndTime: "2027-06-17T00:00:00", CpIDs: []string{"cp-lp1"},
	})
	require.NoError(t, err)

	var plans []AutomationPlan
	status, data, _ := callH(userH, "GET", "", nil, ListPlans)
	require.Equal(t, 200, status)
	require.NoError(t, json.Unmarshal(data, &plans))
	assert.Len(t, plans, 1)
}

func TestPlanActionHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-pa1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)
	plan, err := Service.CreatePlan(userH, PlanInput{
		ScriptLocalID: rec.ID, Name: "n", Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-17T00:00:00", EndTime: "2027-06-17T00:00:00", CpIDs: []string{"cp-pa1"},
	})
	require.NoError(t, err)

	// 非法 id
	status, _, _ := callH(userH, "POST", "", gin.Params{{Key: "id", Value: "x"}}, planActionHandler("start"))
	assert.Equal(t, 400, status)

	status, _, _ = callH(userH, "POST", "", idParams(plan.ID), planActionHandler("start"))
	require.Equal(t, 200, status)
	status, _, _ = callH(userH, "POST", "", idParams(plan.ID), planActionHandler("pause"))
	require.Equal(t, 200, status)
	status, _, _ = callH(userH, "DELETE", "", idParams(plan.ID), planActionHandler("delete"))
	require.Equal(t, 200, status)
	assert.Equal(t, []string{"start", "pause", "delete"}, f.planActions)

	// 不存在的计划 → 404
	status, _, _ = callH(userH, "POST", "", idParams(99999), planActionHandler("start"))
	assert.Equal(t, 404, status)
}

// ---- 任务 handler ----

func TestRunTaskHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-rt1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "r", LuaContent: "log(1)"})
	require.NoError(t, err)

	body := `{"scriptId":` + strconv.FormatUint(uint64(rec.ID), 10) + `,"cpIds":["cp-rt1"],"taskName":"t1"}`
	status, data, _ := callH(userH, "POST", body, nil, RunTask)
	require.Equal(t, 200, status)
	var rows []AutomationTask
	require.NoError(t, json.Unmarshal(data, &rows))
	assert.Len(t, rows, 1)

	// 非法 JSON
	status, _, _ = callH(userH, "POST", `{bad`, nil, RunTask)
	assert.Equal(t, 400, status)
}

func TestListTasksHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-lt1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "r", LuaContent: "log(1)"})
	require.NoError(t, err)
	_, err = Service.RunNow(userH, rec.ID, []string{"cp-lt1"}, "t", nil, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userH)
	c.Request = httptest.NewRequest("GET", "/x?page=1&size=20", nil)
	ListTasks(c)
	var env struct {
		Code int `json:"code"`
		Data struct {
			List  []AutomationTask `json:"list"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, 0, env.Code)
	assert.GreaterOrEqual(t, env.Data.Total, int64(1))
	assert.NotEmpty(t, env.Data.List)
}

func TestTaskDetailHandler(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userH, "cp-td1")
	rec, err := Service.CreateUserScript(userH, ScriptInput{Name: "r", LuaContent: "log(1)"})
	require.NoError(t, err)
	rows, err := Service.RunNow(userH, rec.ID, []string{"cp-td1"}, "t", nil, nil)
	require.NoError(t, err)

	mid := strconv.FormatInt(rows[0].MidTaskID, 10)
	status, data, _ := callH(userH, "GET", "", gin.Params{{Key: "midTaskId", Value: mid}}, TaskDetail)
	require.Equal(t, 200, status)
	var detail TaskReport
	require.NoError(t, json.Unmarshal(data, &detail))
	assert.Equal(t, "COMPLETED", detail.Status)

	// 非法 midTaskId
	status, _, _ = callH(userH, "GET", "", gin.Params{{Key: "midTaskId", Value: "abc"}}, TaskDetail)
	assert.Equal(t, 400, status)

	// 不属于本人 → 404
	status, _, _ = callH(99999, "GET", "", gin.Params{{Key: "midTaskId", Value: mid}}, TaskDetail)
	assert.Equal(t, 404, status)
}
