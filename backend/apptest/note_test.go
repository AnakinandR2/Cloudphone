package apptest

import (
	"fmt"
	"net/http"
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
