package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// library_files_crud_test.go 覆盖任务 9：文件 CRUD 与配额锁定
// （docs/superpowers/plans/2026-07-02-验收测试-planB-接口层全覆盖.md 任务 9）。
//
// 覆盖 TC-11-060/061/062/063/064/040/041。TC-11-040/041 的核心不变式（assertNotLocked
// 拦下载、列表/删除不受锁定影响）已由 modules/library/internal/files_test.go::
// TestOverflowLock_BlocksFetchAllowsListDelete 在 service 层覆盖；本文件承接该点，
// 补齐同一不变式在**接口层**（真实 HTTP + JWT + 状态码/文案）的验证。

// libCrudUploadPNG 经 fake 私有桶完整走一次 presign→PUT→confirm，返回 file_id。
// 调用前测试须已 withFakeS3 装好私有桶（framework.S3Library）。
func libCrudUploadPNG(t *testing.T, r *gin.Engine, token, name string) uint {
	t.Helper()
	return uploadLibraryFile(t, r, token, name, "image/png", tinyPNG(t))
}

// libCrudListPage 请求文件列表并解析为 {list []interface{}, total float64}。
func libCrudListPage(t *testing.T, r *gin.Engine, token, query string) (list []interface{}, total float64) {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/library/files"+query, token, nil)
	require.Equal(t, http.StatusOK, w.Code, "文件列表失败: %s", w.Body.String())
	page := decode(t, w).Data.(map[string]interface{})
	total = page["total"].(float64)
	list, _ = page["list"].([]interface{})
	return list, total
}

// TestLibraryFiles_ListPagingOwnerIsolation 覆盖 TC-11-060：
// 列表分页 GET /library/files?page=&size= 返回 {list,total}，仅 active、按属主隔离，含 tag_ids。
func TestLibraryFiles_ListPagingOwnerIsolation(t *testing.T) {
	r := setupRouter()

	const phoneA = "13909000001"
	const phoneB = "13909000002"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	libCli, _ := newFakeS3(t, "gp-lib-crud1", "")
	withFakeS3(t, nil, libCli)

	// A 上传 3 个文件，B 上传 1 个（不同 md5 内容避免误判去重影响个数）。
	idA1 := libCrudUploadPNG(t, r, tokenA, "a1.png")
	idA2 := libCrudUploadPNG(t, r, tokenA, "a2.png")
	idA3 := libCrudUploadPNG(t, r, tokenA, "a3.png")
	_ = libCrudUploadPNG(t, r, tokenB, "b1.png")

	// A 的列表：total=3，只看到 A 自己的文件；每项含 tag_ids 字段（空数组也算存在）。
	list, total := libCrudListPage(t, r, tokenA, "?page=1&size=20")
	assert.Equal(t, float64(3), total, "A 应有 3 个 active 文件")
	require.Len(t, list, 3)
	seen := map[uint]bool{}
	for _, it := range list {
		m := it.(map[string]interface{})
		seen[uint(m["id"].(float64))] = true
		assert.Equal(t, "active", m["status"], "列表只应含 active 文件")
		_, hasTagIDs := m["tag_ids"]
		assert.True(t, hasTagIDs, "文件项应含 tag_ids 字段")
	}
	assert.True(t, seen[idA1] && seen[idA2] && seen[idA3], "A 列表应恰好含自己上传的 3 个文件")

	// B 的列表：total=1，属主隔离，看不到 A 的文件。
	listB, totalB := libCrudListPage(t, r, tokenB, "?page=1&size=20")
	assert.Equal(t, float64(1), totalB, "B 应只有 1 个 active 文件")
	require.Len(t, listB, 1)
	assert.NotEqual(t, idA1, uint(listB[0].(map[string]interface{})["id"].(float64)))

	// 分页：size=2 时 A 第 1 页应恰好 2 条，total 仍为 3。
	list2, total2 := libCrudListPage(t, r, tokenA, "?page=1&size=2")
	assert.Equal(t, float64(3), total2, "分页不应改变 total")
	assert.Len(t, list2, 2, "size=2 应恰好返回 2 条")
}

