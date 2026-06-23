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

const userKeyAPI = 8101

// callKeyHandler 直接构造 gin.Context（预置/不预置 userID）调用密钥管理 handler，返回 HTTP 状态码 + envelope。
func callKeyHandler(setUID bool, uid int, method, body, idParam string, h gin.HandlerFunc) (int, struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if setUID {
		c.Set("userID", uid)
	}
	if idParam != "" {
		c.Params = gin.Params{{Key: "id", Value: idParam}}
	}
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/x", strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, "/x", nil)
	}
	r.Header.Set("Content-Type", "application/json")
	c.Request = r
	h(c)
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env
}

// ListKeys：未授权→401；授权→返回本人密钥列表（掩码）。
func TestListKeysHandler(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	// 缺 userID → 401。
	status, _ := callKeyHandler(false, 0, "GET", "", "", ListKeys)
	assert.Equal(t, http.StatusUnauthorized, status)

	// 造两把本人密钥 + 一把他人密钥。
	_, err := Service.Create(userKeyAPI, "k1")
	require.NoError(t, err)
	_, err = Service.Create(userKeyAPI, "k2")
	require.NoError(t, err)
	_, err = Service.Create(userKeyAPI+1, "其他人的")
	require.NoError(t, err)

	status, env := callKeyHandler(true, userKeyAPI, "GET", "", "", ListKeys)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 0, env.Code)
	var list []KeyView
	require.NoError(t, json.Unmarshal(env.Data, &list))
	require.Len(t, list, 2, "只列出本人密钥")
	assert.Contains(t, list[0].Masked, "••••")
}

// CreateKey：未授权→401；名称缺失→422；成功→返回完整明文一次。
func TestCreateKeyHandler(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	status, _ := callKeyHandler(false, 0, "POST", `{"name":"x"}`, "", CreateKey)
	assert.Equal(t, http.StatusUnauthorized, status)

	// 请求体非法 JSON → 400。
	status, _ = callKeyHandler(true, userKeyAPI, "POST", `not json`, "", CreateKey)
	assert.Equal(t, http.StatusBadRequest, status)

	// 名称为空 → service 返回 BadRequest（apperr.BadRequest → 400）。
	status, env := callKeyHandler(true, userKeyAPI, "POST", `{"name":"   "}`, "", CreateKey)
	assert.NotEqual(t, http.StatusOK, status)
	assert.NotEqual(t, 0, env.Code)

	// 成功创建：返回完整明文。
	status, env = callKeyHandler(true, userKeyAPI, "POST", `{"name":"我的密钥"}`, "", CreateKey)
	assert.Equal(t, http.StatusOK, status)
	var res CreateKeyResult
	require.NoError(t, json.Unmarshal(env.Data, &res))
	assert.True(t, strings.HasPrefix(res.FullKey, "gp_live_"))
	assert.Equal(t, "我的密钥", res.Name)
}

// RevealKey：未授权→401；ID 非法→400；成功→返回与创建一致明文；越权→404。
func TestRevealKeyHandler(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	res, err := Service.Create(userKeyAPI, "可查看")
	require.NoError(t, err)

	// 缺 userID → 401。
	status, _ := callKeyHandler(false, 0, "GET", "", strconv.Itoa(int(res.ID)), RevealKey)
	assert.Equal(t, http.StatusUnauthorized, status)

	// ID 非法（非数字）→ 400。
	status, _ = callKeyHandler(true, userKeyAPI, "GET", "", "abc", RevealKey)
	assert.Equal(t, http.StatusBadRequest, status)

	// ID=0 → 400（paramUint 拒绝 0）。
	status, _ = callKeyHandler(true, userKeyAPI, "GET", "", "0", RevealKey)
	assert.Equal(t, http.StatusBadRequest, status)

	// 成功：返回完整明文。
	status, env := callKeyHandler(true, userKeyAPI, "GET", "", strconv.Itoa(int(res.ID)), RevealKey)
	assert.Equal(t, http.StatusOK, status)
	var data struct {
		FullKey string `json:"fullKey"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	assert.Equal(t, res.FullKey, data.FullKey)

	// 越权 reveal → 404（apperr.NotFound）。
	status, _ = callKeyHandler(true, userKeyAPI+1, "GET", "", strconv.Itoa(int(res.ID)), RevealKey)
	assert.Equal(t, http.StatusNotFound, status)
}

// RevokeKey：未授权→401；ID 非法→400；成功→撤销且鉴权失效；越权→404。
func TestRevokeKeyHandler(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	res, err := Service.Create(userKeyAPI, "待撤销")
	require.NoError(t, err)

	status, _ := callKeyHandler(false, 0, "POST", "", strconv.Itoa(int(res.ID)), RevokeKey)
	assert.Equal(t, http.StatusUnauthorized, status)

	status, _ = callKeyHandler(true, userKeyAPI, "POST", "", "xyz", RevokeKey)
	assert.Equal(t, http.StatusBadRequest, status)

	// 越权撤销 → 404。
	status, _ = callKeyHandler(true, userKeyAPI+1, "POST", "", strconv.Itoa(int(res.ID)), RevokeKey)
	assert.Equal(t, http.StatusNotFound, status)

	// 成功撤销。
	status, env := callKeyHandler(true, userKeyAPI, "POST", "", strconv.Itoa(int(res.ID)), RevokeKey)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, 0, env.Code)

	_, ok := Service.Authenticate(res.FullKey)
	assert.False(t, ok, "撤销后鉴权应失败")
}

// paramUint 边界：非数字 / 0 → false；正常正整数 → true。
func TestParamUint(t *testing.T) {
	mk := func(v string) (uint, bool) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: v}}
		return paramUint(c, "id")
	}
	_, ok := mk("abc")
	assert.False(t, ok)
	_, ok = mk("0")
	assert.False(t, ok)
	_, ok = mk("-5")
	assert.False(t, ok)
	n, ok := mk("42")
	assert.True(t, ok)
	assert.Equal(t, uint(42), n)
}
