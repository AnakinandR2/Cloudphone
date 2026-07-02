package apptest

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foundation_test.go 是 apptest 的共享测试基础设施：只经公开 HTTP API + framework 门面，
// 不 import 任何模块 internal。fake S3 相关（newFakeS3/fakeS3Store/withFakeS3）在 s3fake_test.go。

// userIDByPhone 由手机号解析前台用户自增 ID（apptest 不能 import user internal，走 framework.DB 原生查 users 表）。
func userIDByPhone(t *testing.T, phone string) uint {
	t.Helper()
	var id uint
	require.NoError(t,
		framework.DB.Table("users").Where("phone = ?", phone).Select("id").Scan(&id).Error)
	require.NotZero(t, id, "应能查到手机号 %s 对应的用户 ID", phone)
	return id
}

// uploadLibraryFile 执行完整 presign → HTTP PUT 到 upload_url → confirm 链路，返回 file_id。
//
// 前置条件：调用前测试已经 withFakeS3 装好私有桶（framework.S3Library）。
// 正确计算全量内容 md5 传给 presign，使 confirm 的 ETag/md5 权威校验通过；slice_md5 留空 →
// 秒传查重不命中（PresignUpload 仅在 blob.slice_md5 非空且与请求一致时才秒传）。
// 若首次相同内容已被去重收编为 blob，presign 仍不会秒传（slice_md5 空），会走真实 PUT+confirm
// 命中「合并进既有 blob」分支，同样返回一个 active 的 file_id。
func uploadLibraryFile(t *testing.T, r *gin.Engine, token, name, mime string, content []byte) uint {
	t.Helper()

	sum := md5.Sum(content)
	contentMD5 := hex.EncodeToString(sum[:])

	// 1) presign。
	pw := doJSON(r, "POST", "/api/v1/library/upload/presign", token, map[string]interface{}{
		"name":       name,
		"size_bytes": len(content),
		"mime":       mime,
		"folder_id":  0,
		"md5":        contentMD5,
		// slice_md5 故意留空 → 不触发秒传（走真实 PUT + confirm 链路）。
	})
	require.Equal(t, http.StatusOK, pw.Code, "presign 失败: %s", pw.Body.String())
	pres := decode(t, pw).Data.(map[string]interface{})
	fileID := uint(pres["file_id"].(float64))
	require.NotZero(t, fileID, "presign 应返回 file_id")

	// 秒传命中（instant=true）：文件已 active，无 upload_url，直接返回。
	if instant, _ := pres["instant"].(bool); instant {
		return fileID
	}

	// 2) HTTP PUT 到 presigned upload_url（真实打 fake S3 端点）。
	uploadURL, _ := pres["upload_url"].(string)
	require.NotEmpty(t, uploadURL, "非秒传应返回 upload_url")
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "PUT 到 upload_url 失败")
	_ = putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode, "PUT 到 upload_url 应 200")

	// 3) confirm（Head 取真实大小 + ETag，权威校验 md5/大小）。
	cw := doJSON(r, "POST", "/api/v1/library/upload/confirm", token, map[string]interface{}{
		"file_id": fileID,
	})
	require.Equal(t, http.StatusOK, cw.Code, "confirm 失败: %s", cw.Body.String())
	f := decode(t, cw).Data.(map[string]interface{})
	require.Equal(t, float64(fileID), f["id"], "confirm 应回该文件")
	require.Equal(t, "active", f["status"], "confirm 后文件应 active")
	return fileID
}

// cleanupUserAndBillingByID 精确删除某前台用户在各域产生的自造数据 + 用户本身，绝不 CleanTable
// 截断共享表（seed 有 admin/角色/定价）。删除范围：library 文件/夹/标签及关联、app 旁挂元数据、
// billing 订单/流水/授权单元/账户、users 自身。按 user_id 精确删，幂等（表不存在或无行都安全）。
func cleanupUserAndBillingByID(t *testing.T, uid uint) {
	t.Helper()
	db := framework.DB
	// library：文件标签关联须先按属主文件删，再删文件/夹/标签。
	db.Exec("DELETE FROM library_file_tags WHERE file_id IN (SELECT id FROM library_files WHERE user_id = ?)", uid)
	db.Exec("DELETE FROM library_files WHERE user_id = ?", uid)
	db.Exec("DELETE FROM library_folders WHERE user_id = ?", uid)
	db.Exec("DELETE FROM library_tags WHERE user_id = ?", uid)
	db.Exec("DELETE FROM library_subscriptions WHERE user_id = ?", uid)
	db.Exec("DELETE FROM library_usage WHERE user_id = ?", uid)
	// app：用户应用旁挂元数据。
	db.Exec("DELETE FROM app_user_meta WHERE user_id = ?", uid)
	// billing：订单项 → 订单 → 流水/授权单元/账户。
	db.Exec("DELETE FROM billing_biz_order_items WHERE order_id IN (SELECT id FROM billing_biz_orders WHERE user_id = ?)", uid)
	db.Exec("DELETE FROM billing_biz_orders WHERE user_id = ?", uid)
	db.Exec("DELETE FROM billing_ledger_entries WHERE user_id = ?", uid)
	db.Exec("DELETE FROM billing_license_units WHERE user_id = ?", uid)
	db.Exec("DELETE FROM billing_accounts WHERE user_id = ?", uid)
	// 用户自身。
	db.Exec("DELETE FROM users WHERE id = ?", uid)
}

