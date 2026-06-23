package mcp

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/proxy"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== 辅助函数单测 =====

func TestToInt64Slice(t *testing.T) {
	assert.Nil(t, toInt64Slice(nil))
	assert.Nil(t, toInt64Slice([]int{}))
	assert.Equal(t, []int64{1, 2, 3}, toInt64Slice([]int{1, 2, 3}))
}

func TestExtractKey(t *testing.T) {
	// Authorization: Bearer 优先。
	r := &http.Request{Header: http.Header{}}
	r.Header.Set("Authorization", "Bearer gp_live_abc")
	assert.Equal(t, "gp_live_abc", extractKey(r))

	// Bearer 后有多余空白也会被 TrimSpace。
	r2 := &http.Request{Header: http.Header{}}
	r2.Header.Set("Authorization", "Bearer   spaced  ")
	assert.Equal(t, "spaced", extractKey(r2))

	// 无 Bearer 前缀 → 回退 X-API-Key。
	r3 := &http.Request{Header: http.Header{}}
	r3.Header.Set("X-API-Key", "  xkey ")
	assert.Equal(t, "xkey", extractKey(r3))

	// 全空 → 空串。
	r4 := &http.Request{Header: http.Header{}}
	assert.Equal(t, "", extractKey(r4))

	// Authorization 非 Bearer 形态（如 Basic）不当作 key，回退到（空的）X-API-Key。
	r5 := &http.Request{Header: http.Header{}}
	r5.Header.Set("Authorization", "Basic zzz")
	assert.Equal(t, "", extractKey(r5))
}

// httpContextFunc：无 key → 原样返回 ctx（不注入 userID）。
func TestHTTPContextFuncNoKey(t *testing.T) {
	base := context.Background()
	out := httpContextFunc(base, &http.Request{Header: http.Header{}})
	_, err := userIDFrom(out)
	assert.Error(t, err, "无 key 不应注入 userID")
}

// httpContextFunc：无效 key（鉴权失败）→ 原样返回，不注入。
func TestHTTPContextFuncInvalidKey(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	r.Header.Set("Authorization", "Bearer gp_live_not_a_real_key")
	out := httpContextFunc(context.Background(), r)
	_, err := userIDFrom(out)
	assert.Error(t, err, "无效 key 不应注入 userID")
}

func TestUserIDFrom(t *testing.T) {
	// 正常注入。
	ctx := context.WithValue(context.Background(), userIDKey, 42)
	uid, err := userIDFrom(ctx)
	require.NoError(t, err)
	assert.Equal(t, 42, uid)

	// 未注入。
	_, err = userIDFrom(context.Background())
	assert.Error(t, err)

	// uid<=0 视为未授权。
	ctx0 := context.WithValue(context.Background(), userIDKey, 0)
	_, err = userIDFrom(ctx0)
	assert.Error(t, err)
}

// ===== 通用结果辅助 =====

func TestJSONResultAndFailAndOk(t *testing.T) {
	res, err := jsonResult(map[string]any{"a": 1})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	// 不可序列化的值（chan）→ 返回错误结果但无传输层 err。
	res2, err := jsonResult(make(chan int))
	require.NoError(t, err)
	assert.True(t, res2.IsError)

	res3, err := fail(errors.New("boom"))
	require.NoError(t, err)
	assert.True(t, res3.IsError)

	res4, err := ok()
	require.NoError(t, err)
	assert.False(t, res4.IsError)
}

func TestBuildServer(t *testing.T) {
	s := buildServer()
	assert.NotNil(t, s)
}

func TestToProxyView(t *testing.T) {
	p := &proxy.Proxy{Name: "n", Protocol: "socks5", Host: "h", Port: 9, Region: "us", Status: "ok", EgressIP: "1.1.1.1"}
	p.ID = 5
	v := toProxyView(p)
	assert.Equal(t, 5, v.ID)
	assert.Equal(t, "n", v.Name)
	assert.Equal(t, "socks5", v.Protocol)
	assert.Equal(t, "1.1.1.1", v.EgressIP)
}

// ===== 各工具的参数校验 / 鉴权拒绝（不触达中台）=====

