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
