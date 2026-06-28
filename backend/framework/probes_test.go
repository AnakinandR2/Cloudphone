package framework

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// newProbeEngine 构造一个仅挂运维探针的引擎；带 Recovery 以覆盖 /debug/panic。
func newProbeEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	RegisterProbes(r)
	return r
}

func doProbe(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// TestLiveness 存活探针不碰依赖，恒 200。
func TestLiveness(t *testing.T) {
	prev := DB
	DB = nil // 即便 DB 未初始化，livez 也必须 200。
	t.Cleanup(func() { DB = prev })

	w := doProbe(newProbeEngine(), "/livez")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

// TestReadinessNoDB DB 未初始化时就绪探针返回 503。
func TestReadinessNoDB(t *testing.T) {
	prev := DB
	DB = nil
	t.Cleanup(func() { DB = prev })

	for _, path := range []string{"/readyz", "/health"} {
		w := doProbe(newProbeEngine(), path)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, path)
	}
}

// TestReadinessWithDB 主库可 ping 时就绪探针 200，且 /health 别名同语义。
func TestReadinessWithDB(t *testing.T) {
	if dbTypeForTest() != "sqlite" {
		t.Skip("仅 sqlite 免外部依赖跑该用例")
	}
	dir := t.TempDir()
	withSQLiteDB(t, filepath.Join(dir, "probe.db"), func(_ *gorm.DB) {
		r := newProbeEngine()
		for _, path := range []string{"/readyz", "/health"} {
			w := doProbe(r, path)
			assert.Equal(t, http.StatusOK, w.Code, path)
			assert.Contains(t, w.Body.String(), `"status":"ok"`, path)
		}
	})
}

// TestDebugStatus 返回路径中的状态码；非法码 400。
func TestDebugStatus(t *testing.T) {
	r := newProbeEngine()
	assert.Equal(t, http.StatusTeapot, doProbe(r, "/debug/status/418").Code)
	assert.Equal(t, http.StatusBadRequest, doProbe(r, "/debug/status/abc").Code)
	assert.Equal(t, http.StatusBadRequest, doProbe(r, "/debug/status/99").Code)
}

// TestDebugPanic 制造 panic 由 Recovery 兜成 500（验证异常捕获链路）。
func TestDebugPanic(t *testing.T) {
	w := doProbe(newProbeEngine(), "/debug/panic")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