// TestLibraryFiles_FilterByFolderTypeTag 覆盖 TC-11-061：
// 按 folder_id/file_type/tag_id 组合过滤正确。
func TestLibraryFiles_FilterByFolderTypeTag(t *testing.T) {
	r := setupRouter()
	const phone = "13909000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-crud2", "")
	withFakeS3(t, nil, libCli)

	// 建一个非根文件夹。
	fw := doJSON(r, "POST", "/api/v1/library/folders", token, map[string]interface{}{"name": "夹1", "parent_id": 0})
	require.Equal(t, http.StatusOK, fw.Code, "建夹失败: %s", fw.Body.String())
	folderID := uint(decode(t, fw).Data.(map[string]interface{})["id"].(float64))

	// 根目录传一张图片，非根文件夹传一张图片。
	rootImg := libCrudUploadPNG(t, r, token, "root.png")
	inFolderImg := libCrudUploadPNG(t, r, token, "infolder.png")
	// 把 inFolderImg 移入 folderID（UpdateFile 走 folder_id）。
	mw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", inFolderImg), token, map[string]interface{}{
		"folder_id": folderID,
	})
	require.Equal(t, http.StatusOK, mw.Code, "移动入夹失败: %s", mw.Body.String())

	// 建一个标签，打到 rootImg 上。
	tw := doJSON(r, "POST", "/api/v1/library/tags", token, map[string]interface{}{"name": "标签A"})
	require.Equal(t, http.StatusOK, tw.Code, "建标签失败: %s", tw.Body.String())
	tagID := uint(decode(t, tw).Data.(map[string]interface{})["id"].(float64))
	stw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d/tags", rootImg), token, map[string]interface{}{
		"tag_ids": []uint{tagID},
	})
	require.Equal(t, http.StatusOK, stw.Code, "打标失败: %s", stw.Body.String())

	// folder_id=0（根）→ 只含 rootImg。
	listRoot, totalRoot := libCrudListPage(t, r, token, "?folder_id=0")
	assert.Equal(t, float64(1), totalRoot, "根目录应只有 1 个文件")
	require.Len(t, listRoot, 1)
	assert.Equal(t, float64(rootImg), listRoot[0].(map[string]interface{})["id"])

	// folder_id=folderID → 只含 inFolderImg。
	listFolder, totalFolder := libCrudListPage(t, r, token, fmt.Sprintf("?folder_id=%d", folderID))
	assert.Equal(t, float64(1), totalFolder, "目标文件夹应只有 1 个文件")
	require.Len(t, listFolder, 1)
	assert.Equal(t, float64(inFolderImg), listFolder[0].(map[string]interface{})["id"])

	// file_type=image → 命中两者。
	listImg, totalImg := libCrudListPage(t, r, token, "?file_type=image")
	assert.Equal(t, float64(2), totalImg, "两张图片都应命中 file_type=image")
	_ = listImg

	// file_type=video → 空。
	_, totalVideo := libCrudListPage(t, r, token, "?file_type=video")
	assert.Equal(t, float64(0), totalVideo, "无视频文件，file_type=video 应为空")

	// tag_id=tagID → 只含 rootImg。
	listTag, totalTag := libCrudListPage(t, r, token, fmt.Sprintf("?tag_id=%d", tagID))
	assert.Equal(t, float64(1), totalTag, "按标签过滤应只命中打标的文件")
	require.Len(t, listTag, 1)
	assert.Equal(t, float64(rootImg), listTag[0].(map[string]interface{})["id"])

	// 组合：folder_id=0 + file_type=image → 仍是 rootImg。
	listCombo, totalCombo := libCrudListPage(t, r, token, "?folder_id=0&file_type=image")
	assert.Equal(t, float64(1), totalCombo)
	require.Len(t, listCombo, 1)
	assert.Equal(t, float64(rootImg), listCombo[0].(map[string]interface{})["id"])
}

