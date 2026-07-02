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

// app_admin_governance_test.go 覆盖 Plan B 任务 14「运营治理」（TC-12-050/051/052）：
// GET /api/v1/admin/apps（跨用户查看，app:view）、POST /api/v1/admin/apps/batch-delete
// （按 meta.user_id 定属主删除，app:manage；无 meta 跳过；空数组 400）、以及仅持 app:view
// 时对 batch-delete 的越权拒绝（403）。这三点此前无接口层自动化覆盖，本文件为首次补齐。
//
// 权限 staff 用 createRole/assignRoles（见 staff_rbac_test.go）现造仅含单一权限的角色，
// 不复用内置超管，确保断言真正绑定 PermissionMiddleware("app:view"/"app:manage") 这一守卫。

// govUploadAndFinalizeApp 走完整「素材库直传 → finalize」链路，为运营治理用例造一条
// parse_status=ready 的 app_user_meta 行，返回该文件的 file_id。调用前需已 withFakeS3。
func govUploadAndFinalizeApp(t *testing.T, r *gin.Engine, token, name, pkg, version, appName string) uint {
	t.Helper()
	apkBytes := minimalXAPK(t, pkg, version, appName)
	fileID := uploadLibraryFile(t, r, token, name, "application/vnd.android.package-archive", apkBytes)
	fw := doJSON(r, "POST", fmt.Sprintf("/api/v1/app/user/%d/finalize", fileID), token, nil)
	require.Equal(t, http.StatusOK, fw.Code, "finalize 失败: %s", fw.Body.String())
	dto := decode(t, fw).Data.(map[string]interface{})
	require.Equal(t, "ready", dto["parse_status"], "预置应用应 finalize 为 ready: %v", dto)
	return fileID
}

// govCreateStaffWithPerm 造一个非超管 staff，仅授予传入的单一权限，返回其登录令牌与 staffID。
// suffix 保证角色名/用户名跨用例唯一，t.Cleanup 由调用方负责精确清理。
func govCreateStaffWithPerm(t *testing.T, r *gin.Engine, adminTok, suffix, perm string) (tok string, staffID int, roleID int) {
	t.Helper()
	username := "gov_staff_" + govPermToName(perm) + "_" + suffix
	roleName := "gov_role_" + govPermToName(perm) + "_" + suffix
	roleID = createRole(t, r, adminTok, roleName, []string{perm})
	createUser(t, r, adminTok, username, "pass123", false)
	staffID = staffIDByUsername(t, username)
	assignRoles(t, r, adminTok, staffID, []int{roleID})
	tok = login(t, r, username, "pass123")
	return tok, staffID, roleID
}

