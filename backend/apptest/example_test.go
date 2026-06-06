package apptest

import (
	"fmt"
	"net/http"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// example 是后台专用模块：所有接口都需 staff 登录 + 权限。

func TestExampleListRequiresAuth(t *testing.T) {
	r := setupRouter()
	// 未登录 → 401
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/example/list", "", nil).Code)
}

func TestExampleListWithAdmin(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })
	r := setupRouter()
	token := adminToken(t, r)

	w := doJSON(r, "GET", "/api/v1/example/list", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)
}

func TestExampleGetWithAdmin(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })
	r := setupRouter()
	token := adminToken(t, r)

	id := createExample(t, r, token, "仅后台可见", "x")

	// 未登录读详情 → 401
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", fmt.Sprintf("/api/v1/example/%d", id), "", nil).Code)

	// 管理员读详情 → 200
	w := doJSON(r, "GET", fmt.Sprintf("/api/v1/example/%d", id), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "仅后台可见", decode(t, w).Data.(map[string]interface{})["title"])

	// 管理员读不存在 → 404
	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", "/api/v1/example/999999", token, nil).Code)
}

func TestExampleCRUDWithAuth(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	r := setupRouter()
	token := adminToken(t, r)

	id := createExample(t, r, token, "新建", "通过API创建")

	w := doJSON(r, "PUT", fmt.Sprintf("/api/v1/example/update/%d", id), token, map[string]string{
		"title": "已更新", "content": "内容已更新",
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "已更新", decode(t, w).Data.(map[string]interface{})["title"])

	w = doJSON(r, "DELETE", fmt.Sprintf("/api/v1/example/delete/%d", id), token, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = doJSON(r, "GET", fmt.Sprintf("/api/v1/example/%d", id), token, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExampleWritesRequireAuth(t *testing.T) {
	r := setupRouter()
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "POST", "/api/v1/example/create", "", map[string]string{"title": "t", "content": "c"}).Code)
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "PUT", "/api/v1/example/update/1", "", map[string]string{"title": "t"}).Code)
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "DELETE", "/api/v1/example/delete/1", "", nil).Code)
}

// 前台令牌、以及无 example 权限的普通后台用户，都不能访问 example。
func TestExampleForbiddenForUserAndUnpermittedStaff(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items"); framework.CleanTable("users") })
	r := setupRouter()
	admin := adminToken(t, r)

	// 前台 user 令牌 → 身份域不匹配 → 401
	userToken := registerUser(t, r, "13900008888")
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/example/list", userToken, nil).Code)

	// 普通后台用户（无 example:view）→ 403
	createUser(t, r, admin, "ex_noperm", "pass123", false)
	staffToken := login(t, r, "ex_noperm", "pass123")
	assert.Equal(t, http.StatusForbidden, doJSON(r, "GET", "/api/v1/example/list", staffToken, nil).Code)
}

func TestExampleListPagination(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	r := setupRouter()
	token := adminToken(t, r)
	for i := 0; i < 12; i++ {
		createExample(t, r, token, fmt.Sprintf("item_%d", i), "c")
	}

	w := doJSON(r, "GET", "/api/v1/example/list?page=1&size=5", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, float64(12), data["total"])
	assert.Len(t, data["list"].([]interface{}), 5)

	w = doJSON(r, "GET", "/api/v1/example/list?page=3&size=5", token, nil)
	data = decode(t, w).Data.(map[string]interface{})
	assert.Len(t, data["list"].([]interface{}), 2)
}
