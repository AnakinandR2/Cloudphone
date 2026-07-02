package apptest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// library_folders_tags_test.go 覆盖 TC-11-065（文件夹 CRUD + 树形移动 + 属主隔离）、
// TC-11-066（标签 CRUD + 文件打标 + 属主隔离），经公开 HTTP。
//
// 承接关系：modules/library/internal/folders_test.go 已在单元层覆盖同名校验/防环细节
// （TestCreateFolder_RejectsDuplicateName、TestMoveFolder），本文件不重复这些纯逻辑分支，
// 聚焦 HTTP 层契约（状态码/响应形状）与跨用户属主隔离（内部单测用同进程 uid 造数据，
// 无法覆盖「他人令牌打接口」这一路径）。

// libFolderCreate 创建文件夹，返回 (folder_id, http响应体)。
func libFolderCreate(t *testing.T, r *gin.Engine, token, name string, parentID uint) (uint, map[string]interface{}) {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/library/folders", token, map[string]interface{}{
		"name": name, "parent_id": parentID,
	})
	require.Equal(t, http.StatusOK, w.Code, "创建文件夹失败: %s", w.Body.String())
	fo := decode(t, w).Data.(map[string]interface{})
	return uint(fo["id"].(float64)), fo
}

// libTagCreate 创建标签，返回 tag_id。
func libTagCreate(t *testing.T, r *gin.Engine, token, name, color string) uint {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/library/tags", token, map[string]interface{}{
		"name": name, "color": color,
	})
	require.Equal(t, http.StatusOK, w.Code, "创建标签失败: %s", w.Body.String())
	tg := decode(t, w).Data.(map[string]interface{})
	return uint(tg["id"].(float64))
}

// libFileTagIDs 从 GET /library/files/:id/tags 的响应中提取 tag_ids 并转为 []uint。
func libFileTagIDs(t *testing.T, w *httptest.ResponseRecorder) []uint {
	t.Helper()
	data := decode(t, w).Data.(map[string]interface{})
	raw, _ := data["tag_ids"].([]interface{})
	out := make([]uint, 0, len(raw))
	for _, v := range raw {
		out = append(out, uint(v.(float64)))
	}
	return out
}

