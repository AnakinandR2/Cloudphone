package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userOpen = 8301

// callRaw 全自定义调用一个开放 handler：可选 userID、method、body、路径参数（id 或 taskId）。
// 返回 HTTP 状态码 + envelope.code。
func callRaw(setUID bool, uid int, method, path, body string, params gin.Params, h gin.HandlerFunc) (int, int) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if setUID {
		c.Set("userID", uid)
	}
	if params != nil {
		c.Params = params
	}
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Content-Type", "application/json")
	c.Request = r
	h(c)
	var env struct {
		Code int `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Code
}

func idParams(id int) gin.Params { return gin.Params{{Key: "id", Value: strconv.Itoa(id)}} }

// seedOpenPhone 造一台属于 owner 的云手机（cpID 默认空=未开通），返回其本地 id。
func seedOpenPhone(t *testing.T, owner int, cpID, status string) int {
	t.Helper()
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO cloud_phones (user_id, cp_id, name, status, proxy_id) VALUES (?,?,?,?,0)",
		owner, cpID, "openphone", status).Error)
	var id int
	require.NoError(t, framework.DB.Raw(
		"SELECT id FROM cloud_phones WHERE user_id=? ORDER BY id DESC LIMIT 1", owner).Scan(&id).Error)
	return id
}

// ===== 鉴权缺失：所有 handler currentUserID/openCtx 未授权分支 → 401 =====

func TestOpenHandlersUnauthorized(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		h       gin.HandlerFunc
		withID  bool
		taskKey bool
	}{
		{"ListPhones", "GET", OpenListPhones, false, false},
		{"GetPhone", "GET", OpenGetPhone, true, false},
		{"CreatePhone", "POST", OpenCreatePhone, false, false},
		{"DestroyPhone", "DELETE", OpenDestroyPhone, true, false},
		{"Power", "POST", OpenPower, true, false},
		{"Restart", "POST", OpenRestart, true, false},
		{"ListApps", "GET", OpenListApps, false, false},
		{"ListScripts", "GET", OpenListScripts, false, false},
		{"ListProxies", "GET", OpenListProxies, false, false},
		{"CreateProxy", "POST", OpenCreateProxy, false, false},
		{"UpdateProxy", "PUT", OpenUpdateProxy, true, false},
		{"DeleteProxy", "DELETE", OpenDeleteProxy, true, false},
		{"BindProxy", "POST", OpenBindProxy, true, false},
		{"Apps", "GET", OpenApps, true, false},
		{"Install", "POST", OpenInstall, true, false},
		{"Uninstall", "POST", OpenUninstall, true, false},
		{"RunScript", "POST", OpenRunScript, true, false},
		{"TaskDetail", "GET", OpenTaskDetail, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var params gin.Params
			if tc.withID {
				params = idParams(1)
			}
			if tc.taskKey {
				params = gin.Params{{Key: "taskId", Value: "1"}}
			}
			status, _ := callRaw(false, 0, tc.method, "/x", "", params, tc.h)
			assert.Equal(t, http.StatusUnauthorized, status, "%s 缺认证应 401", tc.name)
		})
	}
}

// ===== openCtx 非法 ID → 400（含路径无 id 参数与负数/0） =====

func TestOpenCtxBadID(t *testing.T) {
	idHandlers := []struct {
		name   string
		method string
		h      gin.HandlerFunc
	}{
		{"GetPhone", "GET", OpenGetPhone},
		{"DestroyPhone", "DELETE", OpenDestroyPhone},
		{"Power", "POST", OpenPower},
		{"Restart", "POST", OpenRestart},
		{"UpdateProxy", "PUT", OpenUpdateProxy},
		{"DeleteProxy", "DELETE", OpenDeleteProxy},
		{"BindProxy", "POST", OpenBindProxy},
		{"Apps", "GET", OpenApps},
		{"Install", "POST", OpenInstall},
		{"Uninstall", "POST", OpenUninstall},
		{"RunScript", "POST", OpenRunScript},
	}
	for _, tc := range idHandlers {
		t.Run(tc.name+"_nonNumeric", func(t *testing.T) {
			status, _ := callRaw(true, userOpen, tc.method, "/x", "",
				gin.Params{{Key: "id", Value: "abc"}}, tc.h)
			assert.Equal(t, http.StatusBadRequest, status)
		})
		t.Run(tc.name+"_zero", func(t *testing.T) {
			status, _ := callRaw(true, userOpen, tc.method, "/x", "",
				gin.Params{{Key: "id", Value: "0"}}, tc.h)
			assert.Equal(t, http.StatusBadRequest, status)
		})
	}
}

// ===== 云手机列表 / 详情（pure DB，ops==nil 仍可走通）=====

func TestOpenListPhonesPaging(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })
	seedOpenPhone(t, userOpen, "", "CREATED")
	seedOpenPhone(t, userOpen, "", "CREATED")

	// 默认分页。
	status, code := callRaw(true, userOpen, "GET", "/x?page=1&size=20", "", nil, OpenListPhones)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 0, code)

	// 非法分页参数被矫正（page<1、size>200）→ 仍 200。
	status, _ = callRaw(true, userOpen, "GET", "/x?page=0&size=999", "", nil, OpenListPhones)
	assert.Equal(t, http.StatusOK, status)

	// size 非数字 → 矫正为默认 → 200。
	status, _ = callRaw(true, userOpen, "GET", "/x?size=abc", "", nil, OpenListPhones)
	assert.Equal(t, http.StatusOK, status)
}

func TestOpenGetPhone(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id IN (?,?)", userOpen, userOpen+1) })
	id := seedOpenPhone(t, userOpen, "", "CREATED")

	// 本人 → 200。
	status, code := callRaw(true, userOpen, "GET", "/x", "", idParams(id), OpenGetPhone)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 0, code)

	// 他人 → 404（NotFound）。
	status, _ = callRaw(true, userOpen+1, "GET", "/x", "", idParams(id), OpenGetPhone)
	assert.Equal(t, http.StatusNotFound, status)

	// 不存在的 id → 404。
	status, _ = callRaw(true, userOpen, "GET", "/x", "", idParams(999999), OpenGetPhone)
	assert.Equal(t, http.StatusNotFound, status)
}

// ===== 创建云手机：seat 不足 → Conflict（handler 全程执行）=====

func TestOpenCreatePhoneSeatGate(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })

	// 非法 JSON → 400。
	status, _ := callRaw(true, userOpen, "POST", "/x", `not json`, nil, OpenCreatePhone)
	assert.Equal(t, http.StatusBadRequest, status)

	// 无 seat 容量（默认 0）且已有 0 台 → checkSeatAvailable: 0>=0 → Conflict(409)。
	status, code := callRaw(true, userOpen, "POST", "/x", `{"name":"newphone"}`, nil, OpenCreatePhone)
	assert.Equal(t, http.StatusConflict, status)
	assert.NotEqual(t, 0, code)
}

// ===== 开关机 / 重启 / 销毁：未开通（cpID 空）→ Validation(422)；非本人→404 =====

func TestOpenPower(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })
	id := seedOpenPhone(t, userOpen, "", "CREATED")

	// operation 缺失/非法 → 400。
	status, _ := callRaw(true, userOpen, "POST", "/x", `{"operation":"bad"}`, idParams(id), OpenPower)
	assert.Equal(t, http.StatusBadRequest, status)

	// operation=on，但云手机未开通（cpID 空）→ resolveCp Validation(422)。
	status, code := callRaw(true, userOpen, "POST", "/x", `{"operation":"on"}`, idParams(id), OpenPower)
	assert.NotEqual(t, http.StatusOK, status)
	assert.NotEqual(t, 0, code)

	// operation=off 同理走 service → 非 200。
	status, _ = callRaw(true, userOpen, "POST", "/x", `{"operation":"off"}`, idParams(id), OpenPower)
	assert.NotEqual(t, http.StatusOK, status)
}

func TestOpenRestartDestroy(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })
	id := seedOpenPhone(t, userOpen, "", "CREATED")

	// 未开通 → restart Validation(422)。
	status, code := callRaw(true, userOpen, "POST", "/x", "", idParams(id), OpenRestart)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.NotEqual(t, 0, code)

	// destroy 未开通 → Validation(422)。
	status, _ = callRaw(true, userOpen, "DELETE", "/x", "", idParams(id), OpenDestroyPhone)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

// ===== 应用：已装列表 / 安装 / 卸载（未开通 → service 错误）=====

func TestOpenAppsInstallUninstall(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })
	id := seedOpenPhone(t, userOpen, "", "CREATED")

	// 已装列表：未开通 → 非 200。
	status, _ := callRaw(true, userOpen, "GET", "/x", "", idParams(id), OpenApps)
	assert.NotEqual(t, http.StatusOK, status)

	// 安装：未开通 → 非 200（属主/开通校验先于解析与下发）。
	status, _ = callRaw(true, userOpen, "POST", "/x", `{"apps":[{"source":"user","refId":1092}]}`, idParams(id), OpenInstall)
	assert.NotEqual(t, http.StatusOK, status)

	// 卸载：未开通 → 非 200。
	status, _ = callRaw(true, userOpen, "POST", "/x", `{"packageNames":["com.x"]}`, idParams(id), OpenUninstall)
	assert.NotEqual(t, http.StatusOK, status)

	// 非本人手机 → 404。
	status, _ = callRaw(true, userOpen+1, "GET", "/x", "", idParams(id), OpenApps)
	assert.Equal(t, http.StatusNotFound, status)
}

// ===== 脚本：run-script 缺 scriptId → 400；未开通 → 404；查任务非法 id → 400 =====

func TestOpenRunScript(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE user_id=?", userOpen) })
	id := seedOpenPhone(t, userOpen, "", "CREATED")

	// 缺 scriptId → 400。
	status, _ := callRaw(true, userOpen, "POST", "/x", `{}`, idParams(id), OpenRunScript)
	assert.Equal(t, http.StatusBadRequest, status)

	// 非本人手机 → CpIDOf NotFound → 404。
	status, _ = callRaw(true, userOpen+1, "POST", "/x", `{"scriptId":5}`, idParams(id), OpenRunScript)
	assert.Equal(t, http.StatusNotFound, status)

	// 本人但未开通（cpID 空）→ Validation(422)「尚未开通」。
	status, code := callRaw(true, userOpen, "POST", "/x", `{"scriptId":5}`, idParams(id), OpenRunScript)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.NotEqual(t, 0, code)
}

func TestOpenTaskDetail(t *testing.T) {
	// 任务 id 非法（非数字）→ 400。
	status, _ := callRaw(true, userOpen, "GET", "/x", "",
		gin.Params{{Key: "taskId", Value: "abc"}}, OpenTaskDetail)
	assert.Equal(t, http.StatusBadRequest, status)

	// taskId<=0 → 400。
	status, _ = callRaw(true, userOpen, "GET", "/x", "",
		gin.Params{{Key: "taskId", Value: "0"}}, OpenTaskDetail)
	assert.Equal(t, http.StatusBadRequest, status)

	// 不存在的任务 → service 返回错误（非 200）。
	status, _ = callRaw(true, userOpen, "GET", "/x", "",
		gin.Params{{Key: "taskId", Value: "987654"}}, OpenTaskDetail)
	assert.NotEqual(t, http.StatusOK, status)
}

// ===== 代理：CreateProxy 非法 body → 400；BindProxy 缺 proxyId → 400 =====

func TestOpenProxyValidation(t *testing.T) {
	// CreateProxy 非法 JSON → 400。
	status, _ := callRaw(true, userOpen, "POST", "/x", `bad`, nil, OpenCreateProxy)
	assert.Equal(t, http.StatusBadRequest, status)

	// UpdateProxy 非法 JSON → 400（id 合法）。
	status, _ = callRaw(true, userOpen, "PUT", "/x", `bad`, idParams(1), OpenUpdateProxy)
	assert.Equal(t, http.StatusBadRequest, status)

	// BindProxy 缺 proxyId → 400。
	status, _ = callRaw(true, userOpen, "POST", "/x", `{"proxyId":0}`, idParams(1), OpenBindProxy)
	assert.Equal(t, http.StatusBadRequest, status)

	// DeleteProxy 不存在 → 404。
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM proxies WHERE user_id=?", userOpen) })
	status, _ = callRaw(true, userOpen, "DELETE", "/x", "", idParams(987654), OpenDeleteProxy)
	assert.Equal(t, http.StatusNotFound, status)
}

// ===== ListApps source=store 分支（市场应用，app_market） =====

func TestOpenListAppsSources(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market") })
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO app_market (s3_key, app_name, package_name, version, md5, file_size, parse_status) VALUES (?,?,?,?,?,?,?)",
		"app-market/store.apk", "商店应用", "com.store", "2.0",
		"00000000000000000000000000000000", 2048, "ready").Error)
	var mk struct{ ID uint }
	require.NoError(t, framework.DB.Raw("SELECT id FROM app_market WHERE s3_key=?", "app-market/store.apk").Scan(&mk).Error)

	var store []OpenApp
	callOpen(userOpen, "?source=store", OpenListApps, &store)
	found := false
	for _, a := range store {
		if a.RefID == mk.ID && a.Source == "market" {
			found = true
		}
	}
	assert.True(t, found, "store 来源应含市场应用")
}
