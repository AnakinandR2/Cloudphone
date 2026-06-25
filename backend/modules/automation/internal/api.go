package automation

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录前台用户 ID（user 鉴权中间件写入）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

func paramUint(c *gin.Context, key string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}

// scriptBody 是新建/编辑脚本的请求体。参数 schema 写在 luaContent 顶部注释里。
type scriptBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LuaContent  string `json:"luaContent"`
	FileName    string `json:"fileName"`
}

func (b scriptBody) toInput() ScriptInput {
	return ScriptInput{Name: b.Name, Description: b.Description, LuaContent: b.LuaContent, FileName: b.FileName}
}

// ---- 脚本 ----

func ListScripts(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListUserScripts(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func ListStoreScripts(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListStoreScripts()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func ListUsableScripts(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListUsableScripts(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func CreateScript(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var b scriptBody
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rec, err := Service.CreateUserScript(uid, b.toInput())
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

func UpdateScript(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	var b scriptBody
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rec, err := Service.UpdateUserScript(uid, id, b.toInput())
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

func ToggleScript(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	var b struct {
		Enabled bool `json:"enabled"`
	}
	_ = c.ShouldBindJSON(&b)
	if err := Service.ToggleUserScript(uid, id, b.Enabled); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

func DeleteScript(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	if err := Service.DeleteUserScript(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// ---- 计划 ----

func ListPlans(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListPlans(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func CreatePlan(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var b struct {
		ScriptID      uint           `json:"scriptId"`
		Name          string         `json:"name"`
		Frequency     string         `json:"frequency"`
		IntervalValue int            `json:"intervalValue"`
		ExecutionTime string         `json:"executionTime"`
		StartTime     string         `json:"startTime"`
		EndTime       string         `json:"endTime"`
		CpIDs         []string       `json:"cpIds"`
		Params        map[string]any `json:"params"`
	}
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rec, err := Service.CreatePlan(uid, PlanInput{
		ScriptLocalID: b.ScriptID, Name: b.Name, Frequency: b.Frequency,
		IntervalValue: b.IntervalValue, ExecutionTime: b.ExecutionTime,
		StartTime: b.StartTime, EndTime: b.EndTime, CpIDs: b.CpIDs, Params: b.Params,
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

func planActionHandler(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := currentUserID(c)
		if !ok {
			framework.Fail(c, http.StatusUnauthorized, "未授权")
			return
		}
		id, ok := paramUint(c, "id")
		if !ok {
			framework.Fail(c, http.StatusBadRequest, "ID 非法")
			return
		}
		var err error
		switch action {
		case "start":
			err = Service.StartPlan(uid, id)
		case "pause":
			err = Service.PausePlan(uid, id)
		case "delete":
			err = Service.DeletePlan(uid, id)
		}
		if err != nil {
			framework.FailErr(c, err)
			return
		}
		framework.OK(c)
	}
}

// ---- 任务 ----

func RunTask(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var b struct {
		ScriptID       uint                      `json:"scriptId"`
		CpIDs          []string                  `json:"cpIds"`
		TaskName       string                    `json:"taskName"`
		Params         map[string]any            `json:"params"`
		PerPhoneParams map[string]map[string]any `json:"perPhoneParams"`
	}
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rows, err := Service.RunNow(uid, b.ScriptID, b.CpIDs, b.TaskName, b.Params, b.PerPhoneParams)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rows)
}

func ListTasks(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	rows, total, err := Service.LogList(uid, page, size, c.Query("status"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, rows, total)
}

func TaskDetail(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	midID, err := strconv.ParseInt(c.Param("midTaskId"), 10, 64)
	if err != nil || midID <= 0 {
		framework.Fail(c, http.StatusBadRequest, "任务 ID 非法")
		return
	}
	detail, err := Service.TaskDetail(uid, midID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, detail)
}
