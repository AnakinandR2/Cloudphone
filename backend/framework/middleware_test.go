package framework_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func corsRouter(allowed []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(framework.CORSMiddleware(allowed))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func reqWithOrigin(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCORSWhitelistedOriginEchoed(t *testing.T) {
	r := corsRouter([]string{"https://app.example.com"})
	w := reqWithOrigin(r, "GET", "https://app.example.com")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Get("Vary"), "Origin")
}

func TestCORSNonWhitelistedOriginBlocked(t *testing.T) {
	r := corsRouter([]string{"https://app.example.com"})
	w := reqWithOrigin(r, "GET", "https://evil.example.com")

	// 未命中白名单：不下发 CORS 头（浏览器据此拦截）
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflightReturns204(t *testing.T) {
	r := corsRouter([]string{"https://app.example.com"})
	w := reqWithOrigin(r, http.MethodOptions, "https://app.example.com")

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSEmptyAllowlistEchoesAny(t *testing.T) {
	r := corsRouter(nil) // 开发态：放行任意来源
	w := reqWithOrigin(r, "GET", "https://anything.test")

	assert.Equal(t, "https://anything.test", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSWildcardEchoesAny(t *testing.T) {
	r := corsRouter([]string{"*"})
	w := reqWithOrigin(r, "GET", "https://whatever.test")

	assert.Equal(t, "https://whatever.test", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestSecurityHeadersSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(framework.SecurityHeaders())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req, _ := http.NewRequest("GET", "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("Referrer-Policy"))
}
