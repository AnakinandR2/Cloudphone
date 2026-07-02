package apptest

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// app_market_test.go 覆盖应用市场（TC-12-030~037）：admin 上传（含解析失败仍落行）、
// 权限/参数校验、admin 列表含未就绪、批量删除、用户端市场只见 ready 且对全体公有。
// 对应计划文档《2026-07-02-验收测试-planB-接口层全覆盖.md》任务 13。
// Plan A（modules/app/internal/service_test.go）只在 service 单元层测了 FinalizeUserApp/
// ResolveInstallSpecs，未覆盖 market 上传/列表/批量删除的 HTTP 层，故本文件为新增覆盖，非重复。

// marketMultipartUpload 以 multipart/form-data 上传一个 "file" 字段到市场上传接口，
// 用于覆盖 AdminMarketUpload（表单字段名固定为 "file"，见 modules/app/internal/api.go）。
func marketMultipartUpload(t *testing.T, r *gin.Engine, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := mw.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, "/api/v1/admin/apps/market/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// marketMultipartNoFile 发起一个不带任何表单字段的 multipart 请求（未选择文件场景）。
func marketMultipartNoFile(t *testing.T, r *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, "/api/v1/admin/apps/market/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// marketStaffWithPerms 建一个非超管 staff（唯一用户名），赋一个仅含 perms 的角色，返回其令牌。
// 返回的 cleanup 需由调用方 t.Cleanup 注册。
func marketStaffWithPerms(t *testing.T, r *gin.Engine, adminTok, username string, perms []string) (token string, cleanup func()) {
	t.Helper()
	createUser(t, r, adminTok, username, "pass123", false)
	staffID := staffIDByUsername(t, username)
	roleName := "market_role_" + username
	roleID := createRole(t, r, adminTok, roleName, perms)
	assignRoles(t, r, adminTok, staffID, []int{roleID})
	tok := login(t, r, username, "pass123")
	return tok, func() {
		framework.DB.Exec("DELETE FROM staff_roles WHERE staff_id = ?", staffID)
		framework.DB.Exec("DELETE FROM roles WHERE id = ?", roleID)
		framework.DB.Exec("DELETE FROM staff WHERE id = ?", staffID)
	}
}

// findMarketDTOByID 在 market 列表 Data([]interface{}) 中按 id 查找条目。
func findMarketDTOByID(list []interface{}, id float64) map[string]interface{} {
	for _, it := range list {
		m := it.(map[string]interface{})
		if m["id"] == id {
			return m
		}
	}
	return nil
}

// TestAppMarket_UploadReady 覆盖 TC-12-030：staff 持 app:manage，multipart 上传合法 xapk
// → parse_status=ready，落 app_market 行，公有桶收到二进制对象（S3Key 非空且可读回）。
func TestAppMarket_UploadReady(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, pubStore := newFakeS3(t, "gp-pub-market-030", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_up_030", []string{"app:manage"})
	t.Cleanup(cleanup)

	const wantPkg, wantVer, wantName = "com.gp.market030", "1.0.30", "Market030"
	pkgBytes := minimalXAPK(t, wantPkg, wantVer, wantName)

	w := marketMultipartUpload(t, r, tok, "market030.xapk", pkgBytes)
	require.Equal(t, http.StatusOK, w.Code, "市场上传应成功: %s", w.Body.String())
	dto := decode(t, w).Data.(map[string]interface{})
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM app_market WHERE id = ?", uint(dto["id"].(float64)))
	})

	assert.Equal(t, "ready", dto["parse_status"], "合法 xapk 上传应落 ready: %v", dto)
	assert.Equal(t, wantPkg, dto["package_name"])
	assert.Equal(t, wantVer, dto["version"])
	assert.Equal(t, wantName, dto["app_name"])
	assert.Empty(t, dto["parse_error"])

	// 落库应有非空 S3Key，且公有桶确实收到了该对象（按 uuid 前缀 app-market/ 定位）。
	var s3Key string
	require.NoError(t, framework.DB.Table("app_market").
		Where("id = ?", uint(dto["id"].(float64))).Select("s3_key").Scan(&s3Key).Error)
	require.NotEmpty(t, s3Key)
	assert.Contains(t, s3Key, "app-market/")
	_, ok := pubStore.get(s3Key)
	assert.True(t, ok, "公有桶应收到上传的市场应用二进制对象")
}

// TestAppMarket_UploadParseFailedStillPersists 覆盖 TC-12-031：损坏包仍落一行 parse_status=failed，
// S3Key=parse-failed/<uuid> 占位（避免撞唯一索引），且不上传二进制到公有桶。
func TestAppMarket_UploadParseFailedStillPersists(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, pubStore := newFakeS3(t, "gp-pub-market-031", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_up_031", []string{"app:manage"})
	t.Cleanup(cleanup)

	// 扩展名合法但内容损坏 → apkparse.Parse 应失败。
	broken := []byte("this is not a valid apk package content")
	w := marketMultipartUpload(t, r, tok, "broken031.apk", broken)
	require.Equal(t, http.StatusOK, w.Code, "损坏包上传仍应 200 且落 failed 行: %s", w.Body.String())
	dto := decode(t, w).Data.(map[string]interface{})
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM app_market WHERE id = ?", uint(dto["id"].(float64)))
	})

	assert.Equal(t, "failed", dto["parse_status"], "损坏包应落 failed: %v", dto)
	assert.NotEmpty(t, dto["parse_error"], "failed 行应带 parse_error")

	var s3Key string
	require.NoError(t, framework.DB.Table("app_market").
		Where("id = ?", uint(dto["id"].(float64))).Select("s3_key").Scan(&s3Key).Error)
	require.Contains(t, s3Key, "parse-failed/", "失败行应用占位 S3Key 避免撞唯一索引")

	// 占位键不是真实上传的对象，公有桶不应收到它。
	_, ok := pubStore.get(s3Key)
	assert.False(t, ok, "解析失败不应上传二进制到公有桶")
}

