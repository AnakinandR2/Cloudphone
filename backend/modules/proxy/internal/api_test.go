package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// newCtx 构造一个带 ResponseRecorder 的 gin.Context，可选注入 userID、path 参数、JSON body。
func newCtx(method, body string, userID *int, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/", reader)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	if userID != nil {
		c.Set("userID", *userID)
	}
	if params != nil {
		c.Params = params
	}
	return c, w
}

func uid(v int) *int { return &v }

// decodeResp 解析框架统一响应体，返回 code 字段。
func decodeResp(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

// --- 鉴权缺失：所有前台 handler 未注入 userID → 401 ---

func TestHandlersUnauthorized(t *testing.T) {
	handlers := []gin.HandlerFunc{
		GetProxyList, GetProxyOptions, TestProxy, GetProxy,
		CreateProxy, ProbeProxy, BatchImportProxies, UpdateProxy, DeleteProxy,
	}
	for _, h := range handlers {
		c, w := newCtx(http.MethodGet, "", nil, gin.Params{{Key: "id", Value: "1"}})
		h(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

// --- 无效 ID 路径参数 → 400 ---

func TestHandlersBadID(t *testing.T) {
	u := uid(userA)
	badParams := gin.Params{{Key: "id", Value: "abc"}}

	for _, h := range []gin.HandlerFunc{TestProxy, GetProxy, UpdateProxy, DeleteProxy} {
		c, w := newCtx(http.MethodPost, "{}", u, badParams)
		h(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
	// 管理侧两个无 userID 要求，但仍校验 ID
	for _, h := range []gin.HandlerFunc{AdminGetProxy, AdminDeleteProxy} {
		c, w := newCtx(http.MethodGet, "", nil, badParams)
		h(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

// --- CreateProxy：bind 错误 / 成功 ---

func TestCreateProxyHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)

	// 缺 required 字段 → 400
	c, w := newCtx(http.MethodPost, `{"name":""}`, u, nil)
	CreateProxy(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 非法 JSON → 400
	c, w = newCtx(http.MethodPost, `{bad`, u, nil)
	CreateProxy(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功
	c, w = newCtx(http.MethodPost, `{"name":"h1","host":"1.2.3.4","port":1080}`, u, nil)
	CreateProxy(c)
	require.Equal(t, http.StatusOK, w.Code)
	m := decodeResp(t, w)
	assert.EqualValues(t, 0, m["code"])
}

// --- GetProxyList / Options 成功路径 ---

func TestGetListAndOptionsHandlers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)
	_, err := ProxyService.Create(userA, sampleCreate("h", "5.5.5.5"))
	require.NoError(t, err)

	c, w := newCtx(http.MethodGet, "", u, nil)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&size=10&kw=h", nil)
	c.Set("userID", userA)
	GetProxyList(c)
	require.Equal(t, http.StatusOK, w.Code)

	c, w = newCtx(http.MethodGet, "", u, nil)
	GetProxyOptions(c)
	require.Equal(t, http.StatusOK, w.Code)
}

// --- GetProxy：存在 / 不存在(404) ---

func TestGetProxyHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)
	p, err := ProxyService.Create(userA, sampleCreate("g", "6.6.6.6"))
	require.NoError(t, err)

	c, w := newCtx(http.MethodGet, "", u, gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	GetProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 不存在 → FailErr 映射 404
	c, w = newCtx(http.MethodGet, "", u, gin.Params{{Key: "id", Value: "999999"}})
	GetProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TestProxy handler：经 fakeProber 成功写回 ---

func TestTestProxyHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	withFakeProber(t, &fakeProber{res: ProbeResult{EgressIP: "9.9.9.9", LatencyMs: 10, Country: "US"}})
	u := uid(userA)
	p, err := ProxyService.Create(userA, sampleCreate("t", "7.7.7.7"))
	require.NoError(t, err)

	c, w := newCtx(http.MethodPost, "", u, gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	TestProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 越权（不存在）→ 404
	c, w = newCtx(http.MethodPost, "", uid(userB), gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	TestProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- ProbeProxy handler：bind 错误 / 成功 / 探测失败仍 200 ---

func TestProbeProxyHandler(t *testing.T) {
	u := uid(userA)

	// bind 错误（缺 host/port）→ 400
	c, w := newCtx(http.MethodPost, `{}`, u, nil)
	ProbeProxy(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功
	withFakeProber(t, &fakeProber{res: ProbeResult{EgressIP: "1.1.1.1", LatencyMs: 5}})
	c, w = newCtx(http.MethodPost, `{"host":"1.2.3.4","port":1080}`, u, nil)
	ProbeProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 探测失败 → 仍 200，status=fail
	withFakeProber(t, &fakeProber{err: errors.New("refused")})
	c, w = newCtx(http.MethodPost, `{"host":"1.2.3.4","port":1080}`, u, nil)
	ProbeProxy(c)
	require.Equal(t, http.StatusOK, w.Code)
}

// --- BatchImportProxies handler：bind 错误 / 空列表 / 成功 ---

func TestBatchImportHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)

	c, w := newCtx(http.MethodPost, `{bad`, u, nil)
	BatchImportProxies(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 空列表 → 400
	c, w = newCtx(http.MethodPost, `{"proxies":[]}`, u, nil)
	BatchImportProxies(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功
	c, w = newCtx(http.MethodPost, `{"proxies":[{"name":"a","host":"1.1.1.1","port":1080}]}`, u, nil)
	BatchImportProxies(c)
	require.Equal(t, http.StatusOK, w.Code)
}

// --- UpdateProxy handler：bind 错误 / 成功 / 不存在 ---

func TestUpdateProxyHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)
	p, err := ProxyService.Create(userA, sampleCreate("up", "8.8.8.8"))
	require.NoError(t, err)
	idParam := gin.Params{{Key: "id", Value: itoa(int(p.ID))}}

	// bind 错误
	c, w := newCtx(http.MethodPut, `{bad`, u, idParam)
	UpdateProxy(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功
	c, w = newCtx(http.MethodPut, `{"name":"up2"}`, u, idParam)
	UpdateProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 不存在 → 404
	c, w = newCtx(http.MethodPut, `{"name":"x"}`, u, gin.Params{{Key: "id", Value: "999999"}})
	UpdateProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- DeleteProxy handler：成功 / 不存在 ---

func TestDeleteProxyHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	u := uid(userA)
	p, err := ProxyService.Create(userA, sampleCreate("d", "1.0.0.1"))
	require.NoError(t, err)

	c, w := newCtx(http.MethodDelete, "", u, gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	DeleteProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 不存在 → 404
	c, w = newCtx(http.MethodDelete, "", u, gin.Params{{Key: "id", Value: "999999"}})
	DeleteProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- 管理侧 handler：列表 / 详情 / 删除 ---

func TestAdminHandlers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })
	p, err := ProxyService.Create(userA, sampleCreate("admin-px", "2.0.0.1"))
	require.NoError(t, err)

	// 列表（带分页/过滤 query）
	c, w := newCtx(http.MethodGet, "", nil, nil)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&size=20&kw=admin&status=unknown", nil)
	AdminListProxies(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 详情存在
	c, w = newCtx(http.MethodGet, "", nil, gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	AdminGetProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 详情不存在 → 404
	c, w = newCtx(http.MethodGet, "", nil, gin.Params{{Key: "id", Value: "999999"}})
	AdminGetProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 删除成功
	c, w = newCtx(http.MethodDelete, "", nil, gin.Params{{Key: "id", Value: itoa(int(p.ID))}})
	AdminDeleteProxy(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 删除不存在 → 404
	c, w = newCtx(http.MethodDelete, "", nil, gin.Params{{Key: "id", Value: "999999"}})
	AdminDeleteProxy(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func itoa(i int) string { return strconv.Itoa(i) }
