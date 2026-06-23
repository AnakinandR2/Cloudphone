package partner

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCtx 构造一个带 ResponseRecorder 的 gin.Context，供直接调用 handler（计入覆盖率）。
func newCtx(method, target string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	c.Request = r
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// --- admin handler 直调 ---

func TestCreatePartnerHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })

	body, _ := json.Marshal(PartnerCreate{Name: "h-create", PromoURL: "https://x.com"})
	c, w := newCtx("POST", "/admin/partners", body)
	CreatePartner(c)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	assert.Equal(t, 0, resp.Code)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "h-create", data["name"])
	assert.Equal(t, true, data["enabled"])
}

func TestCreatePartnerHandlerBadJSON(t *testing.T) {
	c, w := newCtx("POST", "/admin/partners", []byte("{not-json"))
	CreatePartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePartnerHandlerMissingRequired(t *testing.T) {
	// 缺 name（binding:required）→ 400
	body, _ := json.Marshal(map[string]string{"promo_url": "https://x.com"})
	c, w := newCtx("POST", "/admin/partners", body)
	CreatePartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminListPartnersHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	_, _ = PartnerService.Create(sampleCreate("al-1", 1, true))
	_, _ = PartnerService.Create(sampleCreate("al-2", 2, false)) // 禁用项也应在 admin 列表

	c, w := newCtx("GET", "/admin/partners?page=1&size=10&kw=al-", nil)
	AdminListPartners(c)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	assert.Equal(t, 0, resp.Code)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])
}

func TestAdminListPartnersHandlerWithOrder(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	_, _ = PartnerService.Create(sampleCreate("ord-b", 2, true))
	_, _ = PartnerService.Create(sampleCreate("ord-a", 1, true))

	// SafeOrder 仅认 sort=descending 为降序，其余按升序。
	c, w := newCtx("GET", "/admin/partners?order=name&sort=descending&kw=ord-", nil)
	AdminListPartners(c)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	data := resp.Data.(map[string]interface{})
	list := data["list"].([]interface{})
	require.Len(t, list, 2)
	assert.Equal(t, "ord-b", list[0].(map[string]interface{})["name"]) // name descending
}

func TestUpdatePartnerHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	p, _ := PartnerService.Create(sampleCreate("u-orig", 0, true))

	body, _ := json.Marshal(PartnerUpdate{Name: "u-new", Enabled: ptrBool(false)})
	c, w := newCtx("PUT", "/admin/partners/1", body)
	c.Params = gin.Params{{Key: "id", Value: itoa(int(p.ID))}}
	UpdatePartner(c)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "u-new", data["name"])
	assert.Equal(t, false, data["enabled"])
}

