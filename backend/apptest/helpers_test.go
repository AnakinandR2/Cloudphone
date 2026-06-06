package apptest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	framework.SetupRouter(r)
	return r
}

// doJSON 发起一个 JSON 请求（可选 Bearer 令牌），返回响应记录器。
func doJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// decode 解析统一响应体。
func decode(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// login 通过公开登录接口换取令牌。
func login(t *testing.T, r *gin.Engine, account, password string) string {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/staff/auth/login", "", map[string]string{
		"account": account, "password": password,
	})
	require.Equal(t, http.StatusOK, w.Code, "登录失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	return data["token"].(string)
}

// adminToken 登录内置超管（迁移 v1 预置 admin/admin123）。
func adminToken(t *testing.T, r *gin.Engine) string {
	return login(t, r, "admin", "admin123")
}

// createUser 以超管令牌经公开接口创建用户。
func createUser(t *testing.T, r *gin.Engine, adminTok, username, password string, superuser bool) {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/staff/create", adminTok, map[string]interface{}{
		"username":     username,
		"password":     password,
		"is_active":    true,
		"is_superuser": superuser,
	})
	require.Equal(t, http.StatusOK, w.Code, "创建用户失败: %s", w.Body.String())
}

// createExample 以超管令牌创建一条示例数据，返回其 ID。
func createExample(t *testing.T, r *gin.Engine, adminTok, title, content string) int {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/example/create", adminTok, map[string]string{
		"title": title, "content": content,
	})
	require.Equal(t, http.StatusOK, w.Code, "创建示例失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	return int(data["id"].(float64))
}
