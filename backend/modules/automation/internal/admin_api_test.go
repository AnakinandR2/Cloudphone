package automation

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userAdminClient = 8301

// callAdmin 调一个 admin handler（不读 userID），返回 envelope.code + data。
func callAdmin(method, body string, params gin.Params, h gin.HandlerFunc) (int, json.RawMessage) {
	_, data, code := callH(0, method, body, params, h)
	return code, data
}

func TestAdminStoreCRUD(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)

	// Create：非法 JSON
	code, _ := callAdmin("POST", `{bad`, nil, AdminStoreCreate)
	assert.Equal(t, 400, code)

	// Create 正常
	code, data := callAdmin("POST", `{"name":"商店脚本","luaContent":"log(1)"}`, nil, AdminStoreCreate)
	require.Equal(t, 0, code)
	var rec AutomationScript
	require.NoError(t, json.Unmarshal(data, &rec))
	assert.True(t, rec.Store)
	assert.Equal(t, uint(0), rec.UserID)

	// List
	code, data = callAdmin("GET", "", nil, AdminStoreList)
	require.Equal(t, 0, code)
	var list []AutomationScript
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list, 1)

	// Update：非法 id
	code, _ = callAdmin("PUT", `{"luaContent":"log(2)"}`, gin.Params{{Key: "id", Value: "0"}}, AdminStoreUpdate)
	assert.Equal(t, 400, code)
	// Update：非法 JSON
	code, _ = callAdmin("PUT", `{bad`, idParams(rec.ID), AdminStoreUpdate)
	assert.Equal(t, 400, code)
	// Update 正常
	f.scriptID = 400
	code, data = callAdmin("PUT", `{"name":"改名","luaContent":"log(2)"}`, idParams(rec.ID), AdminStoreUpdate)
	require.Equal(t, 0, code)
	var upd AutomationScript
	require.NoError(t, json.Unmarshal(data, &upd))
	assert.Equal(t, int64(400), upd.ScriptID)

	// Toggle：非法 id
	code, _ = callAdmin("POST", `{"enabled":false}`, gin.Params{{Key: "id", Value: "x"}}, AdminStoreToggle)
	assert.Equal(t, 400, code)
	// Toggle 正常
	code, _ = callAdmin("POST", `{"enabled":false}`, idParams(rec.ID), AdminStoreToggle)
	require.Equal(t, 0, code)
	got, err := Service.repo.scriptByID(rec.ID)
	require.NoError(t, err)
	assert.Equal(t, ScriptDisabled, got.Status)

	// Delete：非法 id
	code, _ = callAdmin("DELETE", "", gin.Params{{Key: "id", Value: "x"}}, AdminStoreDelete)
	assert.Equal(t, 400, code)
	// Delete 正常
	code, _ = callAdmin("DELETE", "", idParams(rec.ID), AdminStoreDelete)
	require.Equal(t, 0, code)
	_, err = Service.repo.scriptByID(rec.ID)
	assert.Error(t, err)
}

func TestAdminStoreUpdateNotStore(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	// 用户脚本（store=false）不能被 admin store 接口操作 → 404
	rec, err := Service.CreateUserScript(userAdminClient, ScriptInput{Name: "u", LuaContent: "log(1)"})
	require.NoError(t, err)

	code, _ := callAdmin("PUT", `{"luaContent":"log(2)"}`, idParams(rec.ID), AdminStoreUpdate)
	assert.Equal(t, 404, code)
	code, _ = callAdmin("POST", `{"enabled":true}`, idParams(rec.ID), AdminStoreToggle)
	assert.Equal(t, 404, code)
	code, _ = callAdmin("DELETE", "", idParams(rec.ID), AdminStoreDelete)
	assert.Equal(t, 404, code)
}

func TestAdminUserScriptGovernance(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	rec, err := Service.CreateUserScript(userAdminClient, ScriptInput{Name: "u", LuaContent: "log(1)"})
	require.NoError(t, err)
	// 还造一个商店脚本，确认它不出现在用户治理列表
	_, err = Service.AdminCreateStoreScript(ScriptInput{Name: "s", LuaContent: "log(2)"})
	require.NoError(t, err)

	// List
	code, data := callAdmin("GET", "", nil, AdminUserScriptList)
	require.Equal(t, 0, code)
	var list []AdminScript
	require.NoError(t, json.Unmarshal(data, &list))
	require.Len(t, list, 1)
	assert.Equal(t, uint(userAdminClient), list[0].UploaderID)

	// Toggle：非法 id
	code, _ = callAdmin("POST", `{"enabled":false}`, gin.Params{{Key: "id", Value: "x"}}, AdminUserScriptToggle)
	assert.Equal(t, 400, code)
	// Toggle 正常（下架）
	code, _ = callAdmin("POST", `{"enabled":false}`, idParams(rec.ID), AdminUserScriptToggle)
	require.Equal(t, 0, code)
	got, err := Service.repo.scriptByID(rec.ID)
	require.NoError(t, err)
	assert.Equal(t, ScriptDisabled, got.Status)

	// 商店脚本不能被用户治理接口操作 → 404
	store, err := Service.repo.listStoreScripts()
	require.NoError(t, err)
	require.NotEmpty(t, store)
	code, _ = callAdmin("POST", `{"enabled":false}`, idParams(store[0].ID), AdminUserScriptToggle)
	assert.Equal(t, 404, code)
	code, _ = callAdmin("DELETE", "", idParams(store[0].ID), AdminUserScriptDelete)
	assert.Equal(t, 404, code)

	// Delete：非法 id
	code, _ = callAdmin("DELETE", "", gin.Params{{Key: "id", Value: "x"}}, AdminUserScriptDelete)
	assert.Equal(t, 400, code)
	// Delete 正常
	code, _ = callAdmin("DELETE", "", idParams(rec.ID), AdminUserScriptDelete)
	require.Equal(t, 0, code)
	_, err = Service.repo.scriptByID(rec.ID)
	assert.Error(t, err)
}
