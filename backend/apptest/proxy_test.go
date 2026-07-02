package apptest

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 前台「我的代理」全链路：创建 → 列表 → 详情 → 更新 → 删除（按属主隔离，用唯一手机号造数据）。
// 不触碰 /proxy/{id}/test 与 /proxy/probe（需真实网络探测）。
func TestProxyOwnerCRUDHTTP(t *testing.T) {
	r := setupRouter()
	token := registerUser(t, r, "13900020001")

	// 未登录不可访问
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/proxy/list", "", nil).Code)

	// 创建
	w := doJSON(r, "POST", "/api/v1/proxy/create", token, map[string]interface{}{
		"name": "apptest-proxy-1", "host": "203.0.113.7", "port": 1080,
		"username": "u1", "password": "secret", "region": "测试区",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	created := decode(t, w).Data.(map[string]interface{})
	proxyID := int(created["id"].(float64))
	assert.Equal(t, "socks5", created["protocol"]) // 默认协议
	// 敏感字段 password 绝不回显
	_, hasPwd := created["password"]
	assert.False(t, hasPwd, "password 不应出现在响应里")

	// 列表能看到本人这条
	data := decode(t, doJSON(r, "GET", "/api/v1/proxy/list?kw=apptest-proxy-1", token, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])

	// 详情
	w = doJSON(r, "GET", fmt.Sprintf("/api/v1/proxy/%d", proxyID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "203.0.113.7", decode(t, w).Data.(map[string]interface{})["host"])

	// 更新
	w = doJSON(r, "PUT", fmt.Sprintf("/api/v1/proxy/update/%d", proxyID), token, map[string]interface{}{
		"name": "apptest-proxy-renamed", "port": 1081,
	})
	require.Equal(t, http.StatusOK, w.Code)
	upd := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, "apptest-proxy-renamed", upd["name"])
	assert.Equal(t, float64(1081), upd["port"])

	// options 下拉（unknown 状态可选）应含本条
	opts := decode(t, doJSON(r, "GET", "/api/v1/proxy/options", token, nil)).Data.([]interface{})
	assert.GreaterOrEqual(t, len(opts), 1)

	// 删除
	require.Equal(t, http.StatusOK, doJSON(r, "DELETE", fmt.Sprintf("/api/v1/proxy/delete/%d", proxyID), token, nil).Code)
	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", fmt.Sprintf("/api/v1/proxy/%d", proxyID), token, nil).Code)
}

// 属主隔离：B 看不到 / 动不了 A 的代理（均 404）。
func TestProxyOwnerIsolationHTTP(t *testing.T) {
	r := setupRouter()
	tokenA := registerUser(t, r, "13900020002")
	tokenB := registerUser(t, r, "13900020003")

	w := doJSON(r, "POST", "/api/v1/proxy/create", tokenA, map[string]interface{}{
		"name": "A的代理", "host": "203.0.113.8", "port": 1080,
	})
	require.Equal(t, http.StatusOK, w.Code)
	proxyID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))

	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", fmt.Sprintf("/api/v1/proxy/%d", proxyID), tokenB, nil).Code)
	assert.Equal(t, http.StatusNotFound,
		doJSON(r, "PUT", fmt.Sprintf("/api/v1/proxy/update/%d", proxyID), tokenB, map[string]string{"name": "篡改"}).Code)
	assert.Equal(t, http.StatusNotFound,
		doJSON(r, "DELETE", fmt.Sprintf("/api/v1/proxy/delete/%d", proxyID), tokenB, nil).Code)
}

// 非法请求：坏 ID → 400；缺 required（name/host/port）→ 400。
func TestProxyInvalidRequestsHTTP(t *testing.T) {
	r := setupRouter()
	token := registerUser(t, r, "13900020004")

	assert.Equal(t, http.StatusBadRequest, doJSON(r, "GET", "/api/v1/proxy/abc", token, nil).Code)
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "POST", "/api/v1/proxy/create", token, map[string]interface{}{"host": "203.0.113.9", "port": 1080}).Code)
}

