package apptest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerUser 注册一个前台用户并返回其令牌。
func registerUser(t *testing.T, r *gin.Engine, phone string) string {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": phone, "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	return decode(t, w).Data.(map[string]interface{})["token"].(string)
}

// 前台「我的笔记」：登录用户可对自己的笔记增删改查，且看不到/动不了别人的。
func TestNoteOwnerIsolationHTTP(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes"); framework.CleanTable("users") })
	r := setupRouter()

	tokenA := registerUser(t, r, "13900000001")
	tokenB := registerUser(t, r, "13900000002")

	// 未登录不可访问
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/note/list", "", nil).Code)

	// A 新建一条笔记
	w := doJSON(r, "POST", "/api/v1/note/create", tokenA, map[string]string{"title": "A的笔记", "content": "hello"})
	require.Equal(t, http.StatusOK, w.Code)
	noteID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))

	// A 能在自己列表里看到
	data := decode(t, doJSON(r, "GET", "/api/v1/note/list", tokenA, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])

	// B 的列表为空，且拿不到 A 的详情 / 改不动 / 删不掉（均 404）
	dataB := decode(t, doJSON(r, "GET", "/api/v1/note/list", tokenB, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(0), dataB["total"])
	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", fmt.Sprintf("/api/v1/note/%d", noteID), tokenB, nil).Code)
	assert.Equal(t, http.StatusNotFound,
		doJSON(r, "PUT", fmt.Sprintf("/api/v1/note/update/%d", noteID), tokenB, map[string]string{"title": "篡改"}).Code)
	assert.Equal(t, http.StatusNotFound,
		doJSON(r, "DELETE", fmt.Sprintf("/api/v1/note/delete/%d", noteID), tokenB, nil).Code)

	// A 能更新与删除自己的
	w = doJSON(r, "PUT", fmt.Sprintf("/api/v1/note/update/%d", noteID), tokenA, map[string]string{"title": "改过了"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "改过了", decode(t, w).Data.(map[string]interface{})["title"])

	require.Equal(t, http.StatusOK, doJSON(r, "DELETE", fmt.Sprintf("/api/v1/note/delete/%d", noteID), tokenA, nil).Code)
	assert.Equal(t, http.StatusNotFound, doJSON(r, "GET", fmt.Sprintf("/api/v1/note/%d", noteID), tokenA, nil).Code)
}

// 分页 + 标题搜索 + 排序：用唯一手机号造数据，不截断共享表。
func TestNoteListPagingSearchSortHTTP(t *testing.T) {
	r := setupRouter()
	token := registerUser(t, r, "13900010001")

	// 造 5 条笔记，标题带可检索关键字。
	titles := []string{"alpha-001", "alpha-002", "alpha-003", "beta-004", "alpha-005"}
	ids := make([]int, 0, len(titles))
	for _, ti := range titles {
		w := doJSON(r, "POST", "/api/v1/note/create", token, map[string]string{"title": ti, "content": "x"})
		require.Equal(t, http.StatusOK, w.Code)
		ids = append(ids, int(decode(t, w).Data.(map[string]interface{})["id"].(float64)))
	}

	// 全量 total=5
	data := decode(t, doJSON(r, "GET", "/api/v1/note/list", token, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(5), data["total"])

	// 标题模糊搜索 alpha → 4 条
	data = decode(t, doJSON(r, "GET", "/api/v1/note/list?title=alpha", token, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(4), data["total"])
	assert.Len(t, data["list"].([]interface{}), 4)

	// 分页：size=2&page=1 → total 仍为 5，但本页只 2 条
	data = decode(t, doJSON(r, "GET", "/api/v1/note/list?page=1&size=2", token, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(5), data["total"])
	assert.Len(t, data["list"].([]interface{}), 2)

	// 排序：order=id&sort=ascending → 第一条应是最早创建的 id
	data = decode(t, doJSON(r, "GET", "/api/v1/note/list?order=id&sort=ascending", token, nil)).Data.(map[string]interface{})
	first := data["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, float64(ids[0]), first["id"])

	// 非法排序字段被白名单忽略（不应 500），仍 200
	assert.Equal(t, http.StatusOK,
		doJSON(r, "GET", "/api/v1/note/list?order=content;DROP+TABLE&sort=ascending", token, nil).Code)
}

// 非法请求：坏 ID（非数字）→ 400；坏 JSON 体 → 400；空标题（缺 required）→ 400。
func TestNoteInvalidRequestsHTTP(t *testing.T) {
	r := setupRouter()
	token := registerUser(t, r, "13900010002")

	// 坏 ID（非数字路径参数）→ 400
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "GET", "/api/v1/note/abc", token, nil).Code)

	// 坏 JSON 体 → 400
	req, _ := http.NewRequest("POST", "/api/v1/note/create", strings.NewReader(`{"title": `))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 缺 required 的 title → 400
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "POST", "/api/v1/note/create", token, map[string]string{"content": "无标题"}).Code)

	// 空内容是允许的（content 非必填）：title 有值即可创建
	w := doJSON(r, "POST", "/api/v1/note/create", token, map[string]string{"title": "仅标题", "content": ""})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", decode(t, w).Data.(map[string]interface{})["content"])
}
