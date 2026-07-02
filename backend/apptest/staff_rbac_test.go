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

// TestStaffRBAC_MintAndEscalationGuards 回归后台 RBAC 授权收敛层：
// S1 铸造越权角色、A2 创建时自封超管、A1 更新时提超管位、A3 赋越权角色，均应被挡。
func TestStaffRBAC_MintAndEscalationGuards(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	db := framework.DB

	// 唯一命名，避免污染共享表。
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	limitedName := "rbac_limited_" + suffix    // 受限管理员
	victimName := "rbac_victim_" + suffix      // 被更新的普通员工
	roleName := "rbac_role_limited_" + suffix  // 受限管理员自身角色
	privRoleName := "rbac_role_priv_" + suffix // 含 billing:manage 的高权角色

	// t.Cleanup 逆序清理本用例造的行（先删关联，再删主体）。
	var limitedID, victimID, roleID, privRoleID int
	t.Cleanup(func() {
		if limitedID != 0 {
			db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", limitedID)
			db.Exec("DELETE FROM staff WHERE id = ?", limitedID)
		}
		if victimID != 0 {
			db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", victimID)
			db.Exec("DELETE FROM staff WHERE id = ?", victimID)
		}
		if roleID != 0 {
			db.Exec("DELETE FROM role_permissions WHERE role_id = ?", roleID)
			db.Exec("DELETE FROM staff_roles WHERE role_id = ?", roleID)
			db.Exec("DELETE FROM roles WHERE id = ?", roleID)
		}
		if privRoleID != 0 {
			db.Exec("DELETE FROM role_permissions WHERE role_id = ?", privRoleID)
			db.Exec("DELETE FROM staff_roles WHERE role_id = ?", privRoleID)
			db.Exec("DELETE FROM roles WHERE id = ?", privRoleID)
		}
	})

	// 1) 超管建一个受限角色：含 staff/role 相关权限，但绝不含 billing:manage。
	limitedPerms := []string{"staff:create", "staff:edit", "role:create", "role:view"}
	roleID = createRole(t, r, admin, roleName, limitedPerms)

	// 2) 超管建一个高权角色：含 billing:manage（用于 A3 越权赋角色）。
	privRoleID = createRole(t, r, admin, privRoleName, []string{"billing:manage"})

	// 3) 超管建两个普通 staff（is_superuser=false），并给受限管理员赋受限角色。
	createUser(t, r, admin, limitedName, "pass123", false)
	createUser(t, r, admin, victimName, "pass123", false)
	limitedID = staffIDByUsername(t, limitedName)
	victimID = staffIDByUsername(t, victimName)
	assignRoles(t, r, admin, limitedID, []int{roleID})

	// 受限管理员登录换令牌。
	limitedTok := login(t, r, limitedName, "pass123")

	// --- S1：铸造越权角色 ---
	// 含自身没有的 billing:manage → 403。
	wDeny := doJSON(r, "POST", "/api/v1/role/create", limitedTok, map[string]interface{}{
		"name":        "rbac_mint_deny_" + suffix,
		"permissions": []string{"role:view", "billing:manage"},
	})
	require.Equal(t, http.StatusForbidden, wDeny.Code, "S1: 铸造含自身没有权限的角色应 403，实际: %s", wDeny.Body.String())

	// 只含自身子集(role:view) → 200，并清理造出来的角色。
	wOK := doJSON(r, "POST", "/api/v1/role/create", limitedTok, map[string]interface{}{
		"name":        "rbac_mint_ok_" + suffix,
		"permissions": []string{"role:view"},
	})
	require.Equal(t, http.StatusOK, wOK.Code, "S1: 铸造自身权限子集的角色应 200，实际: %s", wOK.Body.String())
	mintedID := int(decode(t, wOK).Data.(map[string]interface{})["id"].(float64))
	t.Cleanup(func() {
		db.Exec("DELETE FROM role_permissions WHERE role_id = ?", mintedID)
		db.Exec("DELETE FROM staff_roles WHERE role_id = ?", mintedID)
		db.Exec("DELETE FROM roles WHERE id = ?", mintedID)
	})

	// --- A2：创建时自封超管 ---
	// 受限管理员创建 staff 传 is_superuser:true → 200 但落库/响应均为 false。
	newStaffName := "rbac_a2_" + suffix
	wA2 := doJSON(r, "POST", "/api/v1/staff/create", limitedTok, map[string]interface{}{
		"username":     newStaffName,
		"password":     "pass123",
		"is_active":    true,
		"is_superuser": true,
	})
	require.Equal(t, http.StatusOK, wA2.Code, "A2: 受限管理员创建 staff 应 200，实际: %s", wA2.Body.String())
	a2Data := decode(t, wA2).Data.(map[string]interface{})
	require.False(t, a2Data["is_superuser"].(bool), "A2: 响应 is_superuser 应被强制为 false")
	a2ID := int(a2Data["id"].(float64))
	t.Cleanup(func() {
		db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", a2ID)
		db.Exec("DELETE FROM staff WHERE id = ?", a2ID)
	})
	require.False(t, staffIsSuperuser(t, a2ID), "A2: 落库 staff.is_superuser 应为 false")

	// --- A1：更新时提超管位 ---
	// 受限管理员 PUT 普通员工传 is_superuser:true → 目标仍 false。
	wA1 := doJSON(r, "PUT", fmt.Sprintf("/api/v1/staff/update/%d", victimID), limitedTok, map[string]interface{}{
		"is_superuser": true,
	})
	require.Equal(t, http.StatusOK, wA1.Code, "A1: 受限管理员更新普通员工应 200，实际: %s", wA1.Body.String())
	require.False(t, staffIsSuperuser(t, victimID), "A1: 目标 staff.is_superuser 应仍为 false")

	// --- A3：赋越权角色 ---
	// 受限管理员 PUT 给员工 role_ids 指向含 billing:manage 的高权角色 → 403。
	wA3 := doJSON(r, "PUT", fmt.Sprintf("/api/v1/staff/update/%d", victimID), limitedTok, map[string]interface{}{
		"role_ids": []int{privRoleID},
	})
	require.Equal(t, http.StatusForbidden, wA3.Code, "A3: 赋含自身未拥有权限的角色应 403，实际: %s", wA3.Body.String())
}

