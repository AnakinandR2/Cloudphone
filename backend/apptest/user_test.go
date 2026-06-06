package apptest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 管理侧（staff）对前台用户的查看/禁用：禁用即强制下线、且无法再登录。
func TestAdminUserManagement(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()
	admin := adminToken(t, r)

	reg := decode(t, doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13811112222", "password": "pass123", "nickname": "小测",
	})).Data.(map[string]interface{})
	userID := int(reg["id"].(float64))
	userToken := reg["token"].(string)

	// 列表（按手机号过滤）含该用户
	data := decode(t, doJSON(r, "GET", "/api/v1/admin/users/list?phone=13811112222", admin, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])

	// 详情
	w := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/users/%d", userID), admin, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "13811112222", decode(t, w).Data.(map[string]interface{})["phone"])

	// 前台用户自身不能访问管理侧（身份域不匹配）
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "GET", "/api/v1/admin/users/list", userToken, nil).Code)

	// 禁用
	require.Equal(t, http.StatusOK,
		doJSON(r, "PUT", fmt.Sprintf("/api/v1/admin/users/%d/status", userID), admin, map[string]bool{"is_active": false}).Code)

	// 禁用后：旧令牌失效 + 无法登录
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/user/me", userToken, nil).Code)
	assert.Equal(t, http.StatusForbidden, doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{
		"phone": "13811112222", "password": "pass123",
	}).Code)

	// 重新启用后可登录
	require.Equal(t, http.StatusOK,
		doJSON(r, "PUT", fmt.Sprintf("/api/v1/admin/users/%d/status", userID), admin, map[string]bool{"is_active": true}).Code)
	assert.Equal(t, http.StatusOK, doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{
		"phone": "13811112222", "password": "pass123",
	}).Code)
}

// 无 user:view 权限的普通后台用户访问管理侧 → 403。
func TestAdminUserForbiddenForNormalStaff(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	createUser(t, r, admin, "user_noperm", "pass123", false)
	token := login(t, r, "user_noperm", "pass123")
	assert.Equal(t, http.StatusForbidden,
		doJSON(r, "GET", "/api/v1/admin/users/list", token, nil).Code)
}

func TestUserRegisterAndLogin(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()

	// 注册（直接返回令牌）
	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13800138001", "password": "pass123", "nickname": "小明",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	assert.NotEmpty(t, data["token"])

	// 登录
	w = doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{
		"phone": "13800138001", "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	token := decode(t, w).Data.(map[string]interface{})["token"].(string)

	// 带 user 令牌访问 /me
	w = doJSON(r, "GET", "/api/v1/user/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "13800138001", decode(t, w).Data.(map[string]interface{})["phone"])
}

func TestUserRegisterValidation(t *testing.T) {
	r := setupRouter()
	// 手机号格式错误 → 422
	assert.Equal(t, http.StatusUnprocessableEntity,
		doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{"phone": "123", "password": "pass123"}).Code)
	// 密码过短 → 422
	assert.Equal(t, http.StatusUnprocessableEntity,
		doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{"phone": "13812345670", "password": "123"}).Code)
}

func TestUserLoginWrongPassword(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()

	doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13800138002", "password": "right1",
	})
	w := doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{
		"phone": "13800138002", "password": "wrong",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserMeRequiresAuth(t *testing.T) {
	r := setupRouter()
	w := doJSON(r, "GET", "/api/v1/user/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// 登录会下发 Cookie，且仅凭 Cookie（无 Authorization 头）即可通过鉴权。
func TestUserCookieAuth(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()

	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13800138009", "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var jwtCookie *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == framework.AppConfig.UserJWTCookieName {
			jwtCookie = ck
		}
	}
	require.NotNil(t, jwtCookie, "登录应下发前台 Cookie")
	assert.True(t, jwtCookie.HttpOnly)

	// 仅带 Cookie、不带 Authorization 头访问 /me
	req, _ := http.NewRequest("GET", "/api/v1/user/me", nil)
	req.AddCookie(jwtCookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "13800138009", decode(t, rec).Data.(map[string]interface{})["phone"])
}

func TestUserLogoutClearsCookie(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()

	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13800138010", "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	token := decode(t, w).Data.(map[string]interface{})["token"].(string)

	// 登出需登录态
	un := doJSON(r, "POST", "/api/v1/user/auth/logout", "", nil)
	assert.Equal(t, http.StatusUnauthorized, un.Code)

	// 带令牌登出 → 200 且下发一个立即过期的清除 Cookie
	out := doJSON(r, "POST", "/api/v1/user/auth/logout", token, nil)
	require.Equal(t, http.StatusOK, out.Code)

	var cleared *http.Cookie
	for _, ck := range out.Result().Cookies() {
		if ck.Name == framework.AppConfig.UserJWTCookieName {
			cleared = ck
		}
	}
	require.NotNil(t, cleared, "登出应下发清除 Cookie")
	assert.Empty(t, cleared.Value)
	assert.True(t, cleared.MaxAge < 0, "Cookie 应立即过期")

	// 令牌版本号方案：登出递增版本 → 旧令牌（即便走 Authorization 头）服务端立即失效
	me := doJSON(r, "GET", "/api/v1/user/me", token, nil)
	assert.Equal(t, http.StatusUnauthorized, me.Code, "登出后旧令牌应失效")
}

// 前后台令牌互不通用 —— 身份域隔离的核心验证。

func TestStaffTokenRejectedOnUserRoute(t *testing.T) {
	r := setupRouter()
	staff := adminToken(t, r) // scope=staff
	w := doJSON(r, "GET", "/api/v1/user/me", staff, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserTokenRejectedOnStaffRoute(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()

	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": "13800138003", "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code)
	userToken := decode(t, w).Data.(map[string]interface{})["token"].(string)

	// 用前台令牌访问后台用户列表 → 应 401（身份域不匹配）
	w = doJSON(r, "GET", "/api/v1/staff/list", userToken, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
