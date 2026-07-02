package staff_test

import (
	"testing"

	"manager-backend/framework/apperr"
	staff "manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归测试：后台 RBAC 最小权限边界（审查报告 S1/A1/A2/A3/A4/A5）。
// 覆盖 handler 唯一入口的 *Checked 方法；actorSuper=false 模拟非超管操作者。
// 按 backend/CLAUDE.md 约定：不 CleanTable 共享表，仅用唯一命名 + 按 ID 清理自己造的数据。

// mkUser 创建一个测试用户并登记按 ID 清理。
func mkUser(t *testing.T, username string, super bool) *staff.Staff {
	t.Helper()
	u := createTestUser(t, username, "p", super)
	t.Cleanup(func() { _ = staff.Service.DeleteStaff(u.ID) })
	return u
}

// mkRole 创建一个含指定权限的角色并登记按 ID 清理。
func mkRole(t *testing.T, name string, perms []string) *staff.Role {
	t.Helper()
	r, err := staff.RoleService.Create(&staff.RoleCreate{Name: name, Permissions: perms})
	require.NoError(t, err)
	t.Cleanup(func() { _ = staff.RoleService.Delete(r.ID) })
	return r
}

// grantRole 给用户赋一个含指定权限的新角色（测试造数据，非授权路径）。
func grantRole(t *testing.T, userID int, roleName string, perms []string) {
	t.Helper()
	role := mkRole(t, roleName, perms)
	_, err := staff.Service.UpdateStaffWithForm(userID, &staff.StaffUpdate{RoleIDs: &[]int{role.ID}})
	require.NoError(t, err)
}

// A2：非超管创建时不得造出超管账号。
func TestCreateStaffChecked_NonSuperCannotCreateSuperuser(t *testing.T) {
	u, err := staff.Service.CreateStaffChecked(&staff.StaffCreate{
		Username: "esc-create", Password: "p", IsActive: true, IsSuperuser: true,
	}, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = staff.Service.DeleteStaff(u.ID) })
	assert.False(t, u.IsSuperuser, "非超管创建时 is_superuser 必须被强制为 false")
}

func TestCreateStaffChecked_SuperCanCreateSuperuser(t *testing.T) {
	u, err := staff.Service.CreateStaffChecked(&staff.StaffCreate{
		Username: "sup-create", Password: "p", IsActive: true, IsSuperuser: true,
	}, true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = staff.Service.DeleteStaff(u.ID) })
	assert.True(t, u.IsSuperuser)
}

// A1：非超管不得把 is_superuser 改为 true（自我提权）。
func TestUpdateStaffChecked_NonSuperCannotSelfPromote(t *testing.T) {
	u := mkUser(t, "esc-update", false)
	yes := true
	updated, err := staff.Service.UpdateStaffChecked(u.ID, &staff.StaffUpdate{IsSuperuser: &yes}, u.ID, false)
	require.NoError(t, err)
	assert.False(t, updated.IsSuperuser, "非超管不能把 is_superuser 改为 true")
}

func TestUpdateStaffChecked_SuperCanSetSuperuser(t *testing.T) {
	u := mkUser(t, "sup-update", false)
	yes := true
	updated, err := staff.Service.UpdateStaffChecked(u.ID, &staff.StaffUpdate{IsSuperuser: &yes}, 1, true)
	require.NoError(t, err)
	assert.True(t, updated.IsSuperuser)
}

// A3：非超管不得给（自己/他人）赋含越权权限的角色。
func TestUpdateStaffChecked_NonSuperCannotAssignPrivilegedRole(t *testing.T) {
	actor := mkUser(t, "actor-a3", false)
	grantRole(t, actor.ID, "actor-a3-role", []string{"example:view"})
	powerful := mkRole(t, "priv-role", []string{"staff:delete"})
	_, err := staff.Service.UpdateStaffChecked(actor.ID, &staff.StaffUpdate{RoleIDs: &[]int{powerful.ID}}, actor.ID, false)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

func TestUpdateStaffChecked_NonSuperCanAssignSubsetRole(t *testing.T) {
	actor := mkUser(t, "actor-a3b", false)
	grantRole(t, actor.ID, "actor-a3b-role", []string{"example:view", "example:edit"})
	subset := mkRole(t, "subset-role", []string{"example:view"})
	_, err := staff.Service.UpdateStaffChecked(actor.ID, &staff.StaffUpdate{RoleIDs: &[]int{subset.ID}}, actor.ID, false)
	require.NoError(t, err, "自身权限子集内的角色应允许分配")
}

// S1：非超管创建角色时不得铸造含越权权限的角色。
func TestRoleCreateChecked_NonSuperCannotMintPrivilegedRole(t *testing.T) {
	actor := mkUser(t, "actor-s1", false)
	grantRole(t, actor.ID, "actor-s1-role", []string{"example:view"})
	_, err := staff.RoleService.CreateChecked(
		&staff.RoleCreate{Name: "mint-priv", Permissions: []string{"billing:manage"}}, actor.ID, false)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

func TestRoleCreateChecked_SubsetAllowed(t *testing.T) {
	actor := mkUser(t, "actor-s1b", false)
	grantRole(t, actor.ID, "actor-s1b-role", []string{"example:view", "example:create"})
	role, err := staff.RoleService.CreateChecked(
		&staff.RoleCreate{Name: "mint-subset", Permissions: []string{"example:view"}}, actor.ID, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = staff.RoleService.Delete(role.ID) })
	assert.NotZero(t, role.ID)
}

func TestRoleCreateChecked_SuperUnrestricted(t *testing.T) {
	role, err := staff.RoleService.CreateChecked(
		&staff.RoleCreate{Name: "super-mint", Permissions: []string{"billing:manage", "staff:delete"}}, 1, true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = staff.RoleService.Delete(role.ID) })
	assert.NotZero(t, role.ID)
}

