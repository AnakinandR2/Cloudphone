package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const testSecret = "test-secret-key"

// Generate/Parse 应能往返还原全部声明字段。
func TestGenerateParseRoundTrip(t *testing.T) {
	tok, err := Generate(42, "alice", ScopeUser, 7, testSecret, 1)
	assert.NoError(t, err)
	assert.NotEmpty(t, tok)

	claims, err := Parse(tok, testSecret)
	assert.NoError(t, err)
	assert.Equal(t, 42, claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, ScopeUser, claims.Scope)
	assert.Equal(t, 7, claims.TokenVersion)
}

func TestParseWrongSecret(t *testing.T) {
	tok, _ := Generate(1, "bob", ScopeStaff, 0, testSecret, 1)
	_, err := Parse(tok, "other-secret")
	assert.Error(t, err, "密钥不匹配应验签失败")
}

func TestParseExpired(t *testing.T) {
	// expireHours 为负 → 立即过期。
	tok, _ := Generate(1, "bob", ScopeStaff, 0, testSecret, -1)
	_, err := Parse(tok, testSecret)
	assert.Error(t, err)
}

func TestParseGarbage(t *testing.T) {
	_, err := Parse("not.a.jwt", testSecret)
	assert.Error(t, err)
}

func newRouter(cfg MiddlewareConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/p", cfg.Middleware(), func(c *gin.Context) {
		uid, _ := c.Get("userID")
		scope, _ := c.Get("scope")
		c.JSON(http.StatusOK, gin.H{"uid": uid, "scope": scope})
	})
	return r
}

func doGet(r *gin.Engine, build func(*http.Request)) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/p", nil)
	if build != nil {
		build(req)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestMiddlewareNoToken(t *testing.T) {
	r := newRouter(MiddlewareConfig{Secret: testSecret})
	w := doGet(r, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddlewareValidBearer(t *testing.T) {
	tok, _ := Generate(9, "u", ScopeUser, 0, testSecret, 1)
	r := newRouter(MiddlewareConfig{Secret: testSecret, Scope: ScopeUser})
	w := doGet(r, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"uid":9`, "userID 应写入上下文")
}

func TestMiddlewareBadAuthHeader(t *testing.T) {
	r := newRouter(MiddlewareConfig{Secret: testSecret})
	w := doGet(r, func(req *http.Request) {
		req.Header.Set("Authorization", "Token abc") // 非 Bearer
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddlewareCookieFallback(t *testing.T) {
	tok, _ := Generate(5, "c", ScopeUser, 0, testSecret, 1)
	r := newRouter(MiddlewareConfig{Secret: testSecret, CookieName: "tk"})
	w := doGet(r, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: "tk", Value: tok})
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddlewareScopeMismatch(t *testing.T) {
	tok, _ := Generate(1, "u", ScopeUser, 0, testSecret, 1)
	r := newRouter(MiddlewareConfig{Secret: testSecret, Scope: ScopeStaff})
	w := doGet(r, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code, "作用域不匹配应拒绝")
}

func TestMiddlewareValidateReject(t *testing.T) {
	tok, _ := Generate(1, "u", ScopeUser, 3, testSecret, 1)
	cfg := MiddlewareConfig{Secret: testSecret, Validate: func(*Claims) error {
		return errors.New("登录态已失效") // 模拟 token 版本不符
	}}
	r := newRouter(cfg)
	w := doGet(r, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+tok)
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
