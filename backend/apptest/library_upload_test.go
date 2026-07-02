package apptest

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// library_upload_test.go 覆盖任务 8：素材上传全链路 + 校验 + TTL（TC-11-001/002/004/005/006/008/009/010/011/070/072）。
// 端点：POST /library/upload/presign、POST /library/upload/confirm、GET /library/files/:id/download。
// 与 modules/library/internal 的单元测试（files_test.go/dedup_test.go）不重复：内部单测已覆盖
// reserveUpload/confirmActiveWithinCapacity 等纯逻辑分支，本文件只补 HTTP 层契约（状态码 + 响应体字段 +
// 文案）与需要真实 fake S3 才能验证的行为（PUT 直传、Head 权威校验、presign TTL 字段）。

// libMD5 计算内容的十六进制 md5（presign 请求体 md5 字段）。
func libMD5(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

// libPresign 发起 presign 请求，返回响应记录器（不校验状态码，留给调用方按用例断言）。
func libPresign(r *gin.Engine, token string, body map[string]interface{}) *httptest.ResponseRecorder {
	return doJSON(r, "POST", "/api/v1/library/upload/presign", token, body)
}

// libConfirm 发起 confirm 请求，返回响应记录器。
func libConfirm(r *gin.Engine, token string, fileID uint, tagIDs []uint) *httptest.ResponseRecorder {
	body := map[string]interface{}{"file_id": fileID}
	if len(tagIDs) > 0 {
		body["tag_ids"] = tagIDs
	}
	return doJSON(r, "POST", "/api/v1/library/upload/confirm", token, body)
}

// TC-11-001/002：presign 签发上传地址 + 浏览器直传私有桶（走 uploadLibraryFile 的前两段），
// 断言 file_id/upload_url/s3_key/expires_in/instant 字段形状，以及 s3_key 落在私有桶命名空间下
// （library/<uid>/...），并校验 presign 阶段已在 library_files 落一条 uploading 行。
func TestLibraryUpload_PresignAndPutToPrivateBucket(t *testing.T) {
	const phone = "13908000001"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, store := newFakeS3(t, "gp-lib-001", "")
	withFakeS3(t, nil, libCli)

	content := tinyPNG(t)
	pw := libPresign(r, token, map[string]interface{}{
		"name":       "photo.png",
		"size_bytes": len(content),
		"mime":       "image/png",
		"folder_id":  0,
		"md5":        libMD5(content),
	})
	require.Equal(t, http.StatusOK, pw.Code, "presign 失败: %s", pw.Body.String())
	pres := decode(t, pw).Data.(map[string]interface{})

	fileID := uint(pres["file_id"].(float64))
	require.NotZero(t, fileID, "presign 应返回 file_id")
	assert.Equal(t, false, pres["instant"], "非秒传应 instant=false")
	uploadURL, _ := pres["upload_url"].(string)
	assert.NotEmpty(t, uploadURL, "presign 应返回 upload_url")
	s3Key, _ := pres["s3_key"].(string)
	assert.Contains(t, s3Key, fmt.Sprintf("library/%d/", uid), "s3_key 应落在该用户命名空间下")
	assert.Greater(t, pres["expires_in"].(float64), float64(0), "presign 应返回正的 expires_in")

	// presign 阶段应已落一条 uploading 状态的行（此刻 confirm 尚未调用）。
	var status string
	require.NoError(t, framework.DB.Table("library_files").Where("id = ?", fileID).
		Select("status").Scan(&status).Error)
	assert.Equal(t, "uploading", status, "presign 后应处于 uploading")

	// TC-11-002：浏览器直传私有桶——用裸 HTTP PUT 打 upload_url，S3 侧应 200；
	// 直传后对象应已落入 fake 私有桶存储（对象内容与声明一致）。
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "PUT 到 upload_url 失败")
	_ = putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode, "PUT 到私有桶应 200")

	got, ok := store.get(s3Key)
	require.True(t, ok, "对象应已落入私有桶存储")
	assert.Equal(t, content, got, "落库对象内容应与直传内容一致")

	// confirm 权威校验落库：置 active，size_bytes 以真实值修正，used_bytes 增。
	usedBefore := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	cw := libConfirm(r, token, fileID, nil)
	require.Equal(t, http.StatusOK, cw.Code, "confirm 失败: %s", cw.Body.String())
	f := decode(t, cw).Data.(map[string]interface{})
	assert.Equal(t, "active", f["status"])
	assert.Equal(t, float64(len(content)), f["size_bytes"])
	usedAfter := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	assert.Equal(t, usedBefore+int64(len(content)), usedAfter, "confirm 后 used_bytes 应增加真实大小")
}

