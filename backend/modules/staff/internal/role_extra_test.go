package staff_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖角色的 HTTP 处理器：GetRole / UpdateRole / DeleteRole。
func TestRoleGetUpdateDeleteHTTP(t *testing.T) {
	admin := createTestUser(t, "rolehttpadmin", "p", true)
	t.Cleanup(func() { staff.Service.DeleteStaff(admin.ID) })
	token := getToken(t, admin.ID, admin.Username)
	r := setupRouter()

	role, err := staff.RoleService.Create(&staff.RoleCreate{
		Name: "hr-http", Description: "d", Permissions: []string{"example:view"},
	})
	require.NoError(t, err)

	// GET /role/:id
	w := do(r, "GET", fmt.Sprintf("/api/v1/role/%d", role.ID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &got)
	assert.Equal(t, "hr-http", got["data"].(map[string]interface{})["name"])

	// PUT /role/update/:id
	body, _ := json.Marshal(staff.RoleUpdate{
		Name: "hr-http2", Description: "d2", Permissions: []string{"example:view", "example:create"},
	})
	w = do(r, "PUT", fmt.Sprintf("/api/v1/role/update/%d", role.ID), token, body)
	require.Equal(t, http.StatusOK, w.Code)
	updated, _ := staff.RoleService.GetByID(role.ID)
	assert.Equal(t, "hr-http2", updated.Name)
	assert.Len(t, updated.Permissions, 2)

	// DELETE /role/delete/:id → 之后 GET 404
	w = do(r, "DELETE", fmt.Sprintf("/api/v1/role/delete/%d", role.ID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = do(r, "GET", fmt.Sprintf("/api/v1/role/%d", role.ID), token, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// GetRole 对不存在的 ID 返回 404。
func TestRoleGetHTTPNotFound(t *testing.T) {
	admin := createTestUser(t, "rolehttp404", "p", true)
	t.Cleanup(func() { staff.Service.DeleteStaff(admin.ID) })
	token := getToken(t, admin.ID, admin.Username)
	r := setupRouter()

	w := do(r, "GET", "/api/v1/role/999999", token, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAllRolesService(t *testing.T) {
	r1, err := staff.RoleService.Create(&staff.RoleCreate{Name: "all-roles-1"})
	require.NoError(t, err)
	r2, err := staff.RoleService.Create(&staff.RoleCreate{Name: "all-roles-2"})
	require.NoError(t, err)
	t.Cleanup(func() { staff.RoleService.Delete(r1.ID); staff.RoleService.Delete(r2.ID) })

	all, err := staff.RoleService.GetAllRoles()
	require.NoError(t, err)
	names := map[string]bool{}
	for _, r := range all {
		names[r.Name] = true
	}
	assert.True(t, names["all-roles-1"] && names["all-roles-2"], "GetAllRoles 应包含新建角色")
}

func TestGetStaffRolesForStaffService(t *testing.T) {
	role, err := staff.RoleService.Create(&staff.RoleCreate{Name: "batch-role", Permissions: []string{"example:view"}})
	require.NoError(t, err)
	u := createTestUser(t, "batchroleuser", "p", false)
	t.Cleanup(func() { staff.Service.DeleteStaff(u.ID); staff.RoleService.Delete(role.ID) })

	_, err = staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{RoleIDs: &[]int{role.ID}})
	require.NoError(t, err)

	byUser, err := staff.RoleService.GetStaffRolesForStaff([]int{u.ID})
	require.NoError(t, err)
	require.Len(t, byUser[u.ID], 1)
	assert.Equal(t, "batch-role", byUser[u.ID][0].Name)

	// 空入参返回空 map
	empty, err := staff.RoleService.GetStaffRolesForStaff(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestUpdateStaffService(t *testing.T) {
	u := createTestUser(t, "updatesvc", "p", false)
	t.Cleanup(func() { staff.Service.DeleteStaff(u.ID) })

	u.Name = "新名字"
	u.Avatar = "https://x/a.png"
	u.IsActive = false
	require.NoError(t, staff.Service.UpdateStaff(u))

	got, err := staff.Service.GetStaffByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, "新名字", got.Name)
	assert.Equal(t, "https://x/a.png", got.Avatar)
	assert.False(t, got.IsActive)
}