// govPermToName 把权限标识转换成可用作用户名/角色名一部分的短字符串（去掉冒号）。
func govPermToName(perm string) string {
	out := make([]rune, 0, len(perm))
	for _, c := range perm {
		if c == ':' {
			out = append(out, '_')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// govCleanupStaff 精确清理 govCreateStaffWithPerm 造的 staff + role 关联行。
func govCleanupStaff(staffID, roleID int) {
	db := framework.DB
	if staffID != 0 {
		db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", staffID)
		db.Exec("DELETE FROM staff WHERE id = ?", staffID)
	}
	if roleID != 0 {
		db.Exec("DELETE FROM role_permissions WHERE role_id = ?", roleID)
		db.Exec("DELETE FROM staff_roles WHERE role_id = ?", roleID)
		db.Exec("DELETE FROM roles WHERE id = ?", roleID)
	}
}

// TestAdminAppGovernance_CrossUserListIncludesUploader 覆盖 TC-12-050：
// 持 app:view 的 staff 调 GET /api/v1/admin/apps，应以 app_user_meta 为权威集合列出全部
// 用户应用（含跨用户），且每行应带上传者信息（user_phone/user_nickname）与正确 user_id，
// 不与应用市场（app_market）混列。
func TestAdminAppGovernance_CrossUserListIncludesUploader(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	const phoneA = "13914000001"
	const phoneB = "13914000002"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	// finalize 需私有桶回读、公有桶传图标。
	pubCli, _ := newFakeS3(t, "gp-gov-pub-050", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-gov-lib-050", "")
	withFakeS3(t, pubCli, libCli)

	fileA := govUploadAndFinalizeApp(t, r, tokenA, "a.xapk", "com.gp.gov.a", "1.0.0", "GovAppA")
	fileB := govUploadAndFinalizeApp(t, r, tokenB, "b.xapk", "com.gp.gov.b", "2.0.0", "GovAppB")

	viewTok, viewStaffID, viewRoleID := govCreateStaffWithPerm(t, r, admin, suffix, "app:view")
	t.Cleanup(func() { govCleanupStaff(viewStaffID, viewRoleID) })

	w := doJSON(r, "GET", "/api/v1/admin/apps", viewTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "app:view 应能查看跨用户应用列表: %s", w.Body.String())
	list, ok := decode(t, w).Data.([]interface{})
	require.True(t, ok, "跨用户应用列表应是数组: %v", decode(t, w).Data)

	var gotA, gotB map[string]interface{}
	for _, item := range list {
		m := item.(map[string]interface{})
		switch uint(m["file_id"].(float64)) {
		case fileA:
			gotA = m
		case fileB:
			gotB = m
		}
	}
	require.NotNil(t, gotA, "跨用户列表应包含 A 的应用 file_id=%d", fileA)
	require.NotNil(t, gotB, "跨用户列表应包含 B 的应用 file_id=%d", fileB)

	assert.Equal(t, float64(uidA), gotA["user_id"], "A 的条目 user_id 应等于 A 的用户 ID")
	assert.Equal(t, phoneA, gotA["user_phone"], "A 的条目应带上传者手机号")
	assert.Equal(t, "com.gp.gov.a", gotA["package_name"])
	assert.Equal(t, "ready", gotA["parse_status"])

	assert.Equal(t, float64(uidB), gotB["user_id"], "B 的条目 user_id 应等于 B 的用户 ID")
	assert.Equal(t, phoneB, gotB["user_phone"], "B 的条目应带上传者手机号")
	assert.Equal(t, "com.gp.gov.b", gotB["package_name"])

	// 不含市场应用：admin/apps 与 admin/apps/market 是两张表两个桶（app_user_meta vs app_market），
	// 用两个截然不同的包名断言互不混列——市场包名不应窜进用户应用列表，反之亦然。
	mw := doJSON(r, "GET", "/api/v1/admin/apps/market", viewTok, nil)
	require.Equal(t, http.StatusOK, mw.Code, "app:view 应能查看市场列表: %s", mw.Body.String())
	marketList, _ := decode(t, mw).Data.([]interface{})
	for _, item := range marketList {
		m := item.(map[string]interface{})
		assert.NotEqual(t, "com.gp.gov.a", m["package_name"], "用户应用不应出现在市场列表")
		assert.NotEqual(t, "com.gp.gov.b", m["package_name"], "用户应用不应出现在市场列表")
		_, hasUserID := m["user_id"]
		assert.False(t, hasUserID, "市场条目不应带 user_id（市场为平台资产，非用户属主）")
	}
}

// TestAdminAppGovernance_BatchDeleteResolvesOwnerAndSkipsOrphan 覆盖 TC-12-051：
// 持 app:manage 的 staff 调 POST /api/v1/admin/apps/batch-delete：
//   - 按 meta.user_id 定属主 → DeleteFileForUser 释放配额 + 删 app_user_meta 行；
//   - 传入一个没有 app_user_meta 行的素材库文件 ID（无法定属主）→ 该 ID 被跳过，不报错、
//     该文件在 library_files 中保持不受影响；
//   - 空 file_ids 数组 → 400。
func TestAdminAppGovernance_BatchDeleteResolvesOwnerAndSkipsOrphan(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	const phone = "13914000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	pubCli, _ := newFakeS3(t, "gp-gov-pub-051", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-gov-lib-051", "")
	withFakeS3(t, pubCli, libCli)

	// 有 meta 的应用（可定属主）。
	fileWithMeta := govUploadAndFinalizeApp(t, r, token, "with-meta.xapk", "com.gp.gov.c1", "1.0.0", "GovAppC1")

	// 无 meta 的素材库文件：仅上传不 finalize，不落 app_user_meta 行，模拟「无法定属主」场景。
	fileNoMeta := uploadLibraryFile(t, r, token, "orphan.xapk", "application/vnd.android.package-archive", tinyPNG(t))
	var metaCount int64
	require.NoError(t, framework.DB.Table("app_user_meta").
		Where("library_file_id = ?", fileNoMeta).Count(&metaCount).Error)
	require.Zero(t, metaCount, "前置条件：orphan 文件不应有 app_user_meta 行")

	manageTok, manageStaffID, manageRoleID := govCreateStaffWithPerm(t, r, admin, suffix, "app:manage")
	t.Cleanup(func() { govCleanupStaff(manageStaffID, manageRoleID) })

	// 空数组 → 400「未选择应用」。
	wEmpty := doJSON(r, "POST", "/api/v1/admin/apps/batch-delete", manageTok,
		map[string]interface{}{"file_ids": []uint{}})
	require.Equal(t, http.StatusBadRequest, wEmpty.Code, "空 file_ids 应 400: %s", wEmpty.Body.String())
	assert.Contains(t, decode(t, wEmpty).Message, "未选择应用")

	// 混合传入：有 meta 的 + 无 meta 的孤儿文件。
	wDel := doJSON(r, "POST", "/api/v1/admin/apps/batch-delete", manageTok,
		map[string]interface{}{"file_ids": []uint{fileWithMeta, fileNoMeta}})
	require.Equal(t, http.StatusOK, wDel.Code, "混合批量删除应成功（孤儿项跳过）: %s", wDel.Body.String())

	// 有 meta 的应用：meta 行应被删、素材库文件应被软删（属主定位正确，走了 DeleteFileForUser）。
	var metaAfter int64
	require.NoError(t, framework.DB.Table("app_user_meta").
		Where("library_file_id = ?", fileWithMeta).Count(&metaAfter).Error)
	assert.Zero(t, metaAfter, "定属主删除后 app_user_meta 行应消失")

	var libStatus string
	require.NoError(t, framework.DB.Table("library_files").
		Where("id = ?", fileWithMeta).Select("status").Scan(&libStatus).Error)
	assert.Equal(t, "deleted", libStatus, "定属主删除应真实软删素材库文件")

	// 孤儿文件：因无法定属主被跳过，素材库文件应原样保留（未被误删）。
	var orphanStatus string
	require.NoError(t, framework.DB.Table("library_files").
		Where("id = ?", fileNoMeta).Select("status").Scan(&orphanStatus).Error)
	assert.Equal(t, "active", orphanStatus, "无 meta 无法定属主的文件应被跳过，不应被删除")
}

// TestAdminAppGovernance_BatchDeleteForbiddenForViewOnly 覆盖 TC-12-052：
// 仅持 app:view（无 app:manage）的 staff 调 POST /api/v1/admin/apps/batch-delete → 403，
// 且 message 含「权限不足」（PermissionMiddleware 的具体拒绝文案），目标应用不应被删除。
func TestAdminAppGovernance_BatchDeleteForbiddenForViewOnly(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	const phone = "13914000004"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	pubCli, _ := newFakeS3(t, "gp-gov-pub-052", "https://cdn.test/")
	libCli, _ := newFakeS3(t, "gp-gov-lib-052", "")
	withFakeS3(t, pubCli, libCli)

	fileID := govUploadAndFinalizeApp(t, r, token, "protected.xapk", "com.gp.gov.d1", "1.0.0", "GovAppD1")

	viewTok, viewStaffID, viewRoleID := govCreateStaffWithPerm(t, r, admin, suffix, "app:view")
	t.Cleanup(func() { govCleanupStaff(viewStaffID, viewRoleID) })

	w := doJSON(r, "POST", "/api/v1/admin/apps/batch-delete", viewTok,
		map[string]interface{}{"file_ids": []uint{fileID}})
	require.Equal(t, http.StatusForbidden, w.Code, "仅持 app:view 不应能批量删除应用: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "权限不足", "应返回权限不足的具体拒绝文案")

	// 越权请求应被 PermissionMiddleware 挡在 handler 之前：素材库文件与 meta 行均应原样保留。
	var libStatus string
	require.NoError(t, framework.DB.Table("library_files").
		Where("id = ?", fileID).Select("status").Scan(&libStatus).Error)
	assert.Equal(t, "active", libStatus, "越权删除被拒后文件应仍 active")

	var metaCount int64
	require.NoError(t, framework.DB.Table("app_user_meta").
		Where("library_file_id = ?", fileID).Count(&metaCount).Error)
	assert.Equal(t, int64(1), metaCount, "越权删除被拒后 app_user_meta 行应仍存在")
}
