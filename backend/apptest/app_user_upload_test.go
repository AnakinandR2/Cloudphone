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

// app_user_upload_test.go 覆盖「用户上传 + finalize」链路的接口层用例（对应
// docs/superpowers/plans/2026-07-02-验收测试-planB-接口层全覆盖.md 任务 12）：
// TC-12-001（我的应用列表属主隔离）、TC-12-002（直传+finalize 解析就绪）、
// TC-12-006（finalize 属主校验）、TC-12-007（无效 fileId）、
// TC-12-009（批量删除空 file_ids 拒绝）、TC-12-010（越权批量删除）。
//
// 承接关系：TC-12-002 的「presign→PUT→confirm→finalize→ready」主干链路已由
// foundation_test.go 的 TestFoundation_AppUploadFinalizeE2E 冒烟覆盖（断言
// package_name/version/app_name/parse_status 等字段）；本文件的 TestAppUserList_OwnerIsolation
// 不重复该断言矩阵，而是把 002 并入 001 的属主隔离场景一并验证（同一次上传的产出
// 同时喂给「列表隔离」和「finalize 就绪」两个断言点），避免与 foundation 冒烟重复造数据。
// TC-12-006 的单元层「非属主」判定已由 modules/app/internal/service_test.go 的
// TestFinalizeUserApp_NotOwner 覆盖；本文件补的是 HTTP 边界的具体状态码 + 文案断言
// （404 「文件不存在」），是接口层的独立断言面，不是重复。

// appUploadReadyApp 用最小可解析 xapk 走「presign→PUT→confirm→finalize」全链路，
// 返回 finalize 后的 file_id。调用前需已 withFakeS3（公有桶传图标 + 私有桶回读素材）。
func appUploadReadyApp(t *testing.T, r *gin.Engine, token, pkg, version, name string) uint {
	t.Helper()
	apkBytes := minimalXAPK(t, pkg, version, name)
	fileID := uploadLibraryFile(t, r, token,
		name+".xapk", "application/vnd.android.package-archive", apkBytes)
	require.Greater(t, fileID, uint(0))

	fw := doJSON(r, "POST", fmt.Sprintf("/api/v1/app/user/%d/finalize", fileID), token, nil)
	require.Equal(t, http.StatusOK, fw.Code, "finalize 失败: %s", fw.Body.String())
	dto := decode(t, fw).Data.(map[string]interface{})
	require.Equal(t, "ready", dto["parse_status"], "最小可解析 xapk 应落 ready: %v", dto)
	return fileID
}

// TestAppUserList_OwnerIsolation 覆盖 TC-12-001 + TC-12-002：
// A、B 各自上传+finalize 一个应用；A 的 GET /app/user 只应看到 A 的应用（含刚 finalize 的
// package_name/version/parse_status=ready 等字段），不出现 B 的；B 侧同理镜像验证。
func TestAppUserList_OwnerIsolation(t *testing.T) {
	const phoneA = "13912000001"
	const phoneB = "13912000002"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	pubCli, _ := newFakeS3(t, "gp-pub-001", "https://cdn.test001/")
	libCli, _ := newFakeS3(t, "gp-lib-001", "")
	withFakeS3(t, pubCli, libCli)

	fileA := appUploadReadyApp(t, r, tokenA, "com.gp.usera", "1.0.0", "AppA")
	fileB := appUploadReadyApp(t, r, tokenB, "com.gp.userb", "2.0.0", "AppB")

	// A 的列表：应含 A 自己的 ready 应用（package_name/version 与刚 finalize 一致），不含 B 的。
	lwA := doJSON(r, "GET", "/api/v1/app/user", tokenA, nil)
	require.Equal(t, http.StatusOK, lwA.Code, "A 我的应用列表失败: %s", lwA.Body.String())
	appsA := decode(t, lwA).Data.([]interface{})
	require.Len(t, appsA, 1, "A 应只看到自己上传的 1 个应用")
	gotA := appsA[0].(map[string]interface{})
	assert.Equal(t, float64(fileA), gotA["file_id"])
	assert.Equal(t, "ready", gotA["parse_status"])
	assert.Equal(t, "com.gp.usera", gotA["package_name"])
	assert.Equal(t, "1.0.0", gotA["version"])
	for _, a := range appsA {
		m := a.(map[string]interface{})
		assert.NotEqual(t, float64(fileB), m["file_id"], "A 的列表不应出现 B 的应用")
	}

	// B 的列表：镜像验证，只应看到 B 自己的。
	lwB := doJSON(r, "GET", "/api/v1/app/user", tokenB, nil)
	require.Equal(t, http.StatusOK, lwB.Code, "B 我的应用列表失败: %s", lwB.Body.String())
	appsB := decode(t, lwB).Data.([]interface{})
	require.Len(t, appsB, 1, "B 应只看到自己上传的 1 个应用")
	gotB := appsB[0].(map[string]interface{})
	assert.Equal(t, float64(fileB), gotB["file_id"])
	assert.Equal(t, "ready", gotB["parse_status"])
	assert.Equal(t, "com.gp.userb", gotB["package_name"])
	for _, a := range appsB {
		m := a.(map[string]interface{})
		assert.NotEqual(t, float64(fileA), m["file_id"], "B 的列表不应出现 A 的应用")
	}
}

