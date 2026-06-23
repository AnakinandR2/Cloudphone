package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	"github.com/gin-gonic/gin"
)

// withUserID 是注入前台用户身份的测试中间件（绕过真实 user.AuthMiddleware，
// 直接预置 currentUserID 读取的 "userID"）。uid<=0 表示「不注入」用以测未授权。
func withUserID(uid int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid > 0 {
			c.Set("userID", uid)
		}
		c.Next()
	}
}

// setAppService 临时替换全局 AppService，返回还原函数。
func setAppService(t *testing.T, ops midplatPort) {
	t.Helper()
	prev := AppService
	AppService = newService(newRepository(framework.DB), ops)
	t.Cleanup(func() { AppService = prev })
}

func doReq(r *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v body=%s", err, w.Body.String())
	}
	return resp
}

// ── GetAppList ───────────────────────────────────────────────────

func TestHandler_GetAppList_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/app/list", withUserID(0), GetAppList)
	w := doReq(r, "GET", "/app/list", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_GetAppList_OK(t *testing.T) {
	const owner = 92000
	framework.DB.Create(&CustomerApp{UserID: owner, AppMD5: "h-list", Status: StatusCreating})
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/app/list", withUserID(owner), GetAppList)
	w := doReq(r, "GET", "/app/list", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeResp(t, w)
	if resp.Code != 0 {
		t.Errorf("resp code = %d", resp.Code)
	}
}

// ── GetMarket ────────────────────────────────────────────────────

func TestHandler_GetMarket_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/app/market", withUserID(0), GetMarket)
	if w := doReq(r, "GET", "/app/market", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_GetMarket_OK(t *testing.T) {
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/app/market", withUserID(92010), GetMarket)
	if w := doReq(r, "GET", "/app/market", nil); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── BatchDeleteApps ──────────────────────────────────────────────

func TestHandler_BatchDelete_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/batch-delete", withUserID(0), BatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {1}})
	if w := doReq(r, "POST", "/app/batch-delete", body); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_BatchDelete_EmptyIDs_BadRequest(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/batch-delete", withUserID(92020), BatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {}})
	w := doReq(r, "POST", "/app/batch-delete", body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty ids → want 400, got %d", w.Code)
	}
}

func TestHandler_BatchDelete_NotFound(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/batch-delete", withUserID(92021), BatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {99999990}})
	w := doReq(r, "POST", "/app/batch-delete", body)
	if w.Code != http.StatusNotFound {
		t.Errorf("not-owned → want 404, got %d", w.Code)
	}
}

func TestHandler_BatchDelete_OK(t *testing.T) {
	const owner = 92030
	rec := &CustomerApp{UserID: owner, CpAppID: 9301, AppMD5: "h-bd"}
	framework.DB.Create(rec)
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/batch-delete", withUserID(owner), BatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {int(rec.ID)}})
	w := doReq(r, "POST", "/app/batch-delete", body)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// ── Admin handlers ───────────────────────────────────────────────

func TestHandler_AdminListApps_OK(t *testing.T) {
	framework.DB.Create(&CustomerApp{UserID: 92040, AppMD5: "h-admin-list"})
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/admin/apps", AdminListApps)
	if w := doReq(r, "GET", "/admin/apps", nil); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_AdminBatchDelete_EmptyBadRequest(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/admin/apps/batch-delete", AdminBatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {}})
	if w := doReq(r, "POST", "/admin/apps/batch-delete", body); w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandler_AdminBatchDelete_OK(t *testing.T) {
	rec := &CustomerApp{UserID: 92050, CpAppID: 9501, AppMD5: "h-admin-bd"}
	framework.DB.Create(rec)
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/admin/apps/batch-delete", AdminBatchDeleteApps)
	body, _ := json.Marshal(map[string][]int{"ids": {int(rec.ID)}})
	if w := doReq(r, "POST", "/admin/apps/batch-delete", body); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_AdminStoreList_OK(t *testing.T) {
	framework.DB.Create(&CustomerApp{UserID: 0, Store: true, AppMD5: "h-store-list"})
	setAppService(t, &recordingPort{listResp: &midplat.ListAppsResponse{}})
	r := gin.New()
	r.GET("/admin/apps/store", AdminStoreList)
	if w := doReq(r, "GET", "/admin/apps/store", nil); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_AdminStoreDelete_EmptyAndOK(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/admin/apps/store/batch-delete", AdminStoreDelete)
	body, _ := json.Marshal(map[string][]int{"ids": {}})
	if w := doReq(r, "POST", "/admin/apps/store/batch-delete", body); w.Code != http.StatusBadRequest {
		t.Errorf("empty → want 400, got %d", w.Code)
	}

	rec := &CustomerApp{UserID: 0, Store: true, CpAppID: 9601, AppMD5: "h-store-del"}
	framework.DB.Create(rec)
	body, _ = json.Marshal(map[string][]int{"ids": {int(rec.ID)}})
	if w := doReq(r, "POST", "/admin/apps/store/batch-delete", body); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── upload pipeline handlers (基础校验路径) ──────────────────────

func TestHandler_InitiateUpload_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/initiate", withUserID(0), InitiateUpload)
	if w := doReq(r, "POST", "/app/upload/initiate", []byte(`{}`)); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_InitiateUpload_BadRequest(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/initiate", withUserID(92060), InitiateUpload)
	// 缺 fileName → 400
	if w := doReq(r, "POST", "/app/upload/initiate", []byte(`{"fileSize":1}`)); w.Code != http.StatusBadRequest {
		t.Errorf("missing fileName → want 400, got %d", w.Code)
	}
}

func TestHandler_InitiateUpload_OK(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/initiate", withUserID(92061), InitiateUpload)
	w := doReq(r, "POST", "/app/upload/initiate", []byte(`{"fileName":"app.apk","fileSize":100}`))
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_CompleteUpload_MissingID(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/complete", withUserID(92062), CompleteUpload)
	if w := doReq(r, "POST", "/app/upload/complete", []byte(`{}`)); w.Code != http.StatusBadRequest {
		t.Errorf("missing uploadId → want 400, got %d", w.Code)
	}
}

func TestHandler_CompleteUpload_OK(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/complete", withUserID(92063), CompleteUpload)
	if w := doReq(r, "POST", "/app/upload/complete", []byte(`{"uploadId":"123"}`)); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_ParseApp_MissingID(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/parse", withUserID(92064), ParseApp)
	if w := doReq(r, "POST", "/app/upload/parse", []byte(`{}`)); w.Code != http.StatusBadRequest {
		t.Errorf("missing uploadId → want 400, got %d", w.Code)
	}
}

func TestHandler_ParseApp_OK(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/parse", withUserID(92065), ParseApp)
	if w := doReq(r, "POST", "/app/upload/parse", []byte(`{"uploadId":"77"}`)); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadStatus_MissingID(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.GET("/app/upload/status", withUserID(92066), UploadStatus)
	if w := doReq(r, "GET", "/app/upload/status", nil); w.Code != http.StatusBadRequest {
		t.Errorf("missing uploadId → want 400, got %d", w.Code)
	}
}

func TestHandler_UploadStatus_OK(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.GET("/app/upload/status", withUserID(92067), UploadStatus)
	if w := doReq(r, "GET", "/app/upload/status?uploadId=88", nil); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_UploadPart_BadRequest(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/part", withUserID(92068), UploadPart)
	// 缺 uploadId/partNumber → 400（在取 form file 之前）
	if w := doReq(r, "POST", "/app/upload/part", nil); w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandler_UploadPart_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/part", withUserID(0), UploadPart)
	if w := doReq(r, "POST", "/app/upload/part?uploadId=1&partNumber=1", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_CreateAppFromUpload_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/create", withUserID(0), CreateAppFromUpload)
	if w := doReq(r, "POST", "/app/upload/create", []byte(`{}`)); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_CreateAppFromUpload_BadJSON(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload/create", withUserID(92070), CreateAppFromUpload)
	if w := doReq(r, "POST", "/app/upload/create", []byte(`not-json`)); w.Code != http.StatusBadRequest {
		t.Errorf("bad json → want 400, got %d", w.Code)
	}
}

func TestHandler_CreateAppFromUpload_OK(t *testing.T) {
	setAppService(t, &recordingPort{created: &midplat.CreatedApp{ID: 9701, AppName: "fromUpload", MD5: "h-cfu"}})
	r := gin.New()
	r.POST("/app/upload/create", withUserID(92071), CreateAppFromUpload)
	body := []byte(`{"uploadId":"5","appName":"x","md5":"h-cfu"}`)
	if w := doReq(r, "POST", "/app/upload/create", body); w.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// ── 上传 handler 基础校验（S3/中台未配置场景，跳过深覆盖）──────────

func TestHandler_UploadApp_Unauthorized(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload", withUserID(0), UploadApp)
	if w := doReq(r, "POST", "/app/upload", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_UploadApp_MissingFile(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/app/upload", withUserID(92080), UploadApp)
	// 无 multipart file → 400 未选择文件
	if w := doReq(r, "POST", "/app/upload", nil); w.Code != http.StatusBadRequest {
		t.Errorf("missing file → want 400, got %d", w.Code)
	}
}

func TestHandler_AdminStoreUpload_MissingFile(t *testing.T) {
	setAppService(t, &recordingPort{})
	r := gin.New()
	r.POST("/admin/apps/store/upload", AdminStoreUpload)
	if w := doReq(r, "POST", "/admin/apps/store/upload", nil); w.Code != http.StatusBadRequest {
		t.Errorf("missing file → want 400, got %d", w.Code)
	}
}

// ── currentUserID / parseUploadID 工具 ──────────────────────────

func TestParseUploadID(t *testing.T) {
	if parseUploadID("123") != 123 {
		t.Errorf("parseUploadID(123) wrong")
	}
	if parseUploadID("") != 0 {
		t.Errorf("parseUploadID empty → 0")
	}
	if parseUploadID("abc") != 0 {
		t.Errorf("parseUploadID non-numeric → 0")
	}
}

func TestCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := currentUserID(c); ok {
		t.Errorf("no userID → ok=false")
	}
	c.Set("userID", 42)
	if id, ok := currentUserID(c); !ok || id != 42 {
		t.Errorf("currentUserID = %d,%v want 42,true", id, ok)
	}
	// 非 int 类型 → ok=false
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Set("userID", "notint")
	if _, ok := currentUserID(c2); ok {
		t.Errorf("non-int userID → ok=false")
	}
}
