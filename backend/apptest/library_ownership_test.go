package apptest

import (
	"fmt"
	"net/http"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
	"manager-backend/modules/library"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// library_ownership_test.go 承接 Plan A 与 apptest 任务 8~10（library_upload_test.go /
// library_files_crud_test.go / library_folders_tags_test.go）已覆盖的属主/越权点，只补差集：
//
//   - TC-11-009（presign 携带他人/不存在 folder_id → 404「文件夹不存在」）：已由
//     library_upload_test.go::TestLibraryUpload_PresignForeignFolder_NotFound 覆盖，本文件不重复。
//   - 改名/移动越权：已由 library_files_crud_test.go::TestLibraryFiles_UpdateRenameAndMove 覆盖
//     （跨属主改名 → 404「文件不存在」；移到他人文件夹 → 404「目标文件夹不存在」）。
//   - 打标越权：已由 library_folders_tags_test.go::TestLibraryTags_CRUDAndFileTagging 覆盖
//     （对他人文件 PUT/GET /files/:id/tags → 404「文件不存在」；标签本身 CRUD 越权 → 404「标签不存在」）。
//   - 文件夹 CRUD 越权：已由 library_folders_tags_test.go::TestLibraryFolders_CRUDAndTreeMove 覆盖。
//
// 本文件补的差集：
//  1. 文件级 IDOR 的剩余两个动作面——DELETE /library/files/:id 与
//     GET /library/files/:id/download 此前均未见跨属主的 HTTP 断言（其余动作面改名/移动/打标已覆盖）。
//  2. TC-11-056：POST /phone/files/push-from-library 的 phone_ids 为空 → 400「请选择至少一台云手机」。
//  3. TC-11-055：素材推送里「file_ids 含他人文件」的属主校验不变式——通过 library 公开门面
//     OpenFileContent 直接验证（phone.PushFromLibrary 内部正是转调此门面做属主校验，§见
//     modules/phone/internal/push_from_library.go 第 61 行）。之所以不走真实 HTTP
//     `POST /phone/files/push-from-library`：该接口第一步 resolveCp 对任意 phone_id 都要求
//     `CloudPhone.CpID != "" && s.ops != nil`（真实开通中台实例），而 apptest 环境不配置
//     `framework.AppConfig.Midplat*`，`phone.PhoneService` 是模块 Init 时装配的包级单例、
//     无公开测试钩子可注入假 ops——任何 phone_ids 非空的请求都会先在 resolveCp 处失败
//     （422「云手机尚未开通」或 500「云手机中台未配置」），永远到不了 file_ids 的属主校验。
//     该请求路径本身已由 modules/phone/internal/push_from_library_test.go 用桩 ops 在单元层
//     覆盖（TestPushFromLibrary_NonOwnerPhoneRejected 等），此处改为直调其唯一依赖的属主校验点，
//     验证的是同一条不变式：非本人文件 → NotFound 透传，且不做副作用（不下载/不下发）。

// TestLibraryOwnership_FileDeleteAndDownload_CrossOwnerRejected 补文件级 IDOR 的剩余两个
// 动作面：A 上传一个文件后，B 尝试 DELETE 该文件 / GET 该文件的 download 地址，均应 404
// 「文件不存在」（getFile 按 user_id 过滤，不泄露资源是否存在的差异）；且 B 的越权尝试不能
// 产生任何副作用——A 事后仍能正常下载到该文件（未被误删）。
func TestLibraryOwnership_FileDeleteAndDownload_CrossOwnerRejected(t *testing.T) {
	r := setupRouter()
	const phoneA = "13911000001"
	const phoneB = "13911000002"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	libCli, _ := newFakeS3(t, "gp-lib-own1", "")
	withFakeS3(t, nil, libCli)

	fileID := uploadLibraryFile(t, r, tokenA, "a-secret.png", "image/png", tinyPNG(t))
	require.NotZero(t, fileID)

	// B 越权下载 A 的文件 → 404「文件不存在」。
	downW := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, downW.Code, "B 下载 A 的文件应 404: %s", downW.Body.String())
	assert.Contains(t, decode(t, downW).Message, "文件不存在")

	// B 越权删除 A 的文件 → 404「文件不存在」。
	delW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, delW.Code, "B 删除 A 的文件应 404: %s", delW.Body.String())
	assert.Contains(t, decode(t, delW).Message, "文件不存在")

	// 越权尝试无副作用：A 自己仍能正常下载到该文件（未被误删/未被标记删除）。
	aDownW := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID), tokenA, nil)
	require.Equal(t, http.StatusOK, aDownW.Code, "A 自己下载应仍成功: %s", aDownW.Body.String())
	assert.NotEmpty(t, decode(t, aDownW).Data.(map[string]interface{})["url"], "应返回非空 presigned url")

	// 直查 DB：该文件行仍是 active，未被 B 的越权请求软删。
	var status string
	require.NoError(t, framework.DB.Raw("SELECT status FROM library_files WHERE id = ?", fileID).Scan(&status).Error)
	assert.Equal(t, "active", status, "越权删除不应生效，文件应仍是 active")
}

