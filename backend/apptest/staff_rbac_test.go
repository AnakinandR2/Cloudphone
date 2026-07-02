package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
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
