package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frontRouter 仅挂载前台用户路由（register/login/me/logout），
// 用 user 包自身的 UserAuth 中间件，不依赖 staff 模块。
func frontRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	(&userModule{}).RegisterRoutes(api)
	return r
}

func doJSON(r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, _ := http.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func TestRegisterHandlerSuccess(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := frontRouter()

	w := doJSON(r, "POST", "/api/v1/user/auth/register",
		"", RegisterRequest{Phone: "13811110001", Password: "pass123", Nickname: "阿明"})
	require.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	assert.Equal(t, 0, resp.Code)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "13811110001", data["phone"])
	assert.Equal(t, "阿明", data["nickname"])
	assert.NotEmpty(t, data["token"])
}

func TestRegisterHandlerBadBody(t *testing.T) {
	r := frontRouter()
	// 缺少必填字段 → 400
	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{"nickname": "x"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegisterHandlerInvalidPhone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := frontRouter()
	// service 层校验失败 → 422 (Validation)
	w := doJSON(r, "POST", "/api/v1/user/auth/register",
		"", RegisterRequest{Phone: "123", Password: "pass123"})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, decode(t, w).Message, "手机号")
}

func TestLoginHandlerSuccessAndWrong(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := frontRouter()
	_, err := Service.Register(&RegisterRequest{Phone: "13811110002", Password: "secret1"})
	require.NoError(t, err)

	// 成功
	w := doJSON(r, "POST", "/api/v1/user/auth/login",
		"", LoginRequest{Phone: "13811110002", Password: "secret1"})
	require.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w).Data.(map[string]interface{})
	assert.NotEmpty(t, data["token"])

	// 密码错误 → 401
	w = doJSON(r, "POST", "/api/v1/user/auth/login",
		"", LoginRequest{Phone: "13811110002", Password: "wrong"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginHandlerBadBody(t *testing.T) {
	r := frontRouter()
	w := doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{"phone": "1"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 端到端：注册拿令牌 → /me 通过 UserAuth 中间件 → logout 递增版本 → 旧令牌失效
func TestProfileLogoutFlowThroughMiddleware(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := frontRouter()

	w := doJSON(r, "POST", "/api/v1/user/auth/register",
		"", RegisterRequest{Phone: "13811110003", Password: "pass123", Nickname: "甲"})
	require.Equal(t, http.StatusOK, w.Code)
	token := decode(t, w).Data.(map[string]interface{})["token"].(string)
	require.NotEmpty(t, token)

	// 携带令牌访问 /me
	w = doJSON(r, "GET", "/api/v1/user/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	me := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, "13811110003", me["phone"])

	// 登出（递增令牌版本）
	w = doJSON(r, "POST", "/api/v1/user/auth/logout", token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	// 旧令牌应失效（token version mismatch）→ 401
	w = doJSON(r, "GET", "/api/v1/user/me", token, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMeWithoutToken(t *testing.T) {
	r := frontRouter()
	w := doJSON(r, "GET", "/api/v1/user/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMeWithGarbageToken(t *testing.T) {
	r := frontRouter()
	w := doJSON(r, "GET", "/api/v1/user/me", "not-a-real-jwt", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// GetProfile 直接调用：用户被删除后 GetByID 失败 → FailErr 404
func TestGetProfileUserGone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", 987654)
	GetProfile(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// GetProfile：context 无 userID → 401
func TestGetProfileNoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	GetProfile(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Logout 直接调用：context 无 userID 时只清 cookie 返回 OK（不报错）
func TestLogoutHandlerNoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Logout(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- 管理侧 handler（直接构造 context，跳过 staff 权限中间件，仍计入覆盖） ---

func adminCtx(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, target, nil)
	c.Request = req
	return c, w
}

func TestAdminListUsersHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	_, err := Service.Register(&RegisterRequest{Phone: "13822220001", Password: "pass123"})
	require.NoError(t, err)
	_, err = Service.Register(&RegisterRequest{Phone: "13922220002", Password: "pass123"})
	require.NoError(t, err)

	c, w := adminCtx("GET", "/admin/users/list?page=1&size=10&phone=138&is_active=true")
	AdminListUsers(c)
	require.Equal(t, http.StatusOK, w.Code)
	page := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, float64(1), page["total"])
}

func TestAdminListUsersInactiveFilter(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	u, err := Service.Register(&RegisterRequest{Phone: "13833330001", Password: "pass123"})
	require.NoError(t, err)
	_, err = Service.SetActive(u.ID, false)
	require.NoError(t, err)

	c, w := adminCtx("GET", "/admin/users/list?is_active=false")
	AdminListUsers(c)
	require.Equal(t, http.StatusOK, w.Code)
	page := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, float64(1), page["total"])
}

func TestAdminGetUserHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	u, err := Service.Register(&RegisterRequest{Phone: "13844440001", Password: "pass123"})
	require.NoError(t, err)

	c, w := adminCtx("GET", "/admin/users/"+fmt.Sprint(u.ID))
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(u.ID)}}
	AdminGetUser(c)
	require.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, "13844440001", data["phone"])
}

func TestAdminGetUserBadID(t *testing.T) {
	c, w := adminCtx("GET", "/admin/users/abc")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	AdminGetUser(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminGetUserNotFound(t *testing.T) {
	c, w := adminCtx("GET", "/admin/users/987654")
	c.Params = gin.Params{{Key: "id", Value: "987654"}}
	AdminGetUser(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminSetUserStatusHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	u, err := Service.Register(&RegisterRequest{Phone: "13855550001", Password: "pass123"})
	require.NoError(t, err)

	body, _ := json.Marshal(AdminStatusRequest{IsActive: boolPtr(false)})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("PUT", "/admin/users/x/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(u.ID)}}
	AdminSetUserStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	data := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, false, data["is_active"])
}

func TestAdminSetUserStatusBadID(t *testing.T) {
	c, w := adminCtx("PUT", "/admin/users/abc/status")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	AdminSetUserStatus(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminSetUserStatusBadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("PUT", "/admin/users/1/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	AdminSetUserStatus(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminSetUserStatusNotFound(t *testing.T) {
	body, _ := json.Marshal(AdminStatusRequest{IsActive: boolPtr(true)})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("PUT", "/admin/users/987654/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "987654"}}
	AdminSetUserStatus(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func boolPtr(b bool) *bool { return &b }