// TestLibraryOwnership_PushFromLibrary_EmptyPhoneIDs_Rejected 覆盖 TC-11-056：
// file_ids 合法（1~10 个，无需真实存在——校验顺序上 phone_ids 判空发生在文件属主/内容校验之前，
// 详见 modules/phone/internal/op_api.go::PushFromLibrary），phone_ids 为空 → 400
// 「请选择至少一台云手机」，且不触达 phone.PhoneService（未开通中台也不应报别的错误）。
func TestLibraryOwnership_PushFromLibrary_EmptyPhoneIDs_Rejected(t *testing.T) {
	r := setupRouter()
	const phone = "13911000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/phone/files/push-from-library", token, map[string]interface{}{
		"file_ids":  []uint{1},
		"phone_ids": []int{},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "空 phone_ids 应 400: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "请选择至少一台云手机")

	// phone_ids 字段缺省（nil）同样应命中该校验。
	w2 := doJSON(r, "POST", "/api/v1/phone/files/push-from-library", token, map[string]interface{}{
		"file_ids": []uint{1},
	})
	assert.Equal(t, http.StatusBadRequest, w2.Code, "缺省 phone_ids 应 400: %s", w2.Body.String())
	assert.Contains(t, decode(t, w2).Message, "请选择至少一台云手机")
}

// TestLibraryOwnership_PushFromLibrary_NonOwnerFile_Rejected 覆盖 TC-11-055：
// 非本人素材不可推——phone.PushFromLibrary 下载阶段对每个 fileID 转调 library.OpenFileContent(userID, fid)
// 做属主校验（见 modules/phone/internal/push_from_library.go 第 61 行），非本人文件应 NotFound
// 透传、不产生任何下载副作用。见文件头注释：apptest 环境无法让 PushFromLibrary 的
// resolveCp 前置校验通过（无中台配置），故直接调用其唯一相关依赖 library.OpenFileContent
// 验证同一条不变式：跨属主取用 → 404「文件不存在」。
func TestLibraryOwnership_PushFromLibrary_NonOwnerFile_Rejected(t *testing.T) {
	r := setupRouter()
	const phoneA = "13911000004"
	const phoneB = "13911000005"
	tokenA := registerUser(t, r, phoneA)
	_ = registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	libCli, _ := newFakeS3(t, "gp-lib-own2", "")
	withFakeS3(t, nil, libCli)

	// A 上传一个文件；B 试图经 OpenFileContent（push-from-library 的属主校验点）读取。
	fileID := uploadLibraryFile(t, r, tokenA, "a-only.png", "image/png", tinyPNG(t))
	require.NotZero(t, fileID)

	ref, err := library.OpenFileContent(int(uidB), fileID)
	require.Error(t, err, "非本人文件应返回错误，不应读到内容")
	assert.Nil(t, ref.Reader, "越权时不应打开任何 Reader（无下载副作用）")
	assert.Equal(t, apperr.KindNotFound, apperr.KindOf(err), "跨属主取用应是 NotFound 类别（透传 404 契约）")
	assert.Contains(t, err.Error(), "文件不存在")

	// 对照：A 本人经同一门面能正常取用（证明上面的失败确实是属主隔离，而非误配置）。
	okRef, okErr := library.OpenFileContent(int(uidA), fileID)
	require.NoError(t, okErr, "本人取用应成功")
	require.NotNil(t, okRef.Reader)
	_ = okRef.Reader.Close()
}