// TC-11-004：大小/内容不一致拒绝——presign 声明的 md5/size 与实际直传对象不符，confirm 应
// 422「校验不通过，请重新上传」，并删除临时对象（fake S3 store 中不再存在该 key）+ 删占位行
// （不计用量、库中查不到该 file_id）。
func TestLibraryUpload_ConfirmContentMismatch_Rejected(t *testing.T) {
	const phone = "13908000002"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, store := newFakeS3(t, "gp-lib-002", "")
	withFakeS3(t, nil, libCli)

	declared := tinyPNG(t)
	pw := libPresign(r, token, map[string]interface{}{
		"name":       "mismatch.png",
		"size_bytes": len(declared),
		"mime":       "image/png",
		"folder_id":  0,
		"md5":        libMD5(declared),
	})
	require.Equal(t, http.StatusOK, pw.Code)
	pres := decode(t, pw).Data.(map[string]interface{})
	fileID := uint(pres["file_id"].(float64))
	uploadURL := pres["upload_url"].(string)
	s3Key := pres["s3_key"].(string)

	// 实际直传内容与声明 md5/size 不一致（追加字节，破坏 md5 与大小）。
	actual := append(append([]byte{}, declared...), []byte("tampered-extra-bytes")...)
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(actual))
	require.NoError(t, err)
	putResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	cw := libConfirm(r, token, fileID, nil)
	assert.Equal(t, http.StatusUnprocessableEntity, cw.Code, "内容不一致应 422: %s", cw.Body.String())
	assert.Contains(t, decode(t, cw).Message, "校验不通过", "应提示重新上传")

	// 临时对象应已被删除。
	_, ok := store.get(s3Key)
	assert.False(t, ok, "校验失败应删除临时对象")

	// 占位行应已被删除，不计用量。
	var cnt int64
	require.NoError(t, framework.DB.Table("library_files").Where("id = ?", fileID).Count(&cnt).Error)
	assert.Zero(t, cnt, "校验失败应删除占位行")
}