// --- 本用例局部辅助（apptest 包内，避免与既有 helper 重名；Task 3 复用这些，勿重复定义）---

// createRole 以超管令牌经真实 HTTP 建角色，返回其 ID。
func createRole(t *testing.T, r *gin.Engine, adminTok, name string, perms []string) int {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/role/create", adminTok, map[string]interface{}{
		"name":        name,
		"permissions": perms,
	})
	require.Equal(t, http.StatusOK, w.Code, "创建角色失败: %s", w.Body.String())
	return int(decode(t, w).Data.(map[string]interface{})["id"].(float64))
}

// assignRoles 以超管令牌给指定 staff 赋角色（经 PUT /staff/update/:id 的 role_ids）。
func assignRoles(t *testing.T, r *gin.Engine, adminTok string, staffID int, roleIDs []int) {
	t.Helper()
	w := doJSON(r, "PUT", fmt.Sprintf("/api/v1/staff/update/%d", staffID), adminTok, map[string]interface{}{
		"role_ids": roleIDs,
	})
	require.Equal(t, http.StatusOK, w.Code, "赋角色失败: %s", w.Body.String())
}

// staffIDByUsername 直查 staff 表拿 ID（唯一用户名）。Task 3 复用此签名。
func staffIDByUsername(t *testing.T, username string) int {
	t.Helper()
	var id int
	require.NoError(t, framework.DB.Raw("SELECT id FROM staff WHERE username = ?", username).Scan(&id).Error)
	require.NotZero(t, id, "未找到 staff: %s", username)
	return id
}

// staffIsSuperuser 直查 staff 表的 is_superuser。
func staffIsSuperuser(t *testing.T, id int) bool {
	t.Helper()
	var v bool
	require.NoError(t, framework.DB.Raw("SELECT is_superuser FROM staff WHERE id = ?", id).Scan(&v).Error)
	return v
}