// TestAppUserFinalize_OwnershipAndInvalidID 覆盖 TC-12-006（finalize 属主校验）+
// TC-12-007（无效 fileId）：
//   - B 对 A 的 fileId 调 finalize → library.OpenFileContent 内部按 (userID, fileID) 查
//     文件（getFile 已按属主 scope），B 查不到该行 → apperr.NotFound「文件不存在」→ HTTP 404；
//     且不应写出 app_user_meta（不解析、不落 meta）。
//   - 传 0 / 非数字 fileId → handler 提前校验，400「无效文件 ID」。
func TestAppUserFinalize_OwnershipAndInvalidID(t *testing.T) {
	const phoneA = "13912000003"
	const phoneB = "13912000004"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	pubCli, _ := newFakeS3(t, "gp-pub-002", "https://cdn.test002/")
	libCli, _ := newFakeS3(t, "gp-lib-002", "")
	withFakeS3(t, pubCli, libCli)

	// A 上传（但不 finalize）一个合法 xapk，只到 confirm 步骤（active 状态），供 B 越权 finalize 用。
	apkBytes := minimalXAPK(t, "com.gp.ownercheck", "1.0.0", "OwnerCheckApp")
	fileA := uploadLibraryFile(t, r, tokenA, "ownercheck.xapk", "application/vnd.android.package-archive", apkBytes)
	require.Greater(t, fileA, uint(0))

	// B 对 A 的 fileId 调 finalize：应 404 且文案含「文件不存在」（library 属主校验透传）。
	fw := doJSON(r, "POST", fmt.Sprintf("/api/v1/app/user/%d/finalize", fileA), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, fw.Code, "B 对 A 的文件 finalize 应 404: %s", fw.Body.String())
	assert.Contains(t, decode(t, fw).Message, "文件不存在", "越权 finalize 应透传属主校验文案")

	// 越权 finalize 不应写出 app_user_meta（不解析、不落 meta）。
	countAfterOwnershipDenied := appCountUserMeta(t, fileA)
	assert.Equal(t, int64(0), countAfterOwnershipDenied, "越权 finalize 被拒绝后不应写出 app_user_meta")

	// A 自己 finalize 应成功（佐证上面 404 是属主拒绝而非文件本身有问题）。
	fwOwner := doJSON(r, "POST", fmt.Sprintf("/api/v1/app/user/%d/finalize", fileA), tokenA, nil)
	require.Equal(t, http.StatusOK, fwOwner.Code, "属主自己 finalize 应成功: %s", fwOwner.Body.String())

	// 无效 fileId：非数字。
	fwBad := doJSON(r, "POST", "/api/v1/app/user/abc/finalize", tokenA, nil)
	assert.Equal(t, http.StatusBadRequest, fwBad.Code, "非数字 fileId 应 400")
	assert.Contains(t, decode(t, fwBad).Message, "无效文件 ID")

	// 无效 fileId：0。
	fwZero := doJSON(r, "POST", "/api/v1/app/user/0/finalize", tokenA, nil)
	assert.Equal(t, http.StatusBadRequest, fwZero.Code, "fileId=0 应 400")
	assert.Contains(t, decode(t, fwZero).Message, "无效文件 ID")
}