// 所有 18 个工具在无鉴权 context 下都返回 isError。
func TestAllToolsRejectUnauthenticated(t *testing.T) {
	for _, rt := range allTools() {
		res := call(t, 0, rt.tool.Name, map[string]any{}, nil)
		assert.Truef(t, res.IsError, "%s 无鉴权应被拒", rt.tool.Name)
	}
}

// 必填 id 缺失 → Validation 错误（鉴权通过但参数非法）。
func TestPhoneToolsMissingID(t *testing.T) {
	for _, name := range []string{"get_phone", "destroy_phone", "power_phone", "restart_phone", "bind_proxy", "list_installed_apps", "install_app", "uninstall_app"} {
		res := call(t, userA, name, map[string]any{}, nil)
		assert.Truef(t, res.IsError, "%s 缺 id 应报错", name)
	}
}

// create_phone：name 必填缺失 → 报错。
func TestCreatePhoneMissingName(t *testing.T) {
	res := call(t, userA, "create_phone", map[string]any{}, nil)
	assert.True(t, res.IsError)
}

// power_phone：operation 非 on/off → 报错（在触达 service 前拦截）。
func TestPowerPhoneBadOperation(t *testing.T) {
	res := call(t, userA, "power_phone", map[string]any{"id": 999999, "operation": "reboot"}, nil)
	assert.True(t, res.IsError)
}

// power_phone：合法 on/off 但云手机不存在 → service NotFound（覆盖 op 映射 + fail）。
func TestPowerPhoneNotFound(t *testing.T) {
	for _, op := range []string{"on", "off"} {
		res := call(t, userA, "power_phone", map[string]any{"id": 999999, "operation": op}, nil)
		assert.Truef(t, res.IsError, "op=%s 不存在的机器应 NotFound", op)
	}
}

// bind_proxy：proxyId<=0 → 校验失败。
func TestBindProxyMissingProxyID(t *testing.T) {
	res := call(t, userA, "bind_proxy", map[string]any{"id": 1, "proxyId": 0}, nil)
	assert.True(t, res.IsError)
}

// get_phone / destroy_phone / restart_phone：不存在的 id → service NotFound（覆盖 fail）。
func TestPhoneActionsNotFound(t *testing.T) {
	for _, name := range []string{"get_phone", "destroy_phone", "restart_phone", "list_installed_apps"} {
		res := call(t, userA, name, map[string]any{"id": 999999}, nil)
		assert.Truef(t, res.IsError, "%s 不存在的机器应报错", name)
	}
}

// install_app：缺 appIds → 校验失败。
func TestInstallAppMissingAppIDs(t *testing.T) {
	res := call(t, userA, "install_app", map[string]any{"id": 1}, nil)
	assert.True(t, res.IsError)
}

// install_app：带 appIds 但机器不存在 → service NotFound。
func TestInstallAppNotFound(t *testing.T) {
	res := call(t, userA, "install_app", map[string]any{"id": 999999, "appIds": []any{1, 2}}, nil)
	assert.True(t, res.IsError)
}

// uninstall_app：appIds 与 packageNames 都空 → 校验失败。
func TestUninstallAppMissingBoth(t *testing.T) {
	res := call(t, userA, "uninstall_app", map[string]any{"id": 1}, nil)
	assert.True(t, res.IsError)
}

// uninstall_app：给了 packageNames 但机器不存在 → service NotFound。
func TestUninstallAppNotFound(t *testing.T) {
	res := call(t, userA, "uninstall_app", map[string]any{"id": 999999, "packageNames": []any{"com.x"}}, nil)
	assert.True(t, res.IsError)
}

// ===== 应用工具 =====

// list_apps：source=store（仅商店）路径。
func TestListAppsStoreSource(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM customer_apps WHERE app_name='store-only'") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO customer_apps (user_id, store, cp_app_id, app_name, package_name, version, status) VALUES (?,1,7777,?,?,?,?)",
		userA, "store-only", "com.store.x", "1.0", "NORMAL").Error)

	var store []appView
	res := call(t, userA, "list_apps", map[string]any{"source": "store"}, &store)
	require.False(t, res.IsError)
	found := false
	for _, a := range store {
		if a.AppID == 7777 {
			found = true
		}
	}
	assert.True(t, found, "store source 应含商店应用")
}