// tinyPNG 生成一段最小合法 PNG 字节（2x2 纯色），供 library 上传冒烟用。
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 200, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// minimalXAPK 构造一段最小可解析的 .xapk 字节：本质是内含 manifest.json + icon.png 的 zip。
// apkparse.Parse(xapk) 只需 manifest.json 即可解出 package_name/version_name/name（icon 缺失容忍），
// 故这是最省成本的「真实可解析安装包」，能让 finalize 落 parse_status=ready。
func minimalXAPK(t *testing.T, pkg, version, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	mw, err := zw.Create("manifest.json")
	require.NoError(t, err)
	manifest := fmt.Sprintf(
		`{"package_name":%q,"version_name":%q,"name":%q,"icon":"icon.png"}`,
		pkg, version, name)
	_, err = mw.Write([]byte(manifest))
	require.NoError(t, err)

	iw, err := zw.Create("icon.png")
	require.NoError(t, err)
	_, err = iw.Write(tinyPNG(t))
	require.NoError(t, err)

	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// TestFoundation_LibraryUploadE2E 冒烟：证明「共享上传辅助 + fake 私有桶」可打通完整素材库上传链路。
// registerUser → withFakeS3(私有桶) → uploadLibraryFile 传一段 PNG → 断言 file_id>0
// → GET /library/files 能查到该文件且 status=active → download 返回非空 url → overview 用量增加。
func TestFoundation_LibraryUploadE2E(t *testing.T) {
	const phone = "13911100001"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 私有桶（library 用 framework.S3Library）。公有桶本用例用不到，置 nil。
	libCli, _ := newFakeS3(t, "gp-lib", "")
	withFakeS3(t, nil, libCli)

	content := tinyPNG(t)

	// 上传前用量基线。
	usedBefore := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))

	// 完整 presign → PUT → confirm。
	fileID := uploadLibraryFile(t, r, token, "smoke.png", "image/png", content)
	require.Greater(t, fileID, uint(0))

	// GET /library/files 能查到该文件且 active。
	lw := doJSON(r, "GET", "/api/v1/library/files", token, nil)
	require.Equal(t, http.StatusOK, lw.Code, "文件列表失败: %s", lw.Body.String())
	page := decode(t, lw).Data.(map[string]interface{})
	assert.Equal(t, float64(1), page["total"], "上传后应有 1 个文件")
	list := page["list"].([]interface{})
	require.Len(t, list, 1)
	got := list[0].(map[string]interface{})
	assert.Equal(t, float64(fileID), got["id"])
	assert.Equal(t, "active", got["status"])
	assert.Equal(t, float64(len(content)), got["size_bytes"], "confirm 应以真实大小修正")

	// download 返回非空 presigned url。
	dw := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID), token, nil)
	require.Equal(t, http.StatusOK, dw.Code, "下载取址失败: %s", dw.Body.String())
	url := decode(t, dw).Data.(map[string]interface{})["url"].(string)
	assert.NotEmpty(t, url, "下载应返回非空 presigned url")

	// overview 用量应增加 len(content)。
	usedAfter := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	assert.Equal(t, usedBefore+int64(len(content)), usedAfter, "上传后用量应增加文件真实大小")
}

// TestFoundation_AppUploadFinalizeE2E 冒烟：证明「上传辅助 + fake 公/私桶」可打通应用 finalize 链路。
// registerUser → withFakeS3(公有桶+私有桶) → uploadLibraryFile 传一个最小可解析 XAPK
// → POST /app/user/:fileId/finalize → 断言返回 DTO 且 parse_status=ready、包名/版本符合预期
// → GET /app/user 列表能查到该应用。
func TestFoundation_AppUploadFinalizeE2E(t *testing.T) {
	const phone = "13911100002"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// finalize 需私有桶回读素材、公有桶传图标（app.uploadIcon）。
	pubCli, _ := newFakeS3(t, "gp-pub", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-lib", "")
	withFakeS3(t, pubCli, libCli)

	// 最小可解析安装包（xapk）：manifest.json 提供包名/版本/名称。
	const wantPkg, wantVer, wantName = "com.gp.smoke", "9.9.9", "SmokeApp"
	apkBytes := minimalXAPK(t, wantPkg, wantVer, wantName)
	fileID := uploadLibraryFile(t, r, token,
		"smoke.xapk", "application/vnd.android.package-archive", apkBytes)
	require.Greater(t, fileID, uint(0))

	// finalize：回读对象 → 解析 → 落 app_user_meta（ready）。
	fw := doJSON(r, "POST", fmt.Sprintf("/api/v1/app/user/%d/finalize", fileID), token, nil)
	require.Equal(t, http.StatusOK, fw.Code, "finalize 失败: %s", fw.Body.String())
	dto := decode(t, fw).Data.(map[string]interface{})
	assert.Equal(t, float64(fileID), dto["file_id"])
	assert.Equal(t, "ready", dto["parse_status"], "最小可解析 xapk 应落 ready: %v", dto)
	assert.Equal(t, wantPkg, dto["package_name"])
	assert.Equal(t, wantVer, dto["version"])
	assert.Equal(t, wantName, dto["app_name"])

	// GET /app/user 列表能查到该应用且就绪。
	lw := doJSON(r, "GET", "/api/v1/app/user", token, nil)
	require.Equal(t, http.StatusOK, lw.Code, "我的应用列表失败: %s", lw.Body.String())
	apps := decode(t, lw).Data.([]interface{})
	found := false
	for _, a := range apps {
		m := a.(map[string]interface{})
		if m["file_id"] == float64(fileID) {
			found = true
			assert.Equal(t, "ready", m["parse_status"])
			assert.Equal(t, wantPkg, m["package_name"])
		}
	}
	assert.True(t, found, "我的应用列表应包含刚 finalize 的应用")
}
