package staff_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 改密后旧令牌应立即失效（令牌版本号比对）。
func TestChangePasswordInvalidatesOldToken(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u := createTestUser(t, "pwdtok", "old123", false)
	token := getToken(t, u.ID, u.Username) // 版本 0，对应新建用户的 token_version=0

	r := setupRouter()

	// 改密前：令牌可用
	require.Equal(t, http.StatusOK, do(r, "GET", "/api/v1/staff/auth/me", token, nil).Code)

	// 改密（递增令牌版本）
	body, _ := json.Marshal(staff.ChangePasswordRequest{OldPassword: "old123", NewPassword: "new456"})
	require.Equal(t, http.StatusOK, do(r, "POST", "/api/v1/staff/auth/change-password", token, body).Code)

	// 改密后：同一旧令牌失效
	assert.Equal(t, http.StatusUnauthorized, do(r, "GET", "/api/v1/staff/auth/me", token, nil).Code)

	// 用新密码重新登录可获得可用的新令牌
	login, _ := json.Marshal(staff.LoginRequest{Account: "pwdtok", Password: "new456"})
	lw := do(r, "POST", "/api/v1/staff/auth/login", "", login)
	require.Equal(t, http.StatusOK, lw.Code)
	var resp framework.Response
	json.Unmarshal(lw.Body.Bytes(), &resp)
	newToken := resp.Data.(map[string]interface{})["token"].(string)
	assert.Equal(t, http.StatusOK, do(r, "GET", "/api/v1/staff/auth/me", newToken, nil).Code)
}

func do(r http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	var rd *bytes.Buffer = bytes.NewBuffer(body)
	req, _ := http.NewRequest(method, path, rd)
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