// TestLibraryFiles_KeywordSearch 覆盖 TC-11-062：keyword 搜索按 name LIKE %xxx% 命中。
func TestLibraryFiles_KeywordSearch(t *testing.T) {
	r := setupRouter()
	const phone = "13909000004"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-crud3", "")
	withFakeS3(t, nil, libCli)

	hit := libCrudUploadPNG(t, r, token, "vacation-photo.png")
	_ = libCrudUploadPNG(t, r, token, "invoice.png")

	list, total := libCrudListPage(t, r, token, "?keyword=vacation")
	assert.Equal(t, float64(1), total, "keyword 应只命中含关键词的文件名")
	require.Len(t, list, 1)
	assert.Equal(t, float64(hit), list[0].(map[string]interface{})["id"])

	// 不存在的关键词 → 空结果，不报错。
	_, totalNone := libCrudListPage(t, r, token, "?keyword=doesnotexist")
	assert.Equal(t, float64(0), totalNone)
}

// TestLibraryFiles_SoftDeleteReclaimsUsageAndHidesFromList 覆盖 TC-11-063：
// 软删 DELETE /library/files/:id：不在列表出现、used_bytes 回收。
func TestLibraryFiles_SoftDeleteReclaimsUsageAndHidesFromList(t *testing.T) {
	r := setupRouter()
	const phone = "13909000005"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-crud4", "")
	withFakeS3(t, nil, libCli)

	content := tinyPNG(t)
	usedBefore := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	fileID := uploadLibraryFile(t, r, token, "deleteme.png", "image/png", content)

	usedAfterUpload := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	require.Equal(t, usedBefore+int64(len(content)), usedAfterUpload, "上传后用量应计入真实大小")

	dw := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/files/%d", fileID), token, nil)
	require.Equal(t, http.StatusOK, dw.Code, "删除失败: %s", dw.Body.String())

	// 列表不再出现。
	_, total := libCrudListPage(t, r, token, "")
	assert.Equal(t, float64(0), total, "软删后列表不应再出现该文件")

	// used_bytes 回收到删除前水平。
	usedAfterDelete := int64(libOverview(t, r, token)["usage"].(map[string]interface{})["used_bytes"].(float64))
	assert.Equal(t, usedBefore, usedAfterDelete, "软删后用量应回收")

	// 重复删除（已软删）→ NotFound（getFile 仅查未软删行）。
	dw2 := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/files/%d", fileID), token, nil)
	assert.Equal(t, http.StatusNotFound, dw2.Code, "重复删除应 404: %s", dw2.Body.String())
	assert.Contains(t, decode(t, dw2).Message, "文件不存在")
}

