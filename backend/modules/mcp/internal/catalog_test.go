package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCatalogMatchesRegisteredTools 漂移守卫：分组清单的 code 集合必须与
// 实际注册的全部工具一一对应（数量 + 名字），防止新增/改名工具忘了进分组。
func TestToolCatalogMatchesRegisteredTools(t *testing.T) {
	cat := toolCatalog()

	// 分组顺序固定且各组非空。
	require.Len(t, cat, 4)
	assert.Equal(t, "phone", cat[0].Group)
	assert.Equal(t, "app", cat[1].Group)
	assert.Equal(t, "script", cat[2].Group)
	assert.Equal(t, "proxy", cat[3].Group)
	for _, g := range cat {
		assert.NotEmptyf(t, g.Tools, "分组 %s 不应为空", g.Group)
	}

	// 展开 catalog 的 code 集合，顺带查重。
	got := map[string]bool{}
	total := 0
	for _, g := range cat {
		for _, name := range g.Tools {
			assert.Falsef(t, got[name], "工具 %s 在 catalog 中重复", name)
			got[name] = true
			total++
		}
	}

	// 与 allTools() 注册集合逐一匹配。
	want := map[string]bool{}
	for _, rt := range allTools() {
		want[rt.tool.Name] = true
	}
	assert.Equal(t, len(want), total, "catalog 工具数应等于注册工具数")
	assert.Equal(t, want, got, "catalog 与注册工具集合应完全一致")
}

// TestToolsEndpoint 免鉴权 GET /api/v1/mcp/tools 返回 4 组、18 个 code、信封正确。
func TestToolsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiV1 := r.Group("/api/v1")
	(&mcpModule{}).RegisterRoutes(apiV1)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    []toolGroupView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 4)

	total := 0
	for _, grp := range resp.Data {
		total += len(grp.Tools)
	}
	assert.Equal(t, 18, total)
}

// TestToolsEndpointIgnoresInjectedAuth 守卫「免鉴权」不变量：SetupRouter 会把 auth 中间件
// 作为变参传给各模块的 RegisterRoutes（见 framework/router.go），本模块须忽略它——
// 这里传入一个会对所有请求 401 的中间件，断言 /mcp/tools 仍 200。一旦有人改成对该路由
// 应用注入的中间件，此测试即失败。
func TestToolsEndpointIgnoresInjectedAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiV1 := r.Group("/api/v1")
	denyAll := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	(&mcpModule{}).RegisterRoutes(apiV1, denyAll)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "工具清单应免鉴权：模块须忽略注入的 auth 中间件")
}