// 管理侧代理池：admin 列表（看到前台造的代理）+ admin 删除（adminToken）。
func TestProxyAdminListDeleteHTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	token := registerUser(t, r, "13900020005")

	// 前台造一条
	w := doJSON(r, "POST", "/api/v1/proxy/create", token, map[string]interface{}{
		"name": "apptest-admin-proxy", "host": "203.0.113.10", "port": 1080,
	})
	require.Equal(t, http.StatusOK, w.Code)
	proxyID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))

	// admin 列表（kw 过滤）能找到
	data := decode(t, doJSON(r, "GET", "/api/v1/admin/proxies/list?kw=apptest-admin-proxy", admin, nil)).Data.(map[string]interface{})
	assert.GreaterOrEqual(t, data["total"].(float64), float64(1))

	// admin 详情
	w = doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/proxies/%d", proxyID), admin, nil)
	require.Equal(t, http.StatusOK, w.Code)

	// 前台令牌不能打 admin 路由 → 401（身份域不匹配）
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/admin/proxies/list", token, nil).Code)

	// admin 删除
	require.Equal(t, http.StatusOK, doJSON(r, "DELETE", fmt.Sprintf("/api/v1/admin/proxies/delete/%d", proxyID), admin, nil).Code)
	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/proxies/%d", proxyID), admin, nil).Code)
}

// S2 回归：即时探测 /proxy/probe 必须拦截内网/环回/链路本地/CGNAT 地址，
// 防被当作内网端口扫描/云元数据 SSRF 原语。拦截命中拨号前的 resolvePublicHostPort，
// Probe 把 error 收敛成 ProbeOutcome{status:"fail", message:...}（非接口错误），
// 故 HTTP 200 + 业务 Code=0，但 data.status=="fail" 且 data.message 非空。
// 只验拦截路径，不传公网地址（避免真实拨号）。
func TestProxyProbeRejectsPrivateHTTP(t *testing.T) {
	r := setupRouter()
	token := registerUser(t, r, "13900020006")

	// 未登录不可访问该前台接口
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "POST", "/api/v1/proxy/probe", "", map[string]interface{}{
			"host": "203.0.113.7", "port": 1080,
		}).Code)

	for _, host := range []string{
		"127.0.0.1",       // 环回
		"10.0.0.1",        // 私网
		"169.254.169.254", // 链路本地 / 云元数据
		"100.64.0.1",      // CGNAT（RFC6598）
	} {
		w := doJSON(r, "POST", "/api/v1/proxy/probe", token, map[string]interface{}{
			"host": host, "port": 6379,
		})
		// 拦截被收敛为“探测失败”结果：HTTP 200 + 业务成功码，但内容判失败。
		require.Equal(t, http.StatusOK, w.Code, "host=%s body=%s", host, w.Body.String())
		resp := decode(t, w)
		require.Equal(t, 0, resp.Code, "host=%s 业务码应为 0（探测结果而非接口错误）", host)
		data, ok := resp.Data.(map[string]interface{})
		require.True(t, ok, "host=%s data 应为对象", host)
		assert.Equal(t, "fail", data["status"], "host=%s 应判为探测失败（被内网拦截）", host)
		assert.NotEmpty(t, data["message"], "host=%s 失败应带拦截原因", host)
		// 断言命中的是 SSRF 守卫的拒绝文案（resolvePublicHostPort/isDisallowedIP），
		// 而非真实 SOCKS5 拨号失败的通用报错——否则即使守卫被误删，对内网地址的
		// 真实拨号同样会失败并收敛成同样的 {status:fail}，测试会“假通过”。
		msg, _ := data["message"].(string)
		assert.Contains(t, msg, "代理地址不合法", "host=%s message 应为守卫拒绝文案而非通用拨号失败", host)
		// 内网拦截发生在拨号前，绝不该返回出口 IP
		assert.Empty(t, data["egress_ip"], "host=%s 被拦截不应有出口 IP", host)
	}
}
