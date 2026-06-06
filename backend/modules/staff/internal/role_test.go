package staff_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- RoleService Tests ----

func TestRoleCreateAndGet(t *testing.T) {
	role, err := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "test-role",
		Description: "test desc",
		Permissions: []string{"staff:view", "example:view"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test-role", role.Name)
	assert.Equal(t, 2, len(role.Permissions))

	got, err := staff.RoleService.GetByID(role.ID)
	require.NoError(t, err)
	assert.Equal(t, role.ID, got.ID)
	assert.Contains(t, got.Permissions, "staff:view")
	assert.Contains(t, got.Permissions, "example:view")
}

func TestRoleCreateDuplicateName(t *testing.T) {
	_, err := staff.RoleService.Create(&staff.RoleCreate{Name: "dup-role"})
	require.NoError(t, err)
	_, err = staff.RoleService.Create(&staff.RoleCreate{Name: "dup-role"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色名已存在")
}

func TestRoleCreateInvalidPermission(t *testing.T) {
	_, err := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "bad-perm-role",
		Permissions: []string{"nonexist:perm"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的权限标识")
}

func TestRoleUpdate(t *testing.T) {
	role, err := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "update-role",
		Permissions: []string{"staff:view"},
	})
	require.NoError(t, err)

	updated, err := staff.RoleService.Update(role.ID, &staff.RoleUpdate{
		Name:        "update-role-v2",
		Description: "updated",
		Permissions: []string{"staff:view", "staff:create", "staff:edit"},
	})
	require.NoError(t, err)
	assert.Equal(t, "update-role-v2", updated.Name)
	assert.Equal(t, 3, len(updated.Permissions))
}

func TestRoleDelete(t *testing.T) {
	role, err := staff.RoleService.Create(&staff.RoleCreate{Name: "del-role"})
	require.NoError(t, err)

	err = staff.RoleService.Delete(role.ID)
	require.NoError(t, err)

	_, err = staff.RoleService.GetByID(role.ID)
	assert.Error(t, err)
}

func TestRoleList(t *testing.T) {
	for i := 0; i < 3; i++ {
		_, err := staff.RoleService.Create(&staff.RoleCreate{Name: fmt.Sprintf("list-role-%d", i)})
		require.NoError(t, err)
	}

	list, total, err := staff.RoleService.GetList(1, 10, "list-role", "", "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(total), 3)
	assert.GreaterOrEqual(t, len(list), 3)
}

// ---- Staff-Role Assignment Tests ----

func TestUserRoleAssignment(t *testing.T) {
	u := createTestUser(t, "role-assign-user", "pass123", false)
	role1, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "assign-role-1",
		Permissions: []string{"staff:view", "example:view"},
	})
	role2, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "assign-role-2",
		Permissions: []string{"example:create", "example:edit"},
	})

	_, err := staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
		Username: u.Username,
		RoleIDs:  &[]int{role1.ID, role2.ID},
	})
	require.NoError(t, err)

	roles, err := staff.RoleService.GetStaffRoles(u.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, len(roles))

	perms, err := staff.RoleService.GetStaffPermissions(u.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(perms), 4)
	assert.Contains(t, perms, "staff:view")
	assert.Contains(t, perms, "example:create")
}

func TestHasPermission(t *testing.T) {
	u := createTestUser(t, "hasperm-user", "pass123", false)
	role, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "hasperm-role",
		Permissions: []string{"staff:view", "staff:edit"},
	})
	_, err := staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
		Username: u.Username,
		RoleIDs:  &[]int{role.ID},
	})
	require.NoError(t, err)

	has, err := staff.RoleService.HasPermission(u.ID, "staff:view")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = staff.RoleService.HasPermission(u.ID, "staff:delete")
	require.NoError(t, err)
	assert.False(t, has)

	has, err = staff.RoleService.HasPermission(u.ID, "staff:delete", "staff:view")
	require.NoError(t, err)
	assert.True(t, has, "any-match should return true")
}

func TestGetStaffProfile(t *testing.T) {
	u := createTestUser(t, "profile-user", "pass123", false)
	role, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "profile-role",
		Permissions: []string{"dashboard:view", "example:view"},
	})
	_, _ = staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
		Username: u.Username,
		RoleIDs:  &[]int{role.ID},
	})

	profile, err := staff.Service.GetStaffProfile(u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Username, profile.Username)
	assert.Contains(t, profile.Permissions, "dashboard:view")
	assert.Equal(t, 1, len(profile.Roles))
	assert.Equal(t, "profile-role", profile.Roles[0].Name)
}

func TestGetStaffProfileSuperuser(t *testing.T) {
	u := createTestUser(t, "super-profile", "pass123", true)
	profile, err := staff.Service.GetStaffProfile(u.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"*"}, profile.Permissions)
}

// ---- API Tests ----

