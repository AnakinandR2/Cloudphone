package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"manager-backend/framework"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	// 建所有已注册模块的表（工具委托 phone/app/automation/proxy，需其表与服务装配）。
	if err := framework.RunSetup(framework.DB); err != nil {
		panic(err)
	}
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

const userA = 9101

// handlerByName 从工具表里取某工具的处理器。
func handlerByName(t *testing.T, name string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	t.Helper()
	for _, rt := range allTools() {
		if rt.tool.Name == name {
			return rt.handler
		}
	}
	t.Fatalf("工具 %s 未注册", name)
	return nil
}

// call 以指定属主 + 入参调用工具，返回结果与解包到 out 的 data。
func call(t *testing.T, uid int, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()
	if uid > 0 {
		ctx = context.WithValue(ctx, userIDKey, uid)
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := handlerByName(t, name)(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	if out != nil && !res.IsError {
		text := res.Content[0].(mcp.TextContent).Text
		require.NoError(t, json.Unmarshal([]byte(text), out))
	}
	return res
}

// 全部 18 个工具都注册了。
func TestAllToolsRegistered(t *testing.T) {
	assert.Len(t, allTools(), 18)
}

// 未鉴权（context 无 userID）→ 工具返回 isError。
func TestUnauthenticatedRejected(t *testing.T) {
	res := call(t, 0, "list_phones", map[string]any{}, nil)
	assert.True(t, res.IsError, "无密钥应被拒")
}

// 代理闭环：create → list → update → bind 到云手机 → delete。
func TestProxyToolsClosure(t *testing.T) {
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM proxies")
		framework.DB.Exec("DELETE FROM cloud_phones")
	})

	var created proxyView
	res := call(t, userA, "create_proxy", map[string]any{
		"name": "px1", "protocol": "socks5", "host": "1.2.3.4", "port": 1080,
	}, &created)
	require.False(t, res.IsError)
	require.NotZero(t, created.ID)
	assert.Equal(t, "1.2.3.4", created.Host)

	var list []proxyView
	call(t, userA, "list_proxies", map[string]any{}, &list)
	require.Len(t, list, 1)

	var updated proxyView
	call(t, userA, "update_proxy", map[string]any{
		"id": created.ID, "host": "5.6.7.8",
	}, &updated)
	assert.Equal(t, "5.6.7.8", updated.Host)

	// 绑定到一台本人云手机：proxy_id 落到该机。
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO cloud_phones (user_id, cp_id, name, status, proxy_id) VALUES (?,?,?,?,0)",
		userA, "cp-mcp", "m", "STOPPED").Error)
	var phoneID int
	require.NoError(t, framework.DB.Raw("SELECT id FROM cloud_phones WHERE cp_id='cp-mcp'").Scan(&phoneID).Error)
	res = call(t, userA, "bind_proxy", map[string]any{"id": phoneID, "proxyId": created.ID}, nil)
	require.False(t, res.IsError)
	var bound int
	framework.DB.Raw("SELECT proxy_id FROM cloud_phones WHERE id=?", phoneID).Scan(&bound)
	assert.Equal(t, created.ID, bound, "proxy 应绑定到该云手机")

	res = call(t, userA, "delete_proxy", map[string]any{"id": created.ID}, nil)
	require.False(t, res.IsError)
	call(t, userA, "list_proxies", map[string]any{}, &list)
	assert.Empty(t, list)
}

// list_apps?source=store：应用市场（app_market, ready）→ source=market、refId=market 行 id。
func TestListAppsTool(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO app_market (s3_key, app_name, package_name, version, md5, file_size, parse_status) VALUES (?,?,?,?,?,?,?)",
		"app-market/wx.apk", "微信", "com.tencent.mm", "8.0",
		"00000000000000000000000000000000", 4096, "ready").Error)
	var mk struct{ ID uint }
	require.NoError(t, framework.DB.Raw("SELECT id FROM app_market WHERE s3_key=?", "app-market/wx.apk").Scan(&mk).Error)

	var store []appView
	call(t, userA, "list_apps", map[string]any{"source": "store"}, &store)
	require.Len(t, store, 1)
	assert.Equal(t, "market", store[0].Source)
	assert.Equal(t, mk.ID, store[0].RefID)
	assert.Equal(t, "com.tencent.mm", store[0].PackageName)
}

// list_scripts：只返回启用脚本，scriptId 可直接喂 run_script。
func TestListScriptsTool(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM automation_scripts") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO automation_scripts (user_id, store, script_id, name, version, status) VALUES (?,0,100,?,?,?)",
		userA, "签到", "1.0.0", "enabled").Error)
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO automation_scripts (user_id, store, script_id, name, version, status) VALUES (?,0,101,?,?,?)",
		userA, "停用的", "1.0.0", "disabled").Error)

	var scripts []scriptView
	call(t, userA, "list_scripts", map[string]any{}, &scripts)
	require.Len(t, scripts, 1, "停用脚本不应出现")
	assert.Equal(t, "签到", scripts[0].Name)
	assert.NotZero(t, scripts[0].ScriptID)
}