// TestLibraryFolders_CRUDAndTreeMove 承接 Plan A（无独立 apptest），补 TC-11-065：
// 文件夹 GET/POST/PUT/POST move/DELETE 全链路 + 树形移动 + 非空夹禁止删除 + 属主隔离。
func TestLibraryFolders_CRUDAndTreeMove(t *testing.T) {
	const phoneA = "13910000001"
	const phoneB = "13910000002"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	// --- GET 初始为空 ---
	lw := doJSON(r, "GET", "/api/v1/library/folders", tokenA, nil)
	require.Equal(t, http.StatusOK, lw.Code)
	initList, ok := decode(t, lw).Data.([]interface{})
	require.True(t, ok, "文件夹列表应为数组")
	assert.Empty(t, initList, "新用户初始文件夹列表应为空")

	// --- POST 新建根夹「作品」---
	rootID, rootFo := libFolderCreate(t, r, tokenA, "作品", 0)
	require.NotZero(t, rootID)
	assert.Equal(t, "作品", rootFo["name"])
	assert.Equal(t, float64(0), rootFo["parent_id"], "未传 parent_id 应落根")

	// --- POST 新建子夹「草稿」于 rootID 下 ---
	childID, childFo := libFolderCreate(t, r, tokenA, "草稿", rootID)
	require.NotZero(t, childID)
	assert.Equal(t, float64(rootID), childFo["parent_id"], "子夹应挂在指定父夹下")

	// GET 应能看到两个夹（树形由前端组装，接口只返回平铺列表）。
	lw2 := doJSON(r, "GET", "/api/v1/library/folders", tokenA, nil)
	require.Equal(t, http.StatusOK, lw2.Code)
	list2 := decode(t, lw2).Data.([]interface{})
	assert.Len(t, list2, 2, "应能查到刚建的两个文件夹")

	// --- PUT 重命名子夹 ---
	uw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/folders/%d", childID), tokenA, map[string]interface{}{
		"name": "草稿箱",
	})
	require.Equal(t, http.StatusOK, uw.Code, "重命名失败: %s", uw.Body.String())
	renamed := decode(t, uw).Data.(map[string]interface{})
	assert.Equal(t, "草稿箱", renamed["name"])

	// 重命名为空名 → 422「文件夹名不能为空」。
	emptyW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/folders/%d", childID), tokenA, map[string]interface{}{
		"name": "   ",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, emptyW.Code, "空名应 422: %s", emptyW.Body.String())
	assert.Contains(t, decode(t, emptyW).Message, "文件夹名不能为空")

	// --- POST /:id/move 树形移动：子夹移出到根 ---
	mw := doJSON(r, "POST", fmt.Sprintf("/api/v1/library/folders/%d/move", childID), tokenA, map[string]interface{}{
		"parent_id": 0,
	})
	require.Equal(t, http.StatusOK, mw.Code, "移动失败: %s", mw.Body.String())
	moved := decode(t, mw).Data.(map[string]interface{})
	assert.Equal(t, float64(0), moved["parent_id"], "移动后应落根")

	// 移到自身 → 422「不能移动到自身」。
	selfW := doJSON(r, "POST", fmt.Sprintf("/api/v1/library/folders/%d/move", childID), tokenA, map[string]interface{}{
		"parent_id": childID,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, selfW.Code, "移到自身应 422: %s", selfW.Body.String())
	assert.Contains(t, decode(t, selfW).Message, "不能移动到自身")

	// 移到不存在的目标夹 → 404「目标文件夹不存在」。
	missingW := doJSON(r, "POST", fmt.Sprintf("/api/v1/library/folders/%d/move", childID), tokenA, map[string]interface{}{
		"parent_id": 9999999,
	})
	assert.Equal(t, http.StatusNotFound, missingW.Code, "移到不存在目标应 404: %s", missingW.Body.String())
	assert.Contains(t, decode(t, missingW).Message, "目标文件夹不存在")

	// --- DELETE 非空夹（有子夹）应拒绝 409 ---
	// 把 childID 挪回 rootID 下，让 rootID 变为非空夹。
	backW := doJSON(r, "POST", fmt.Sprintf("/api/v1/library/folders/%d/move", childID), tokenA, map[string]interface{}{
		"parent_id": rootID,
	})
	require.Equal(t, http.StatusOK, backW.Code)
	delRootW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/folders/%d", rootID), tokenA, nil)
	assert.Equal(t, http.StatusConflict, delRootW.Code, "非空夹删除应 409: %s", delRootW.Body.String())
	assert.Contains(t, decode(t, delRootW).Message, "文件夹非空")

	// --- DELETE 空夹（先删子夹）应成功 ---
	delChildW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/folders/%d", childID), tokenA, nil)
	require.Equal(t, http.StatusOK, delChildW.Code, "删除空子夹失败: %s", delChildW.Body.String())
	delRootW2 := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/folders/%d", rootID), tokenA, nil)
	require.Equal(t, http.StatusOK, delRootW2.Code, "清空后删根夹失败: %s", delRootW2.Body.String())

	lw3 := doJSON(r, "GET", "/api/v1/library/folders", tokenA, nil)
	require.Equal(t, http.StatusOK, lw3.Code)
	assert.Empty(t, decode(t, lw3).Data.([]interface{}), "全删后应为空")

	// --- 属主隔离：B 建一个夹，A 不能改名/移动/删除 B 的夹（404，不泄露存在性差异） ---
	bFolderID, _ := libFolderCreate(t, r, tokenB, "B的私密文件夹", 0)

	renameOtherW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/folders/%d", bFolderID), tokenA, map[string]interface{}{
		"name": "改名",
	})
	assert.Equal(t, http.StatusNotFound, renameOtherW.Code, "A 改 B 的夹应 404: %s", renameOtherW.Body.String())
	assert.Contains(t, decode(t, renameOtherW).Message, "文件夹不存在")

	moveOtherW := doJSON(r, "POST", fmt.Sprintf("/api/v1/library/folders/%d/move", bFolderID), tokenA, map[string]interface{}{
		"parent_id": 0,
	})
	assert.Equal(t, http.StatusNotFound, moveOtherW.Code, "A 移动 B 的夹应 404: %s", moveOtherW.Body.String())
	assert.Contains(t, decode(t, moveOtherW).Message, "文件夹不存在")

	delOtherW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/folders/%d", bFolderID), tokenA, nil)
	assert.Equal(t, http.StatusNotFound, delOtherW.Code, "A 删 B 的夹应 404: %s", delOtherW.Body.String())
	assert.Contains(t, decode(t, delOtherW).Message, "文件夹不存在")

	// B 自己仍能看到、删除自己的夹（证明上面的 404 是属主隔离而非误删）。
	bListW := doJSON(r, "GET", "/api/v1/library/folders", tokenB, nil)
	require.Equal(t, http.StatusOK, bListW.Code)
	assert.Len(t, decode(t, bListW).Data.([]interface{}), 1, "B 的夹应还在")

	// --- 未登录 401 ---
	noAuthW := doJSON(r, "GET", "/api/v1/library/folders", "", nil)
	assert.Equal(t, http.StatusUnauthorized, noAuthW.Code)

	// --- 建夹时父夹不存在 → 404 ---
	badParentW := doJSON(r, "POST", "/api/v1/library/folders", tokenA, map[string]interface{}{
		"name": "孤儿夹", "parent_id": 8888888,
	})
	assert.Equal(t, http.StatusNotFound, badParentW.Code, "父夹不存在应 404: %s", badParentW.Body.String())
	assert.Contains(t, decode(t, badParentW).Message, "父文件夹不存在")

	// --- 空名建夹 → 422 ---
	emptyCreateW := doJSON(r, "POST", "/api/v1/library/folders", tokenA, map[string]interface{}{
		"name": "",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, emptyCreateW.Code, "空名建夹应 422: %s", emptyCreateW.Body.String())
	assert.Contains(t, decode(t, emptyCreateW).Message, "文件夹名不能为空")
}