// TestAppUserBatchDelete_EmptyRejected 覆盖 TC-12-009：批量删除我的应用，
// 空 file_ids（含未传字段，绑定为 nil 切片）一律 400「未选择应用」，且不影响该用户已有应用。
func TestAppUserBatchDelete_EmptyRejected(t *testing.T) {
	const phone = "13912000005"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	pubCli, _ := newFakeS3(t, "gp-pub-003", "https://cdn.test003/")
	libCli, _ := newFakeS3(t, "gp-lib-003", "")
	withFakeS3(t, pubCli, libCli)

	fileID := appUploadReadyApp(t, r, token, "com.gp.keepme", "1.0.0", "KeepMeApp")

	// 显式空数组。
	w1 := doJSON(r, "POST", "/api/v1/app/user/batch-delete", token, map[string]interface{}{
		"file_ids": []uint{},
	})
	assert.Equal(t, http.StatusBadRequest, w1.Code, "空 file_ids 应 400")
	assert.Contains(t, decode(t, w1).Message, "未选择应用")

	// 完全不传 file_ids 字段（绑定为零值 nil 切片），同样应拒绝。
	w2 := doJSON(r, "POST", "/api/v1/app/user/batch-delete", token, map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, w2.Code, "未传 file_ids 应 400")
	assert.Contains(t, decode(t, w2).Message, "未选择应用")

	// 两次都被拒绝后，该用户的应用应仍在（未被误删）。
	lw := doJSON(r, "GET", "/api/v1/app/user", token, nil)
	require.Equal(t, http.StatusOK, lw.Code)
	apps := decode(t, lw).Data.([]interface{})
	found := false
	for _, a := range apps {
		if a.(map[string]interface{})["file_id"] == float64(fileID) {
			found = true
		}
	}
	assert.True(t, found, "被拒绝的空批量删除不应影响已有应用")
}

// TestAppUserBatchDelete_CrossOwnerRejected 覆盖 TC-12-010：A 用 B 的 file_id 调
// batch-delete，library.DeleteFileForUser 内部属主校验（getFile 按 (userID, fileID) scope）
// 应拒绝，B 的应用不会被删掉；A 自己的应用不受影响。
func TestAppUserBatchDelete_CrossOwnerRejected(t *testing.T) {
	const phoneA = "13912000006"
	const phoneB = "13912000007"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	pubCli, _ := newFakeS3(t, "gp-pub-004", "https://cdn.test004/")
	libCli, _ := newFakeS3(t, "gp-lib-004", "")
	withFakeS3(t, pubCli, libCli)

	fileA := appUploadReadyApp(t, r, tokenA, "com.gp.mine", "1.0.0", "MineApp")
	fileB := appUploadReadyApp(t, r, tokenB, "com.gp.notyours", "1.0.0", "NotYoursApp")

	// A 尝试删 B 的 file_id：应被属主校验拒绝（404「文件不存在」），B 的应用不受影响。
	w := doJSON(r, "POST", "/api/v1/app/user/batch-delete", tokenA, map[string]interface{}{
		"file_ids": []uint{fileB},
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "A 删 B 的应用应 404: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "文件不存在", "越权批量删除应透传属主校验文案")

	// B 的应用仍在。
	lwB := doJSON(r, "GET", "/api/v1/app/user", tokenB, nil)
	require.Equal(t, http.StatusOK, lwB.Code)
	appsB := decode(t, lwB).Data.([]interface{})
	foundB := false
	for _, a := range appsB {
		if a.(map[string]interface{})["file_id"] == float64(fileB) {
			foundB = true
		}
	}
	assert.True(t, foundB, "A 越权删除失败后 B 的应用应仍在")

	// A 自己的应用不受影响（仍在）。
	lwA := doJSON(r, "GET", "/api/v1/app/user", tokenA, nil)
	require.Equal(t, http.StatusOK, lwA.Code)
	appsA := decode(t, lwA).Data.([]interface{})
	foundA := false
	for _, a := range appsA {
		if a.(map[string]interface{})["file_id"] == float64(fileA) {
			foundA = true
		}
	}
	assert.True(t, foundA, "A 自己的应用不应因越权尝试而受影响")

	// A 删自己的 fileA 应成功（佐证越权是属主拒绝，而非批量删除整体失效）。
	wOwn := doJSON(r, "POST", "/api/v1/app/user/batch-delete", tokenA, map[string]interface{}{
		"file_ids": []uint{fileA},
	})
	assert.Equal(t, http.StatusOK, wOwn.Code, "A 删自己的应用应成功: %s", wOwn.Body.String())
}

// appCountUserMeta 统计某 library_file_id 是否已落 app_user_meta 行（0/1），
// 用于断言越权 finalize 被拒绝后不写出旁挂元数据。
func appCountUserMeta(t *testing.T, fileID uint) int64 {
	t.Helper()
	var n int64
	require.NoError(t,
		framework.DB.Table("app_user_meta").Where("library_file_id = ?", fileID).Count(&n).Error)
	return n
}