// TC-11-005：presign 传空文件名 → 422「文件名不能为空」，且不建行。
func TestLibraryUpload_PresignEmptyName_Rejected(t *testing.T) {
	const phone = "13908000003"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-003", "")
	withFakeS3(t, nil, libCli)

	var before int64
	require.NoError(t, framework.DB.Table("library_files").Where("user_id = ?", uid).Count(&before).Error)

	pw := libPresign(r, token, map[string]interface{}{
		"name": "", "size_bytes": 100, "mime": "image/png", "folder_id": 0,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, pw.Code, "空文件名应 422: %s", pw.Body.String())
	assert.Contains(t, decode(t, pw).Message, "文件名不能为空")

	var after int64
	require.NoError(t, framework.DB.Table("library_files").Where("user_id = ?", uid).Count(&after).Error)
	assert.Equal(t, before, after, "校验失败不应建行")
}

// TC-11-006：presign 传非法大小（size_bytes<=0）→ 422「文件大小非法」。
func TestLibraryUpload_PresignInvalidSize_Rejected(t *testing.T) {
	const phone = "13908000004"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-004", "")
	withFakeS3(t, nil, libCli)

	pw := libPresign(r, token, map[string]interface{}{
		"name": "a.png", "size_bytes": 0, "mime": "image/png", "folder_id": 0,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, pw.Code, "size_bytes=0 应 422: %s", pw.Body.String())
	assert.Contains(t, decode(t, pw).Message, "文件大小非法")

	pw2 := libPresign(r, token, map[string]interface{}{
		"name": "a.png", "size_bytes": -10, "mime": "image/png", "folder_id": 0,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, pw2.Code, "size_bytes<0 应 422: %s", pw2.Body.String())
	assert.Contains(t, decode(t, pw2).Message, "文件大小非法")
}

// TC-11-008：同夹重名自动去重命名——同一文件夹下先落一个 video.mp4，再传同名文件，
// 落库名应自动变为 video_<id>.mp4（扩展名前附自增 id）。
func TestLibraryUpload_DuplicateNameAutoRenamed(t *testing.T) {
	const phone = "13908000005"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-005", "")
	withFakeS3(t, nil, libCli)

	content1 := tinyPNG(t)
	first := uploadLibraryFile(t, r, token, "video.mp4", "video/mp4", content1)
	require.Greater(t, first, uint(0))

	// 第二次传同名，内容不同（避免全局去重 md5 干扰对文件名逻辑的验证）。
	content2 := append(append([]byte{}, content1...), []byte("second-copy")...)
	pw := libPresign(r, token, map[string]interface{}{
		"name":       "video.mp4",
		"size_bytes": len(content2),
		"mime":       "video/mp4",
		"folder_id":  0,
		"md5":        libMD5(content2),
	})
	require.Equal(t, http.StatusOK, pw.Code, "第二次 presign 失败: %s", pw.Body.String())
	pres := decode(t, pw).Data.(map[string]interface{})
	secondID := uint(pres["file_id"].(float64))
	require.NotZero(t, secondID)
	require.NotEqual(t, first, secondID)

	var name string
	require.NoError(t, framework.DB.Table("library_files").Where("id = ?", secondID).
		Select("name").Scan(&name).Error)
	assert.Equal(t, fmt.Sprintf("video_%d.mp4", secondID), name, "同夹重名应自动加 _<id> 去重")
}

// TC-11-009：文件夹归属校验——presign 带他人（或不存在）的 folder_id → 404「文件夹不存在」。
func TestLibraryUpload_PresignForeignFolder_NotFound(t *testing.T) {
	const phoneA = "13908000006"
	const phoneB = "13908000007"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	libCli, _ := newFakeS3(t, "gp-lib-006", "")
	withFakeS3(t, nil, libCli)

	// A 建一个文件夹。
	fw := doJSON(r, "POST", "/api/v1/library/folders", tokenA, map[string]interface{}{
		"name": "A的文件夹", "parent_id": 0,
	})
	require.Equal(t, http.StatusOK, fw.Code, "建文件夹失败: %s", fw.Body.String())
	folderID := uint(decode(t, fw).Data.(map[string]interface{})["id"].(float64))

	// B 用 A 的 folder_id presign → 404。
	pw := libPresign(r, tokenB, map[string]interface{}{
		"name": "x.png", "size_bytes": 100, "mime": "image/png", "folder_id": folderID,
	})
	assert.Equal(t, http.StatusNotFound, pw.Code, "他人 folder_id 应 404: %s", pw.Body.String())
	assert.Contains(t, decode(t, pw).Message, "文件夹不存在")

	// 不存在的 folder_id 同样 404。
	pw2 := libPresign(r, tokenB, map[string]interface{}{
		"name": "x.png", "size_bytes": 100, "mime": "image/png", "folder_id": folderID + 999999,
	})
	assert.Equal(t, http.StatusNotFound, pw2.Code, "不存在的 folder_id 应 404: %s", pw2.Body.String())
	assert.Contains(t, decode(t, pw2).Message, "文件夹不存在")
}

// TC-11-010：S3 未配置降级——不装 fakeS3（framework.S3Library==nil）时，presign 与 download
// 均应 503「对象存储未配置，暂无法上传或下载文件」。presign 侧的裸契约已由
// library_test.go::TestLibraryPresignWithoutS3Returns503 覆盖；本用例补 download 侧。
func TestLibraryUpload_S3NotConfigured_DownloadReturns503(t *testing.T) {
	if framework.S3Library != nil {
		t.Skip("已配置 S3Library，跳过 503 契约用例")
	}
	const phone = "13908000008"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	dw := doJSON(r, "GET", "/api/v1/library/files/1/download", token, nil)
	assert.Equal(t, http.StatusServiceUnavailable, dw.Code, "未配置 S3 时下载应 503: %s", dw.Body.String())
	assert.Contains(t, decode(t, dw).Message, "对象存储未配置")

	// 同一未配置环境下 presign 亦应 503（与 library_test.go 的契约用例重复断言一次，确认本文件内自洽）。
	pw := libPresign(r, token, map[string]interface{}{
		"name": "a.png", "size_bytes": 100, "mime": "image/png", "folder_id": 0,
	})
	assert.Equal(t, http.StatusServiceUnavailable, pw.Code, "未配置 S3 时上传应 503: %s", pw.Body.String())
	assert.Contains(t, decode(t, pw).Message, "对象存储未配置")
}

// TC-11-011：文件类型归类——不同 mime/ext 上传 confirm 后查列表，file_type 应按 mime/扩展名
// 正确归类为 app/image/video/audio/document/other。
func TestLibraryUpload_FileTypeClassification(t *testing.T) {
	const phone = "13908000009"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	pubCli, _ := newFakeS3(t, "gp-pub-009", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-lib-009", "")
	withFakeS3(t, pubCli, libCli)

	png := tinyPNG(t)
	xapk := minimalXAPK(t, "com.gp.classify", "1.0.0", "ClassifyApp")
	doc := []byte("%PDF-1.4 fake pdf content for classification test")
	other := []byte("random-bytes-without-known-extension")

	cases := []struct {
		name, mime, ext, wantType string
		content                   []byte
	}{
		{"pic", "image/png", ".png", "image", png},
		{"pkg", "application/vnd.android.package-archive", ".xapk", "app", xapk},
		{"doc", "application/pdf", ".pdf", "document", doc},
		{"blob", "application/octet-stream", ".bin", "other", other},
	}

	fileIDToType := map[uint]string{}
	for _, c := range cases {
		fname := c.name + c.ext
		fid := uploadLibraryFile(t, r, token, fname, c.mime, c.content)
		fileIDToType[fid] = c.wantType
	}

	lw := doJSON(r, "GET", "/api/v1/library/files?size=50", token, nil)
	require.Equal(t, http.StatusOK, lw.Code, "文件列表失败: %s", lw.Body.String())
	page := decode(t, lw).Data.(map[string]interface{})
	list := page["list"].([]interface{})
	seen := map[uint]bool{}
	for _, it := range list {
		m := it.(map[string]interface{})
		fid := uint(m["id"].(float64))
		wantType, ok := fileIDToType[fid]
		if !ok {
			continue
		}
		seen[fid] = true
		assert.Equal(t, wantType, m["file_type"], "file_id=%d 应归类为 %s", fid, wantType)
	}
	assert.Len(t, seen, len(cases), "应能在列表中找到全部上传的文件")
}

// TC-11-070：下载签名 URL 时效——GET /library/files/:id/download 返回 presigned GET URL，
// 携带 expires_in 语义由 S3_PRESIGN_GET_TTL 决定（默认 15m，测试环境未显式配置即走默认值）；
// 本用例断言接口返回非空、可直接用于 GET 的 presigned url（含查询串签名参数）。
func TestLibraryUpload_DownloadPresignedURL(t *testing.T) {
	const phone = "13908000010"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-010", "")
	withFakeS3(t, nil, libCli)

	content := tinyPNG(t)
	fileID := uploadLibraryFile(t, r, token, "ttl.png", "image/png", content)

	dw := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID), token, nil)
	require.Equal(t, http.StatusOK, dw.Code, "下载取址失败: %s", dw.Body.String())
	url := decode(t, dw).Data.(map[string]interface{})["url"].(string)
	assert.NotEmpty(t, url, "下载应返回非空 presigned url")
	assert.Contains(t, url, "?", "presigned GET url 应带签名查询参数")

	// presigned url 应可直接 GET 到与上传一致的内容（验证「时效内可读」的正向侧）。
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "时效内 presigned url 应可直接读取私有对象")
}

// TC-11-072：上传签名 URL 时效——presign 返回的 expires_in 应为正数秒值（= S3_PRESIGN_PUT_TTL，
// 测试环境未显式配置时走 service 内回退默认 30 分钟 = 1800 秒）。
func TestLibraryUpload_PresignExpiresIn(t *testing.T) {
	const phone = "13908000011"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-011", "")
	withFakeS3(t, nil, libCli)

	content := tinyPNG(t)
	pw := libPresign(r, token, map[string]interface{}{
		"name":       "ttl2.png",
		"size_bytes": len(content),
		"mime":       "image/png",
		"folder_id":  0,
		"md5":        libMD5(content),
	})
	require.Equal(t, http.StatusOK, pw.Code, "presign 失败: %s", pw.Body.String())
	pres := decode(t, pw).Data.(map[string]interface{})
	expiresIn := pres["expires_in"].(float64)
	assert.Greater(t, expiresIn, float64(0), "expires_in 应为正数秒值")
	// 未经 framework.LoadConfig 的 apptest 环境下 AppConfig 的 S3PresignPutTTL 大概率为零值，
	// service 回退默认 30 分钟；若测试环境显式配置了 AppConfig，则遵循注入值，不作强相等断言，
	// 只保证契约字段存在且为正（时效具体数值由 framework.AppConfig 配置项单测覆盖）。
	assert.LessOrEqual(t, expiresIn, float64(24*3600), "expires_in 应是合理的时效性数值（<=24h）")
}