// TestLibraryFiles_UpdateRenameAndMove 覆盖 TC-11-064：
// PUT /library/files/:id 改名/移动文件夹；目标 folder_id 非 0 时校验归属（他人文件夹/不存在文件夹 → 404）。
func TestLibraryFiles_UpdateRenameAndMove(t *testing.T) {
	r := setupRouter()
	const phoneA = "13909000006"
	const phoneB = "13909000007"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	libCli, _ := newFakeS3(t, "gp-lib-crud5", "")
	withFakeS3(t, nil, libCli)

	fileID := libCrudUploadPNG(t, r, tokenA, "orig.png")

	// 1) 改名成功。
	rw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenA, map[string]interface{}{
		"name": "renamed.png",
	})
	require.Equal(t, http.StatusOK, rw.Code, "改名失败: %s", rw.Body.String())
	assert.Equal(t, "renamed.png", decode(t, rw).Data.(map[string]interface{})["name"])

	// 2) 移动到 A 自己的合法文件夹成功。
	fw := doJSON(r, "POST", "/api/v1/library/folders", tokenA, map[string]interface{}{"name": "A的夹", "parent_id": 0})
	require.Equal(t, http.StatusOK, fw.Code, "建夹失败: %s", fw.Body.String())
	folderA := uint(decode(t, fw).Data.(map[string]interface{})["id"].(float64))
	mw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenA, map[string]interface{}{
		"folder_id": folderA,
	})
	require.Equal(t, http.StatusOK, mw.Code, "移动失败: %s", mw.Body.String())
	assert.Equal(t, float64(folderA), decode(t, mw).Data.(map[string]interface{})["folder_id"])

	// 3) 移到不存在的文件夹 → 404「目标文件夹不存在」。
	badW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenA, map[string]interface{}{
		"folder_id": uint(9999999),
	})
	assert.Equal(t, http.StatusNotFound, badW.Code, "移到不存在的文件夹应 404: %s", badW.Body.String())
	assert.Contains(t, decode(t, badW).Message, "目标文件夹不存在")

	// 4) 移到 B 的文件夹（跨属主）→ 404「目标文件夹不存在」（getFolder 按属主查，B 的夹对 A 不可见）。
	fwB := doJSON(r, "POST", "/api/v1/library/folders", tokenB, map[string]interface{}{"name": "B的夹", "parent_id": 0})
	require.Equal(t, http.StatusOK, fwB.Code, "B 建夹失败: %s", fwB.Body.String())
	folderB := uint(decode(t, fwB).Data.(map[string]interface{})["id"].(float64))
	crossW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenA, map[string]interface{}{
		"folder_id": folderB,
	})
	assert.Equal(t, http.StatusNotFound, crossW.Code, "移到他人文件夹应 404: %s", crossW.Body.String())
	assert.Contains(t, decode(t, crossW).Message, "目标文件夹不存在")

	// 5) 改名为空 → 422「文件名不能为空」。
	emptyW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenA, map[string]interface{}{
		"name": "   ",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, emptyW.Code, "空文件名应 422: %s", emptyW.Body.String())
	assert.Contains(t, decode(t, emptyW).Message, "文件名不能为空")

	// 6) B 操作 A 的文件（跨属主改名）→ 404「文件不存在」（getFile 按属主查）。
	ownerW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d", fileID), tokenB, map[string]interface{}{
		"name": "hijack.png",
	})
	assert.Equal(t, http.StatusNotFound, ownerW.Code, "跨属主改名应 404: %s", ownerW.Body.String())
	assert.Contains(t, decode(t, ownerW).Message, "文件不存在")
}

// libCrudForceOverflowLock 直接经 framework.DB 把某用户的用量置为「超出当前容量」，
// 造出 TC-11-040/041 的超额锁定前置态。先用 overview 读出该用户当前实时容量（默认免费档
// FreeQuotaBytes，不写死具体字节数，避免与定价配置耦合），再把 used_bytes 设为 capacity+1。
// library_usage 每用户一行，不存在时插入；用 sqlite 支持的 INSERT OR REPLACE 覆盖写。
func libCrudForceOverflowLock(t *testing.T, r *gin.Engine, token string, uid uint) (capacity int64) {
	t.Helper()
	ov := libOverview(t, r, token)
	capacity = int64(ov["usage"].(map[string]interface{})["capacity_bytes"].(float64))
	overUsed := capacity + 1
	now := time.Now()
	require.NoError(t, framework.DB.Exec(
		`INSERT OR REPLACE INTO library_usage (user_id, used_bytes, updated_at) VALUES (?, ?, ?)`,
		uid, overUsed, now,
	).Error)
	return capacity
}