// list_apps：默认 source=all（不传 source）。
func TestListAppsDefaultAll(t *testing.T) {
	var all []appView
	res := call(t, userA, "list_apps", map[string]any{}, &all)
	assert.False(t, res.IsError)
}

// ===== 脚本 / 任务工具 =====

// run_script：缺 scriptId → 校验失败。
func TestRunScriptMissingScriptID(t *testing.T) {
	res := call(t, userA, "run_script", map[string]any{"id": 1}, nil)
	assert.True(t, res.IsError)
}

// run_script：scriptId 合法但云手机不存在 → CpIDOf NotFound。
func TestRunScriptPhoneNotFound(t *testing.T) {
	res := call(t, userA, "run_script", map[string]any{"id": 999999, "scriptId": 1}, nil)
	assert.True(t, res.IsError)
}

// run_script：云手机存在但未开通（cpId 为空）→ "尚未开通"。
func TestRunScriptPhoneNotProvisioned(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE cp_id=''") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO cloud_phones (user_id, cp_id, name, status, proxy_id) VALUES (?,?,?,?,0)",
		userA, "", "未开通", "CREATING").Error)
	var pid int
	require.NoError(t, framework.DB.Raw(
		"SELECT id FROM cloud_phones WHERE user_id=? AND cp_id='' ORDER BY id DESC LIMIT 1", userA).Scan(&pid).Error)
	require.NotZero(t, pid)

	res := call(t, userA, "run_script", map[string]any{"id": pid, "scriptId": 1}, nil)
	assert.True(t, res.IsError, "未开通的云手机不能跑脚本")
}

// get_task：缺 taskId → 校验失败。
func TestGetTaskMissingTaskID(t *testing.T) {
	res := call(t, userA, "get_task", map[string]any{}, nil)
	assert.True(t, res.IsError)
}

// get_task：taskId 合法但任务不存在 → NotFound（或 ops 未配置错误），均 isError。
func TestGetTaskNotFound(t *testing.T) {
	res := call(t, userA, "get_task", map[string]any{"taskId": 999999}, nil)
	assert.True(t, res.IsError)
}

// ===== 代理工具：校验 =====

// create_proxy：核心字段缺失 → proxyInputFrom 校验失败。
func TestCreateProxyMissingCore(t *testing.T) {
	res := call(t, userA, "create_proxy", map[string]any{"name": "only-name"}, nil)
	assert.True(t, res.IsError)
}

// update_proxy / delete_proxy：缺 id → 校验失败。
func TestProxyUpdateDeleteMissingID(t *testing.T) {
	for _, name := range []string{"update_proxy", "delete_proxy"} {
		res := call(t, userA, name, map[string]any{}, nil)
		assert.Truef(t, res.IsError, "%s 缺 id 应报错", name)
	}
}

// update_proxy / delete_proxy：id 合法但不存在 → service NotFound。
func TestProxyUpdateDeleteNotFound(t *testing.T) {
	for _, name := range []string{"update_proxy", "delete_proxy"} {
		res := call(t, userA, name, map[string]any{"id": 999999}, nil)
		assert.Truef(t, res.IsError, "%s 不存在 id 应报错", name)
	}
}

// proxyInputFrom 直接单测：requireCore 两路。
func TestProxyInputFrom(t *testing.T) {
	mk := func(args map[string]any) mcp.CallToolRequest {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = args
		return req
	}
	// requireCore=true 且核心齐全 → 无错误。
	in, err := proxyInputFrom(mk(map[string]any{
		"name": "p", "protocol": "http", "host": "h", "port": 8080,
	}), true)
	require.NoError(t, err)
	assert.Equal(t, "p", in.Name)
	assert.Equal(t, 8080, in.Port)

	// requireCore=true 但缺 port → 错误。
	_, err = proxyInputFrom(mk(map[string]any{
		"name": "p", "protocol": "http", "host": "h",
	}), true)
	assert.Error(t, err)

	// requireCore=false 允许部分字段。
	in2, err := proxyInputFrom(mk(map[string]any{"name": "only"}), false)
	require.NoError(t, err)
	assert.Equal(t, "only", in2.Name)
}