// grantStaffDelete 直接经原生 SQL 给某员工挂一个仅含 staff:delete 权限的角色（造数据，非授权路径）。
// 返回清理函数，删除本次造出的 role/role_permissions/staff_roles 行。
func grantStaffDelete(t *testing.T, staffID int, roleName string) func() {
	t.Helper()
	db := framework.DB
	require.NoError(t, db.Exec(
		"INSERT INTO roles (name, description, is_builtin) VALUES (?, '', ?)", roleName, false).Error)
	var roleID int
	require.NoError(t, db.Table("roles").Where("name = ?", roleName).Pluck("id", &roleID).Error)
	require.NotZero(t, roleID)
	require.NoError(t, db.Exec(
		"INSERT INTO role_permissions (role_id, permission) VALUES (?, ?)", roleID, "staff:delete").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO staff_roles (staff_id, role_id) VALUES (?, ?)", staffID, roleID).Error)
	return func() {
		db.Exec("DELETE FROM staff_roles WHERE role_id = ?", roleID)
		db.Exec("DELETE FROM role_permissions WHERE role_id = ?", roleID)
		db.Exec("DELETE FROM roles WHERE id = ?", roleID)
	}
}

// A5：DeleteStaffChecked 删除守卫（HTTP 层，经 DELETE /api/v1/staff/delete/:id）。
// 受限管理员 = 非超管但持 staff:delete，通过 PermissionMiddleware 后由 *Checked 拦截越权删除。
func TestDeleteStaffGuards_HTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	// 唯一命名，避免污染共享表、避免重跑（清理被打断）时 409。
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// 造一个受限管理员（非超管，稍后挂 staff:delete）
	restricted := "rbac_del_actor_" + suffix
	createUser(t, r, admin, restricted, "pass123", false)
	restrictedID := staffIDByUsername(t, restricted)
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM staff WHERE username = ?", restricted) })

	cleanupRole := grantStaffDelete(t, restrictedID, "rbac_del_role_"+suffix)
	t.Cleanup(cleanupRole)

	actorTok := login(t, r, restricted, "pass123")

	// A5-a：受限管理员删超管（种子 admin 是超管）→ 403（非超管无权删超管）。
	adminID := staffIDByUsername(t, "admin")
	wa := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/staff/delete/%d", adminID), actorTok, nil)
	assert.Equal(t, http.StatusForbidden, wa.Code, "受限管理员删超管应 403: %s", wa.Body.String())

	// A5-b：受限管理员删自己 → 403（禁止自删）。
	wb := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/staff/delete/%d", restrictedID), actorTok, nil)
	assert.Equal(t, http.StatusForbidden, wb.Code, "删自己应 403: %s", wb.Body.String())

	// A5-c（末位超管守卫的可达半边）：超管 admin 删一个“非末位”超管 → 成功（count>1，守卫放行）。
	// 说明：count<=1 的 409 分支在生产 HTTP 下不可达（唯一超管即操作者自身，先被自删守卫拦截），
	// 且 apptest 不得截断/降级共享的 admin 超管行；此处断言超管可删非末位超管这一守卫放行路径。
	secondSuper := "rbac_second_super_" + suffix
	createUser(t, r, admin, secondSuper, "pass123", true) // admin 为超管 → CreateStaffChecked 保留 is_superuser=true
	secondSuperID := staffIDByUsername(t, secondSuper)
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM staff WHERE username = ?", secondSuper) })

	wc := doJSON(r, "DELETE", fmt.Sprintf("/api/v1/staff/delete/%d", secondSuperID), admin, nil)
	assert.Equal(t, http.StatusOK, wc.Code, "超管删非末位超管应成功: %s", wc.Body.String())
	// 已删除：库中该行应消失。
	var cnt int64
	framework.DB.Table("staff").Where("id = ?", secondSuperID).Count(&cnt)
	assert.Equal(t, int64(0), cnt, "非末位超管应已被删除")
}

// A4/X2：禁用前台用户后，其旧 user 令牌立即失效（token_version 递增）且无法再登录。
func TestDisabledUserTokenAndLoginRejected(t *testing.T) {
	const phone = "13900050051" // 唯一手机号，避免污染共享 users 表
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM users WHERE phone = ?", phone) })

	r := setupRouter()
	admin := adminToken(t, r)

	userTok := registerUser(t, r, phone) // 密码固定 "pass123"
	// 前置健壮性：禁用前旧令牌可用。
	require.Equal(t, http.StatusOK, doJSON(r, "GET", "/api/v1/user/me", userTok, nil).Code)

	// 取前台用户 id（users 表）。
	var userID int
	require.NoError(t, framework.DB.Table("users").Where("phone = ?", phone).Pluck("id", &userID).Error)
	require.NotZero(t, userID)

	// adminToken PUT 禁用：is_active=false（AdminStatusRequest.IsActive 为 *bool binding:"required"）。
	wd := doJSON(r, "PUT", fmt.Sprintf("/api/v1/admin/users/%d/status", userID), admin,
		map[string]interface{}{"is_active": false})
	require.Equal(t, http.StatusOK, wd.Code, "禁用应成功: %s", wd.Body.String())

	// X2：禁用后旧 user 令牌打 /user/me → 401（token_version 不匹配，UserAuth 拒绝）。
	assert.Equal(t, http.StatusUnauthorized,
		doJSON(r, "GET", "/api/v1/user/me", userTok, nil).Code, "禁用后旧令牌应失效")

	// A4：被禁用手机号再登录 → 403（Authenticate 对 !IsActive 返回 Forbidden）。
	wl := doJSON(r, "POST", "/api/v1/user/auth/login", "", map[string]string{
		"phone": phone, "password": "pass123",
	})
	assert.Equal(t, http.StatusForbidden, wl.Code, "被禁用账号登录应 403: %s", wl.Body.String())
}