// TestLibraryTags_CRUDAndFileTagging 承接 Plan A（无独立 apptest），补 TC-11-066：
// 标签 GET/POST/PUT/DELETE 全链路 + 文件打标 PUT /library/files/:id/tags + 属主隔离
// （包括「用他人 tag_id 打自己的文件」应被静默过滤，不越权绑定）。
func TestLibraryTags_CRUDAndFileTagging(t *testing.T) {
	const phoneA = "13910000003"
	const phoneB = "13910000004"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	// 私有桶（打标需先有文件，走 uploadLibraryFile）。
	libCli, _ := newFakeS3(t, "gp-lib-tags", "")
	withFakeS3(t, nil, libCli)

	// --- GET 初始为空 ---
	lw := doJSON(r, "GET", "/api/v1/library/tags", tokenA, nil)
	require.Equal(t, http.StatusOK, lw.Code)
	assert.Empty(t, decode(t, lw).Data.([]interface{}), "新用户初始标签列表应为空")

	// --- POST 新建标签 ---
	tagID := libTagCreate(t, r, tokenA, "重要", "#ff0000")
	require.NotZero(t, tagID)

	tagID2 := libTagCreate(t, r, tokenA, "待整理", "#00ff00")
	require.NotZero(t, tagID2)

	lw2 := doJSON(r, "GET", "/api/v1/library/tags", tokenA, nil)
	require.Equal(t, http.StatusOK, lw2.Code)
	assert.Len(t, decode(t, lw2).Data.([]interface{}), 2)

	// 空名建标签 → 422。
	emptyTagW := doJSON(r, "POST", "/api/v1/library/tags", tokenA, map[string]interface{}{
		"name": "  ", "color": "#000000",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, emptyTagW.Code, "空名建标签应 422: %s", emptyTagW.Body.String())
	assert.Contains(t, decode(t, emptyTagW).Message, "标签名不能为空")

	// --- PUT 改名/改色 ---
	uw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/tags/%d", tagID), tokenA, map[string]interface{}{
		"name": "非常重要", "color": "#ffaa00",
	})
	require.Equal(t, http.StatusOK, uw.Code, "改标签失败: %s", uw.Body.String())
	updated := decode(t, uw).Data.(map[string]interface{})
	assert.Equal(t, "非常重要", updated["name"])
	assert.Equal(t, "#ffaa00", updated["color"])

	// 只改颜色不改名（name 不传，保留原名）。
	colorOnlyW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/tags/%d", tagID), tokenA, map[string]interface{}{
		"color": "#123456",
	})
	require.Equal(t, http.StatusOK, colorOnlyW.Code)
	colorOnly := decode(t, colorOnlyW).Data.(map[string]interface{})
	assert.Equal(t, "非常重要", colorOnly["name"], "未传 name 应保留原名")
	assert.Equal(t, "#123456", colorOnly["color"])

	// 改名为空串 → 422。
	emptyRenameW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/tags/%d", tagID), tokenA, map[string]interface{}{
		"name": "",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, emptyRenameW.Code, "改名为空应 422: %s", emptyRenameW.Body.String())
	assert.Contains(t, decode(t, emptyRenameW).Message, "标签名不能为空")

	// --- 属主隔离：B 建一个标签，A 不能改/删 B 的标签（404） ---
	bTagID := libTagCreate(t, r, tokenB, "B的标签", "#111111")

	renameOtherW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/tags/%d", bTagID), tokenA, map[string]interface{}{
		"name": "抢过来",
	})
	assert.Equal(t, http.StatusNotFound, renameOtherW.Code, "A 改 B 的标签应 404: %s", renameOtherW.Body.String())
	assert.Contains(t, decode(t, renameOtherW).Message, "标签不存在")

	delOtherW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/tags/%d", bTagID), tokenA, nil)
	assert.Equal(t, http.StatusNotFound, delOtherW.Code, "A 删 B 的标签应 404: %s", delOtherW.Body.String())
	assert.Contains(t, decode(t, delOtherW).Message, "标签不存在")

	// --- 文件打标：A 上传一个文件，打上自己的标签 ---
	fileID := uploadLibraryFile(t, r, tokenA, "tagme.png", "image/png", tinyPNG(t))
	require.NotZero(t, fileID)

	setW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, map[string]interface{}{
		"tag_ids": []uint{tagID, tagID2},
	})
	require.Equal(t, http.StatusOK, setW.Code, "打标失败: %s", setW.Body.String())

	getTagsW := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, nil)
	require.Equal(t, http.StatusOK, getTagsW.Code)
	gotIDs := libFileTagIDs(t, getTagsW)
	assert.ElementsMatch(t, []uint{tagID, tagID2}, gotIDs, "应回显刚打的两个标签")

	// 覆盖式打标：换成只有 tagID2（应替换而非追加）。
	overwriteW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, map[string]interface{}{
		"tag_ids": []uint{tagID2},
	})
	require.Equal(t, http.StatusOK, overwriteW.Code)
	getTagsW2 := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, nil)
	require.Equal(t, http.StatusOK, getTagsW2.Code)
	gotIDs2 := libFileTagIDs(t, getTagsW2)
	assert.ElementsMatch(t, []uint{tagID2}, gotIDs2, "覆盖式打标应替换而非追加")

	// 用他人（B）的 tag_id 打自己的文件 → 应被静默过滤，不越权绑定别人的标签。
	mixedW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, map[string]interface{}{
		"tag_ids": []uint{tagID, bTagID},
	})
	require.Equal(t, http.StatusOK, mixedW.Code, "含他人 tag_id 的请求本身不应报错（静默过滤）: %s", mixedW.Body.String())
	getTagsW3 := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, nil)
	require.Equal(t, http.StatusOK, getTagsW3.Code)
	gotIDs3 := libFileTagIDs(t, getTagsW3)
	assert.ElementsMatch(t, []uint{tagID}, gotIDs3, "他人标签 ID 应被过滤，不应绑定成功")

	// 打标到他人（B）的文件 → 404（属主校验先于标签过滤）。
	bFileID := uploadLibraryFile(t, r, tokenB, "b-file.png", "image/png", tinyPNG(t))
	require.NotZero(t, bFileID)
	setOtherFileW := doJSON(r, "PUT", fmt.Sprintf("/api/v1/library/files/%d/tags", bFileID), tokenA, map[string]interface{}{
		"tag_ids": []uint{tagID},
	})
	assert.Equal(t, http.StatusNotFound, setOtherFileW.Code, "A 给 B 的文件打标应 404: %s", setOtherFileW.Body.String())
	assert.Contains(t, decode(t, setOtherFileW).Message, "文件不存在")

	getOtherFileTagsW := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/tags", bFileID), tokenA, nil)
	assert.Equal(t, http.StatusNotFound, getOtherFileTagsW.Code, "A 查 B 的文件标签应 404: %s", getOtherFileTagsW.Body.String())
	assert.Contains(t, decode(t, getOtherFileTagsW).Message, "文件不存在")

	// --- DELETE 标签后应解除文件关联 ---
	delTagW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/tags/%d", tagID), tokenA, nil)
	require.Equal(t, http.StatusOK, delTagW.Code, "删标签失败: %s", delTagW.Body.String())
	getTagsW4 := doJSON(r, "GET", fmt.Sprintf("/api/v1/library/files/%d/tags", fileID), tokenA, nil)
	require.Equal(t, http.StatusOK, getTagsW4.Code)
	assert.Empty(t, libFileTagIDs(t, getTagsW4), "删除标签后文件不应再关联该标签")

	// 删除不存在的标签（已删过一次）→ 404。
	delAgainW := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/library/tags/%d", tagID), tokenA, nil)
	assert.Equal(t, http.StatusNotFound, delAgainW.Code, "重复删除应 404: %s", delAgainW.Body.String())
	assert.Contains(t, decode(t, delAgainW).Message, "标签不存在")

	// --- 未登录 401 ---
	noAuthW := doJSON(r, "GET", "/api/v1/library/tags", "", nil)
	assert.Equal(t, http.StatusUnauthorized, noAuthW.Code)
}