func TestRoleAPICreateAndList(t *testing.T) {
	router := setupRouter()
	admin := createTestUser(t, "role-api-admin", "pass123", true)
	token := getToken(t, admin.ID, admin.Username)

	body, _ := json.Marshal(staff.RoleCreate{
		Name:        "api-role",
		Description: "api test",
		Permissions: []string{"staff:view", "staff:create"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/role/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/role/list?page=1&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestRoleAPIPermissionCheck(t *testing.T) {
	router := setupRouter()
	normalUser := createTestUser(t, "role-api-normal", "pass123", false)
	token := getToken(t, normalUser.ID, normalUser.Username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/role/list?page=1&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code, "normal user without role:view should be forbidden")
}

func TestRoleAPIPermissionGranted(t *testing.T) {
	router := setupRouter()
	normalUser := createTestUser(t, "role-api-granted", "pass123", false)
	role, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "api-viewer-role",
		Permissions: []string{"role:view"},
	})
	_, _ = staff.Service.UpdateStaffWithForm(normalUser.ID, &staff.StaffUpdate{
		Username: normalUser.Username,
		RoleIDs:  &[]int{role.ID},
	})
	token := getToken(t, normalUser.ID, normalUser.Username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/role/list?page=1&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code, "user with role:view should access role list")
}

func TestGetPermissionsAPI(t *testing.T) {
	router := setupRouter()
	u := createTestUser(t, "perm-api-user", "pass123", false)
	token := getToken(t, u.ID, u.Username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/role/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 5)
}

func TestAuthMeWithPermissions(t *testing.T) {
	router := setupRouter()
	u := createTestUser(t, "me-perm-user", "pass123", false)
	role, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "me-test-role",
		Permissions: []string{"dashboard:view"},
	})
	_, _ = staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
		Username: u.Username,
		RoleIDs:  &[]int{role.ID},
	})
	token := getToken(t, u.ID, u.Username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/staff/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	perms := data["permissions"].([]interface{})
	assert.Contains(t, perms, "dashboard:view")
	roles := data["roles"].([]interface{})
	assert.Equal(t, 1, len(roles))
}

func TestDeleteRoleCascade(t *testing.T) {
	u := createTestUser(t, "cascade-user", "pass123", false)
	role, _ := staff.RoleService.Create(&staff.RoleCreate{
		Name:        "cascade-role",
		Permissions: []string{"staff:view"},
	})
	_, _ = staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
		Username: u.Username,
		RoleIDs:  &[]int{role.ID},
	})

	roles, _ := staff.RoleService.GetStaffRoles(u.ID)
	assert.Equal(t, 1, len(roles))

	err := staff.RoleService.Delete(role.ID)
	require.NoError(t, err)

	roles, _ = staff.RoleService.GetStaffRoles(u.ID)
	assert.Equal(t, 0, len(roles))

	perms, _ := staff.RoleService.GetStaffPermissions(u.ID)
	assert.Equal(t, 0, len(perms))
}

// ---- Readonly Demo Staff & Role Tests ----

func TestReadonlyRoleSeeded(t *testing.T) {
	list, total, err := staff.RoleService.GetList(1, 100, "示例只读", "", "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(total), 1, "示例只读角色应已存在")

	var found bool
	for _, r := range list {
		if r.Name == "示例只读" {
			found = true
			assert.True(t, r.IsBuiltin, "示例只读角色应为内置角色")
			assert.GreaterOrEqual(t, r.PermissionCount, 1)
			break
		}
	}
	assert.True(t, found, "角色列表中应包含示例只读")
}

func TestReadonlyUserSeeded(t *testing.T) {
	u := ensureReadonlyUser(t)
	assert.True(t, u.IsActive, "readonly 用户应为启用状态")
	assert.False(t, u.IsSuperuser, "readonly 用户不应是超管")
}

func TestReadonlyUserHasOnlyExampleView(t *testing.T) {
	u := ensureReadonlyUser(t)

	perms, err := staff.RoleService.GetStaffPermissions(u.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"example:view"}, perms, "readonly 用户应仅有 example:view 权限")
}

func TestReadonlyUserCannotCreateExample(t *testing.T) {
	u := ensureReadonlyUser(t)

	has, err := staff.RoleService.HasPermission(u.ID, "example:create")
	require.NoError(t, err)
	assert.False(t, has, "readonly 用户不应有 example:create 权限")
}

func TestReadonlyUserCannotAccessUserModule(t *testing.T) {
	u := ensureReadonlyUser(t)

	has, err := staff.RoleService.HasPermission(u.ID, "staff:view")
	require.NoError(t, err)
	assert.False(t, has, "readonly 用户不应有 user:view 权限")
}

func TestReadonlyStaffProfile(t *testing.T) {
	u := ensureReadonlyUser(t)

	profile, err := staff.Service.GetStaffProfile(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "readonly", profile.Username)
	assert.False(t, profile.IsSuperuser)
	assert.Equal(t, []string{"example:view"}, profile.Permissions)
	assert.Equal(t, 1, len(profile.Roles))
	assert.Equal(t, "示例只读", profile.Roles[0].Name)
}

func TestReadonlyUserNoExampleCreatePermission(t *testing.T) {
	u := ensureReadonlyUser(t)

	has, err := staff.RoleService.HasPermission(u.ID, "example:create")
	require.NoError(t, err)
	assert.False(t, has, "readonly 用户不应有 example:create 权限")
}

func TestReadonlyUserHasExampleViewPermission(t *testing.T) {
	u := ensureReadonlyUser(t)

	has, err := staff.RoleService.HasPermission(u.ID, "example:view")
	require.NoError(t, err)
	assert.True(t, has, "readonly 用户应有 example:view 权限")
}

func TestReadonlyUserAPIUserListForbidden(t *testing.T) {
	router := setupRouter()
	u := ensureReadonlyUser(t)
	token := getToken(t, u.ID, u.Username)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/staff/list?page=1&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, 403, w.Code, "readonly 用户访问用户列表应返回 403")
}

func TestReadonlyBuiltinRoleCannotBeDeleted(t *testing.T) {
	list, _, _ := staff.RoleService.GetList(1, 100, "示例只读", "", "")
	var roleID int
	for _, r := range list {
		if r.Name == "示例只读" {
			roleID = r.ID
			break
		}
	}
	require.NotZero(t, roleID)

	err := staff.RoleService.Delete(roleID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "内置角色不可删除")
}