// S1（Update）：非超管更新角色时不得引入越权权限。
func TestRoleUpdateChecked_NonSuperCannotEscalatePermissions(t *testing.T) {
	actor := mkUser(t, "actor-s1u", false)
	grantRole(t, actor.ID, "actor-s1u-role", []string{"example:view"})
	target := mkRole(t, "edit-target", []string{"example:view"})
	_, err := staff.RoleService.UpdateChecked(target.ID,
		&staff.RoleUpdate{Name: "edit-target", Permissions: []string{"example:view", "billing:manage"}}, actor.ID, false)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

// A5（更新侧对称保护）：非超管不得修改超管账号（改密/禁用 → 账号接管）。
func TestUpdateStaffChecked_NonSuperCannotModifySuperuser(t *testing.T) {
	super := mkUser(t, "target-super-upd", true)
	actor := mkUser(t, "actor-upd", false)

	_, err := staff.Service.UpdateStaffChecked(super.ID, &staff.StaffUpdate{Password: "attacker-pw"}, actor.ID, false)
	require.Error(t, err, "非超管改超管密码应被拒")
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))

	no := false
	_, err = staff.Service.UpdateStaffChecked(super.ID, &staff.StaffUpdate{IsActive: &no}, actor.ID, false)
	require.Error(t, err, "非超管禁用超管应被拒")
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

// A5：删除保护。
func TestDeleteStaffChecked_CannotDeleteSelf(t *testing.T) {
	u := mkUser(t, "self-del", false)
	err := staff.Service.DeleteStaffChecked(u.ID, u.ID, false)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

func TestDeleteStaffChecked_NonSuperCannotDeleteSuperuser(t *testing.T) {
	super := mkUser(t, "target-super", true)
	actor := mkUser(t, "actor-del", false)
	err := staff.Service.DeleteStaffChecked(super.ID, actor.ID, false)
	require.Error(t, err)
	assert.Equal(t, apperr.KindForbidden, apperr.KindOf(err))
}

func TestDeleteStaffChecked_SuperCanDeleteNormalUser(t *testing.T) {
	super := mkUser(t, "actor-super", true)
	victim := mkUser(t, "normal-victim", false)
	err := staff.Service.DeleteStaffChecked(victim.ID, super.ID, true)
	require.NoError(t, err)
	_, err = staff.Service.GetStaffByID(victim.ID)
	require.Error(t, err, "目标应已被删除")
}

// A4：禁用递增 token_version 使旧令牌失效；普通更新不递增（避免误登出）。
func TestUpdateStaffChecked_DisableBumpsTokenVersion(t *testing.T) {
	u := mkUser(t, "disable-me", false)
	tvBefore, err := staff.Service.CurrentTokenVersion(u.ID)
	require.NoError(t, err)
	no := false
	_, err = staff.Service.UpdateStaffChecked(u.ID, &staff.StaffUpdate{IsActive: &no}, 1, true)
	require.NoError(t, err)
	tvAfter, err := staff.Service.CurrentTokenVersion(u.ID)
	require.NoError(t, err)
	assert.Greater(t, tvAfter, tvBefore, "禁用应递增 token_version 使旧令牌立即失效")
}

func TestUpdateStaffChecked_RenameDoesNotBumpTokenVersion(t *testing.T) {
	u := mkUser(t, "rename-me", false)
	tvBefore, _ := staff.Service.CurrentTokenVersion(u.ID)
	newName := "renamed"
	_, err := staff.Service.UpdateStaffChecked(u.ID, &staff.StaffUpdate{Name: &newName}, 1, true)
	require.NoError(t, err)
	tvAfter, _ := staff.Service.CurrentTokenVersion(u.ID)
	assert.Equal(t, tvBefore, tvAfter, "普通字段更新不应递增 token_version")
}