// TestLibraryFiles_OverflowLock_BlocksDownload_AllowsListDelete 覆盖 TC-11-040/041：
// 超额锁定（used_bytes > capacity_bytes）禁止「取用」（下载 403「容量超限，请扩容或删除文件后再使用」），
// 但列表/删除不受锁定影响（便于用户自救腾容量）。
//
// 承接 Plan A（modules/library/internal/files_test.go::TestOverflowLock_BlocksFetchAllowsListDelete
// 已在 service 层覆盖 assertNotLocked/ListFiles/DeleteFile 的行为），本用例在**接口层**用真实
// HTTP + JWT 复核同一不变式：下载走 403 具体状态码 + 特定文案，列表/删除仍 200 且删除后用量正确回收、
// 解锁后下载恢复。
func TestLibraryFiles_OverflowLock_BlocksDownload_AllowsListDelete(t *testing.T) {
	r := setupRouter()
	const phone = "13909000008"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	libCli, _ := newFakeS3(t, "gp-lib-crud6", "")
	withFakeS3(t, nil, libCli)

	// 先在容量内正常上传一个文件（active），再人为把 used_bytes 顶到超容量，
	// 模拟「降容/扩容到期后」的超额锁定场景（文件仍在，只是账面用量被顶爆）。
	fileID := libCrudUploadPNG(t, r, token, "locked.png")

	capacity := libCrudForceOverflowLock(t, r, token, uid)
	require.Greater(t, capacity, int64(0), "免费档容量应 > 0，才能构造出超额场景")

	// overview 应反映锁定标志。
	ov := libOverview(t, r, token)
	assert.True(t, ov["usage"].(map[string]interface{})["locked"].(bool), "超额后 overview.usage.locked 应为 true")

	// 取用（下载）被拦：403 + 特定文案。
	dlW := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID), token, nil)
	assert.Equal(t, http.StatusForbidden, dlW.Code, "超额锁定应拦下载: %s", dlW.Body.String())
	assert.Contains(t, decode(t, dlW).Message, "容量超限，请扩容或删除文件后再使用")

	// 列表仍可用（不受锁定影响），且能看到该文件。
	list, total := libCrudListPage(t, r, token, "")
	assert.Equal(t, float64(1), total, "锁定态列表仍应返回文件")
	require.Len(t, list, 1)
	assert.Equal(t, float64(fileID), list[0].(map[string]interface{})["id"])

	// 删除仍可用（用户自救腾容量）：200。软删按【真实文件大小】精确回收——账面用量此前被
	// 人为顶爆到 capacity+1（与文件真实大小脱钩），故删除后不会清零，而是精确减去该文件真实字节。
	var fileSize int64
	require.NoError(t, framework.DB.
		Raw("SELECT size_bytes FROM library_files WHERE id = ?", fileID).Scan(&fileSize).Error)
	require.Greater(t, fileSize, int64(0), "文件真实大小应 > 0")
	usedBeforeDelete := capacity + 1

	delW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/files/%d", fileID), token, nil)
	require.Equal(t, http.StatusOK, delW.Code, "锁定态删除应仍可用: %s", delW.Body.String())

	var usedAfterDelete int64
	require.NoError(t, framework.DB.
		Raw("SELECT used_bytes FROM library_usage WHERE user_id = ?", uid).
		Scan(&usedAfterDelete).Error)
	assert.Equal(t, usedBeforeDelete-fileSize, usedAfterDelete,
		"软删应按真实文件大小精确回收（capacity+1 - 该文件真实字节）")

	// 解锁：把账面用量清到 0（模拟扩容/继续删文件回到容量内），下载应恢复 200。
	require.NoError(t, framework.DB.Exec(
		"UPDATE library_usage SET used_bytes = 0 WHERE user_id = ?", uid).Error)
	assert.False(t, libOverview(t, r, token)["usage"].(map[string]interface{})["locked"].(bool),
		"清零用量后 overview.usage.locked 应恢复 false")
	fileID2 := libCrudUploadPNG(t, r, token, "unlocked.png")
	dlW2 := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/download", fileID2), token, nil)
	assert.Equal(t, http.StatusOK, dlW2.Code, "解锁后下载应恢复正常: %s", dlW2.Body.String())
	url, _ := decode(t, dlW2).Data.(map[string]interface{})["url"].(string)
	assert.NotEmpty(t, url, "解锁后下载应返回非空 presigned url")
}
