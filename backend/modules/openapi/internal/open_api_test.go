package openapi

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// callOpen 用给定属主与查询串调用一个开放 handler，返回解包后的 data。
func callOpen(userID int, query string, h gin.HandlerFunc, out any) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest("GET", "/x"+query, nil)
	h(c)
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	_ = json.Unmarshal(env.Data, out)
}

// callBody 以 method+body 调用一个开放 handler（可带路径参数 id），返回 envelope.code。
func callBody(userID int, method, body string, idParam string, h gin.HandlerFunc, out any) int {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	if idParam != "" {
		c.Params = gin.Params{{Key: "id", Value: idParam}}
	}
	c.Request = httptest.NewRequest(method, "/x", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h(c)
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if out != nil {
		_ = json.Unmarshal(env.Data, out)
	}
	return env.Code
}

// /apps?source=store：应用市场（app_market, ready）→ refId 等于 market 行 id，Store=true。
// 我的应用（source=mine）走素材库 + 解析元数据，依赖私有 S3，单测此处只验市场闭环。
func TestOpenListAppsClosesInstallLoop(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO app_market (s3_key, app_name, package_name, version, md5, file_size, parse_status) VALUES (?,?,?,?,?,?,?)",
		"app-market/tiktok.apk", "TikTok", "com.ss.android.ugc.trill", "40",
		"00000000000000000000000000000000", 4096, "ready").Error)
	var mk struct{ ID uint }
	require.NoError(t, framework.DB.Raw("SELECT id FROM app_market WHERE s3_key=?", "app-market/tiktok.apk").Scan(&mk).Error)

	var store []OpenApp
	callOpen(userA, "?source=store", OpenListApps, &store)
	require.Len(t, store, 1)
	assert.Equal(t, "market", store[0].Source)
	assert.Equal(t, mk.ID, store[0].RefID)
	assert.Equal(t, "com.ss.android.ugc.trill", store[0].PackageName)
	assert.True(t, store[0].Store)
	assert.Equal(t, "ready", store[0].Status)
}

// /scripts：只返回启用脚本，scriptId 可直接喂 run-script。
func TestOpenListScriptsClosesRunLoop(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM automation_scripts") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO automation_scripts (user_id, store, script_id, name, version, status) VALUES (?,0,100,?,?,?)",
		userA, "签到", "1.0.0", "enabled").Error)
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO automation_scripts (user_id, store, script_id, name, version, status) VALUES (?,0,101,?,?,?)",
		userA, "停用的", "1.0.0", "disabled").Error)

	var scripts []OpenScript
	callOpen(userA, "", OpenListScripts, &scripts)
	require.Len(t, scripts, 1, "停用脚本不应出现")
	assert.Equal(t, "签到", scripts[0].Name)
	assert.NotZero(t, scripts[0].ScriptID, "scriptId 可直接喂 run-script")
}

// 代理闭环：增 / 查 / 改 / 删，且能绑定到云手机。
func TestOpenProxyCrudAndBind(t *testing.T) {
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM proxies")
		framework.DB.Exec("DELETE FROM cloud_phones")
	})

	var created OpenProxy
	code := callBody(userA, "POST", `{"name":"px1","protocol":"socks5","host":"1.2.3.4","port":1080}`, "", OpenCreateProxy, &created)
	require.Equal(t, 0, code)
	require.NotZero(t, created.ID)
	assert.Equal(t, "1.2.3.4", created.Host)

	var list []OpenProxy
	callOpen(userA, "", OpenListProxies, &list)
	require.Len(t, list, 1)

	var updated OpenProxy
	callBody(userA, "PUT", `{"name":"px1b","protocol":"socks5","host":"5.6.7.8","port":1080}`, strconv.Itoa(created.ID), OpenUpdateProxy, &updated)
	assert.Equal(t, "5.6.7.8", updated.Host)

	// 绑定到一台本人云手机：proxy_id 落到该机。
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO cloud_phones (user_id, cp_id, name, status, proxy_id) VALUES (?,?,?,?,0)",
		userA, "cp-bind", "m", "STOPPED").Error)
	var phoneID int
	require.NoError(t, framework.DB.Raw("SELECT id FROM cloud_phones WHERE cp_id='cp-bind'").Scan(&phoneID).Error)
	code = callBody(userA, "POST", `{"proxyId":`+strconv.Itoa(created.ID)+`}`, strconv.Itoa(phoneID), OpenBindProxy, nil)
	require.Equal(t, 0, code)
	var boundProxyID int
	framework.DB.Raw("SELECT proxy_id FROM cloud_phones WHERE id=?", phoneID).Scan(&boundProxyID)
	assert.Equal(t, created.ID, boundProxyID, "proxy 应绑定到该云手机")

	// 删除代理。
	code = callBody(userA, "DELETE", ``, strconv.Itoa(created.ID), OpenDeleteProxy, nil)
	require.Equal(t, 0, code)
	callOpen(userA, "", OpenListProxies, &list)
	assert.Empty(t, list)
}