func TestUpdatePartnerHandlerBadID(t *testing.T) {
	body, _ := json.Marshal(PartnerUpdate{Name: "x"})
	c, w := newCtx("PUT", "/admin/partners/abc", body)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	UpdatePartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePartnerHandlerBadJSON(t *testing.T) {
	c, w := newCtx("PUT", "/admin/partners/1", []byte("{bad"))
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	UpdatePartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePartnerHandlerNotFound(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	body, _ := json.Marshal(PartnerUpdate{Name: "x"})
	c, w := newCtx("PUT", "/admin/partners/987654", body)
	c.Params = gin.Params{{Key: "id", Value: "987654"}}
	UpdatePartner(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeletePartnerHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	p, _ := PartnerService.Create(sampleCreate("d-1", 0, true))

	c, w := newCtx("DELETE", "/admin/partners/1", nil)
	c.Params = gin.Params{{Key: "id", Value: itoa(int(p.ID))}}
	DeletePartner(c)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err := PartnerService.GetByID(int(p.ID))
	assert.Error(t, err)
}

func TestDeletePartnerHandlerBadID(t *testing.T) {
	c, w := newCtx("DELETE", "/admin/partners/x", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	DeletePartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeletePartnerHandlerNotFound(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	c, w := newCtx("DELETE", "/admin/partners/55555", nil)
	c.Params = gin.Params{{Key: "id", Value: "55555"}}
	DeletePartner(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminListClicksHandler(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})
	p, _ := PartnerService.Create(sampleCreate("clk", 0, true))
	id := int(p.ID)
	_, _ = PartnerService.RecordClick(id, &PartnerClick{Channel: "my"})
	_, _ = PartnerService.RecordClick(id, &PartnerClick{Channel: "www"})

	c, w := newCtx("GET", "/admin/partners/1/clicks?page=1&size=10", nil)
	c.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	AdminListClicks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])
}

func TestAdminListClicksHandlerBadID(t *testing.T) {
	c, w := newCtx("GET", "/admin/partners/x/clicks", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	AdminListClicks(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- 公开 handler ---

func TestListPartnersHandler(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })
	_, _ = PartnerService.Create(sampleCreate("pub-a", 1, true))
	_, _ = PartnerService.Create(sampleCreate("pub-hidden", 0, false))

	c, w := newCtx("GET", "/partner/list", nil)
	ListPartners(c)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	list := resp.Data.([]interface{})
	require.Len(t, list, 1) // 仅启用项
	assert.Equal(t, "pub-a", list[0].(map[string]interface{})["name"])
}

func TestClickPartnerHandlerAnonymous(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})
	p, _ := PartnerService.Create(sampleCreate("cph", 0, true))
	id := int(p.ID)

	body, _ := json.Marshal(ClickRequest{Channel: "www", AnonymousID: "a1", UTMSource: "ad"})
	c, w := newCtx("POST", "/partner/1/click", body)
	c.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c.Request.Header.Set("User-Agent", "test-ua")
	c.Request.Header.Set("Referer", "https://ref")
	ClickPartner(c)

	assert.Equal(t, http.StatusOK, w.Code)
	_, total, _ := PartnerService.ListClicks(id, 1, 10)
	assert.Equal(t, int64(1), total)
}

func TestClickPartnerHandlerBadID(t *testing.T) {
	c, w := newCtx("POST", "/partner/x/click", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	ClickPartner(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 点击 body 解析失败（软标识可缺省）→ 仍返回成功，不阻断。
func TestClickPartnerHandlerBadBodyTolerant(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})
	p, _ := PartnerService.Create(sampleCreate("cph-bad", 0, true))
	id := int(p.ID)

	c, w := newCtx("POST", "/partner/1/click", []byte("{garbage"))
	c.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	ClickPartner(c)
	assert.Equal(t, http.StatusOK, w.Code) // 解析失败按空处理，不报错

	_, total, _ := PartnerService.ListClicks(id, 1, 10)
	assert.Equal(t, int64(1), total)
}

// 不存在合作商点击：静默成功（不阻断前端跳转）。
func TestClickPartnerHandlerMissingPartnerStillOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partner_clicks") })
	c, w := newCtx("POST", "/partner/1/click", nil)
	c.Params = gin.Params{{Key: "id", Value: "888888"}}
	ClickPartner(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- optionalUserID 各分支 ---

func userToken(t *testing.T, uid int, secret string) string {
	t.Helper()
	tok, err := auth.Generate(uid, "u", auth.ScopeUser, 0, secret, 1)
	require.NoError(t, err)
	return tok
}

func TestOptionalUserIDAnonymous(t *testing.T) {
	c, _ := newCtx("POST", "/partner/1/click", nil)
	assert.Equal(t, 0, optionalUserID(c))
}

func TestOptionalUserIDBearer(t *testing.T) {
	secret := framework.AppConfig.UserJWTSecret
	if secret == "" {
		secret = framework.AppConfig.JWTSecret
	}
	c, _ := newCtx("POST", "/partner/1/click", nil)
	c.Request.Header.Set("Authorization", "Bearer "+userToken(t, 77, secret))
	assert.Equal(t, 77, optionalUserID(c))
}

func TestOptionalUserIDInvalidToken(t *testing.T) {
	c, _ := newCtx("POST", "/partner/1/click", nil)
	c.Request.Header.Set("Authorization", "Bearer not.a.jwt")
	assert.Equal(t, 0, optionalUserID(c))
}

func TestOptionalUserIDWrongScope(t *testing.T) {
	// staff scope 令牌不应被识别为 user
	secret := framework.AppConfig.UserJWTSecret
	if secret == "" {
		secret = framework.AppConfig.JWTSecret
	}
	tok, _ := auth.Generate(5, "s", auth.ScopeStaff, 0, secret, 1)
	c, _ := newCtx("POST", "/partner/1/click", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tok)
	assert.Equal(t, 0, optionalUserID(c))
}

func TestOptionalUserIDCookieFallback(t *testing.T) {
	name := framework.AppConfig.UserJWTCookieName
	if name == "" {
		t.Skip("UserJWTCookieName 未配置，跳过 Cookie 回退分支")
	}
	secret := framework.AppConfig.UserJWTSecret
	if secret == "" {
		secret = framework.AppConfig.JWTSecret
	}
	c, _ := newCtx("POST", "/partner/1/click", nil)
	c.Request.AddCookie(&http.Cookie{Name: name, Value: userToken(t, 88, secret)})
	assert.Equal(t, 88, optionalUserID(c))
}

// ClickPartner 携带有效 user 令牌：UserID 被归因写入明细。
func TestClickPartnerHandlerLoggedIn(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})
	p, _ := PartnerService.Create(sampleCreate("cph-login", 0, true))
	id := int(p.ID)
	secret := framework.AppConfig.UserJWTSecret
	if secret == "" {
		secret = framework.AppConfig.JWTSecret
	}

	c, w := newCtx("POST", "/partner/1/click", nil)
	c.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c.Request.Header.Set("Authorization", "Bearer "+userToken(t, 321, secret))
	ClickPartner(c)
	assert.Equal(t, http.StatusOK, w.Code)

	clicks, _, _ := PartnerService.ListClicks(id, 1, 10)
	require.Len(t, clicks, 1)
	assert.Equal(t, uint(321), clicks[0].UserID)
}

// --- UploadPartnerImage 基础校验（S3 全局变量，跳过深覆盖）---

func multipartImage(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, _ = fw.Write(data)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestUploadPartnerImageNoS3(t *testing.T) {
	// 默认测试环境 framework.S3 为 nil → 503
	if framework.S3 != nil {
		t.Skip("S3 已配置，跳过未配置分支")
	}
	c, w := newCtx("POST", "/admin/partners/upload", nil)
	UploadPartnerImage(c)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUploadPartnerImageMissingFile(t *testing.T) {
	if framework.S3 == nil {
		t.Skip("S3 未配置，缺文件分支被 503 短路")
	}
	c, w := newCtx("POST", "/admin/partners/upload", nil)
	UploadPartnerImage(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadPartnerImageBadExt(t *testing.T) {
	if framework.S3 == nil {
		t.Skip("S3 未配置，扩展名校验分支被 503 短路")
	}
	buf, ct := multipartImage(t, "file", "evil.exe", []byte("x"))
	c, w := newCtx("POST", "/admin/partners/upload", nil)
	c.Request = httptest.NewRequest("POST", "/admin/partners/upload", buf)
	c.Request.Header.Set("Content-Type", ct)
	UploadPartnerImage(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// itoa 避免引入 strconv 仅为一个用途。
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
