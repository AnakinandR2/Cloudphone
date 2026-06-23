package openapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userMW = 8201

// 用中间件 + 一个回显 userID 的终端 handler 跑一次完整请求。
func runWithKeyAuth(t *testing.T, setHeader func(*http.Request)) (*httptest.ResponseRecorder, int) {
	t.Helper()
	r := gin.New()
	var seenUID int
	r.GET("/probe", keyAuthMiddleware(), func(c *gin.Context) {
		if v, ok := c.Get("userID"); ok {
			seenUID = v.(int)
		}
		framework.OK(c)
	})
	req := httptest.NewRequest("GET", "/probe", nil)
	if setHeader != nil {
		setHeader(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, seenUID
}

// 缺密钥 → 401 且终端 handler 未执行。
func TestKeyAuthMissing(t *testing.T) {
	w, uid := runWithKeyAuth(t, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, 0, uid)
}

// 无效密钥 → 401。
func TestKeyAuthInvalid(t *testing.T) {
	w, uid := runWithKeyAuth(t, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer gp_live_deadbeefdeadbeef")
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, 0, uid)
}

// 已撤销密钥 → 401。
func TestKeyAuthRevoked(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })
	res, err := Service.Create(userMW, "撤销测试")
	require.NoError(t, err)
	require.NoError(t, Service.Revoke(userMW, res.ID))

	w, uid := runWithKeyAuth(t, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+res.FullKey)
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, 0, uid)
}

// 有效密钥（Bearer）→ 200，userID 注入正确。
func TestKeyAuthBearerOK(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })
	res, err := Service.Create(userMW, "bearer")
	require.NoError(t, err)

	w, uid := runWithKeyAuth(t, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+res.FullKey)
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userMW, uid)
}

// 有效密钥（X-API-Key 回退）→ 200。
func TestKeyAuthXAPIKeyOK(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })
	res, err := Service.Create(userMW, "xapikey")
	require.NoError(t, err)

	w, uid := runWithKeyAuth(t, func(req *http.Request) {
		req.Header.Set("X-API-Key", res.FullKey)
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userMW, uid)
}

// bearerToken 解析多种格式。
func TestBearerToken(t *testing.T) {
	mk := func(set func(*http.Request)) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("GET", "/x", nil)
		set(req)
		c.Request = req
		return bearerToken(c)
	}

	// Bearer 前缀。
	assert.Equal(t, "abc123", mk(func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer abc123")
	}))
	// 裸 Authorization（无 Bearer 前缀）→ 原样返回（trim）。
	assert.Equal(t, "rawkey", mk(func(r *http.Request) {
		r.Header.Set("Authorization", "  rawkey  ")
	}))
	// 无 Authorization → 回退 X-API-Key。
	assert.Equal(t, "fromheader", mk(func(r *http.Request) {
		r.Header.Set("X-API-Key", " fromheader ")
	}))
	// 两者皆无 → 空。
	assert.Equal(t, "", mk(func(r *http.Request) {}))
}