// TestAppMarket_UploadForbiddenWithoutManage 覆盖 TC-12-032：仅持 app:view 不能上传 → 403。
func TestAppMarket_UploadForbiddenWithoutManage(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, _ := newFakeS3(t, "gp-pub-market-032", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_up_032", []string{"app:view"})
	t.Cleanup(cleanup)

	pkgBytes := minimalXAPK(t, "com.gp.market032", "1.0.0", "Market032")
	w := marketMultipartUpload(t, r, tok, "market032.xapk", pkgBytes)
	require.Equal(t, http.StatusForbidden, w.Code, "仅持 app:view 上传应 403: %s", w.Body.String())
}

// TestAppMarket_UploadMissingFile 覆盖 TC-12-033：未选文件 → 400「未选择文件」。
func TestAppMarket_UploadMissingFile(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, _ := newFakeS3(t, "gp-pub-market-033", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_up_033", []string{"app:manage"})
	t.Cleanup(cleanup)

	w := marketMultipartNoFile(t, r, tok)
	require.Equal(t, http.StatusBadRequest, w.Code, "未选择文件应 400: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "未选择文件")
}

// TestAppMarket_UploadUnsupportedExtension 补充校验：非 apk/xapk 扩展名 → 400「仅支持 apk / xapk」
// （TC-12-030 表述的另一半断言：非 apk/xapk 扩展名 400）。
func TestAppMarket_UploadUnsupportedExtension(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, _ := newFakeS3(t, "gp-pub-market-030ext", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_up_030ext", []string{"app:manage"})
	t.Cleanup(cleanup)

	w := marketMultipartUpload(t, r, tok, "notanapp.txt", []byte("hello"))
	require.Equal(t, http.StatusBadRequest, w.Code, "非 apk/xapk 扩展名应 400: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "仅支持 apk / xapk")
}

// TestAppMarket_AdminListIncludesFailed 覆盖 TC-12-034：admin 市场列表含未就绪（failed）行，
// 供运营维护；ListMarket(false) 不做 ready 过滤。
func TestAppMarket_AdminListIncludesFailed(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, _ := newFakeS3(t, "gp-pub-market-034", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_list_034", []string{"app:manage", "app:view"})
	t.Cleanup(cleanup)

	readyBytes := minimalXAPK(t, "com.gp.market034ready", "1.0.0", "Market034Ready")
	wReady := marketMultipartUpload(t, r, tok, "market034ready.xapk", readyBytes)
	require.Equal(t, http.StatusOK, wReady.Code)
	readyDTO := decode(t, wReady).Data.(map[string]interface{})
	readyID := uint(readyDTO["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", readyID) })

	wFailed := marketMultipartUpload(t, r, tok, "market034failed.apk", []byte("garbage not an apk"))
	require.Equal(t, http.StatusOK, wFailed.Code)
	failedDTO := decode(t, wFailed).Data.(map[string]interface{})
	failedID := uint(failedDTO["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", failedID) })

	lw := doJSON(r, "GET", "/api/v1/admin/apps/market", tok, nil)
	require.Equal(t, http.StatusOK, lw.Code, "admin 市场列表失败: %s", lw.Body.String())
	list := decode(t, lw).Data.([]interface{})

	got := findMarketDTOByID(list, float64(readyID))
	require.NotNil(t, got, "admin 列表应包含 ready 行")
	assert.Equal(t, "ready", got["parse_status"])

	gotFailed := findMarketDTOByID(list, float64(failedID))
	require.NotNil(t, gotFailed, "admin 列表应包含 failed 行（供运营维护）")
	assert.Equal(t, "failed", gotFailed["parse_status"])
}

// TestAppMarket_BatchDelete 覆盖 TC-12-035：admin 批量删除 → 删公有桶对象（S3Key 非空）+ 删行；
// 空 ids 返回 400「未选择应用」。
func TestAppMarket_BatchDelete(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	pubCli, pubStore := newFakeS3(t, "gp-pub-market-035", "https://cdn.test/")
	withFakeS3(t, pubCli, nil)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_del_035", []string{"app:manage"})
	t.Cleanup(cleanup)

	pkgBytes := minimalXAPK(t, "com.gp.market035", "1.0.0", "Market035")
	uw := marketMultipartUpload(t, r, tok, "market035.xapk", pkgBytes)
	require.Equal(t, http.StatusOK, uw.Code)
	dto := decode(t, uw).Data.(map[string]interface{})
	id := uint(dto["id"].(float64))
	// 防御性清理：happy-path 靠下方删除接口清行，但若中途 require 失败中止，此行会泄漏进共享 DB。
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", id) })

	var s3Key string
	require.NoError(t, framework.DB.Table("app_market").Where("id = ?", id).Select("s3_key").Scan(&s3Key).Error)
	require.NotEmpty(t, s3Key)
	_, existsBefore := pubStore.get(s3Key)
	require.True(t, existsBefore, "上传后公有桶应有该对象")

	// 空 ids 拒绝。
	ew := doJSON(r, "POST", "/api/v1/admin/apps/market/batch-delete", tok, map[string]interface{}{
		"ids": []uint{},
	})
	require.Equal(t, http.StatusBadRequest, ew.Code, "空 ids 应 400: %s", ew.Body.String())
	assert.Contains(t, decode(t, ew).Message, "未选择应用")

	// 真删除：应删掉公有桶对象 + 行。
	dw := doJSON(r, "POST", "/api/v1/admin/apps/market/batch-delete", tok, map[string]interface{}{
		"ids": []uint{id},
	})
	require.Equal(t, http.StatusOK, dw.Code, "批量删除失败: %s", dw.Body.String())

	var cnt int64
	require.NoError(t, framework.DB.Table("app_market").Where("id = ?", id).Count(&cnt).Error)
	assert.Zero(t, cnt, "删除后行应消失")

	_, existsAfter := pubStore.get(s3Key)
	assert.False(t, existsAfter, "删除后公有桶对象应被移除")
}

// TestAppMarket_UserListOnlyReadyAndPublicToAll 覆盖 TC-12-036/037：用户端市场只见 ready
// （failed/parsing 不出现），且市场对全体用户公有 —— A、B 两个不同前台用户看到同一份 ready 列表。
func TestAppMarket_UserListOnlyReadyAndPublicToAll(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	// 用户端最后还会打一次「我的应用」（library.ListUsableFiles），需要私有桶也配置好，否则 503。
	pubCli, _ := newFakeS3(t, "gp-pub-market-036", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-lib-market-036", "")
	withFakeS3(t, pubCli, libCli)

	tok, cleanup := marketStaffWithPerms(t, r, admin, "market_user_036", []string{"app:manage"})
	t.Cleanup(cleanup)

	readyBytes := minimalXAPK(t, "com.gp.market036ready", "1.0.0", "Market036Ready")
	wReady := marketMultipartUpload(t, r, tok, "market036ready.xapk", readyBytes)
	require.Equal(t, http.StatusOK, wReady.Code)
	readyDTO := decode(t, wReady).Data.(map[string]interface{})
	readyID := uint(readyDTO["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", readyID) })

	wFailed := marketMultipartUpload(t, r, tok, "market036failed.apk", []byte("garbage not an apk"))
	require.Equal(t, http.StatusOK, wFailed.Code)
	failedDTO := decode(t, wFailed).Data.(map[string]interface{})
	failedID := uint(failedDTO["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", failedID) })

	const phoneA, phoneB = "13913000030", "13913000031"
	tokA := registerUser(t, r, phoneA)
	uidA := userIDByPhone(t, phoneA)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	tokB := registerUser(t, r, phoneB)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	for _, tok := range []string{tokA, tokB} {
		mw := doJSON(r, "GET", "/api/v1/app/market", tok, nil)
		require.Equal(t, http.StatusOK, mw.Code, "用户端市场列表失败: %s", mw.Body.String())
		list := decode(t, mw).Data.([]interface{})

		got := findMarketDTOByID(list, float64(readyID))
		require.NotNil(t, got, "用户端应看到 ready 应用")
		assert.Equal(t, "ready", got["parse_status"])

		gotFailed := findMarketDTOByID(list, float64(failedID))
		assert.Nil(t, gotFailed, "用户端不应看到 failed 应用")
	}

	// 两人看到的是同一份公有市场（非属主隔离）：都能看到同一个 ready ID，且条目内容一致。
	mwA := doJSON(r, "GET", "/api/v1/app/market", tokA, nil)
	require.Equal(t, http.StatusOK, mwA.Code)
	listA := decode(t, mwA).Data.([]interface{})
	mwB := doJSON(r, "GET", "/api/v1/app/market", tokB, nil)
	require.Equal(t, http.StatusOK, mwB.Code)
	listB := decode(t, mwB).Data.([]interface{})
	assert.Equal(t, len(listA), len(listB), "A/B 两人看到的市场应用数量应一致（公有）")

	gotA := findMarketDTOByID(listA, float64(readyID))
	gotB := findMarketDTOByID(listB, float64(readyID))
	require.NotNil(t, gotA)
	require.NotNil(t, gotB)
	assert.Equal(t, gotA["app_name"], gotB["app_name"], "A/B 看到的同一市场应用信息应一致")
	assert.Equal(t, gotA["package_name"], gotB["package_name"])

	// 与「我的应用」互不混列：A 的「我的应用」列表不应出现市场应用（不同来源，不同接口/表）。
	uw := doJSON(r, "GET", "/api/v1/app/user", tokA, nil)
	require.Equal(t, http.StatusOK, uw.Code)
	userApps := decode(t, uw).Data.([]interface{})
	assert.Empty(t, userApps, "A 未上传过我的应用，市场应用不应混入我的应用列表")
}
