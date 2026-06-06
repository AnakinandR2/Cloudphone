package staff_test

import (
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsFirstExternalStaffWithBuiltinOnly(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "roles", "role_permissions", "staff") })

	for _, u := range []struct {
		name string
		sup  bool
	}{
		{"admin", true},
		{"test", false},
		{"readonly", false},
	} {
		staff.Service.CreateStaff(&staff.Staff{
			Username:    u.name,
			IsActive:    true,
			IsSuperuser: u.sup,
		})
	}

	assert.True(t, staff.IsFirstExternalStaff(), "只有内置用户时应返回 true")
}

func TestIsFirstExternalStaffWithExternalUser(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "roles", "role_permissions", "staff") })

	staff.Service.CreateStaff(&staff.Staff{Username: "admin", IsActive: true, IsSuperuser: true})
	staff.Service.CreateStaff(&staff.Staff{Username: "test", IsActive: true})
	staff.Service.CreateStaff(&staff.Staff{Username: "readonly", IsActive: true})
	staff.Service.CreateStaff(&staff.Staff{Username: "external_sso_user", IsActive: true})

	assert.False(t, staff.IsFirstExternalStaff(), "有外部用户后应返回 false")
}

func TestCreateStaffWithForm(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u, err := staff.Service.CreateStaffWithForm(&staff.StaffCreate{
		Username: "testuser",
		Password: "pass123",
		IsActive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "testuser", u.Username)
	assert.True(t, u.IsActive)
	assert.False(t, u.IsSuperuser)
	assert.NotZero(t, u.ID)
}

func TestCreateStaffDuplicate(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	_, err := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "dup", Password: "p"})
	require.NoError(t, err)

	_, err = staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "dup", Password: "p"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestGetStaffByID(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	created, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "byid", Password: "p", IsActive: true})

	got, err := staff.Service.GetStaffByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "byid", got.Username)
}

func TestGetStaffByIDNotFound(t *testing.T) {
	_, err := staff.Service.GetStaffByID(999999)
	assert.Error(t, err)
}

func TestGetStaffByUsername(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "findme", Password: "p", IsActive: true})

	u, err := staff.Service.GetStaffByUsername("findme")
	require.NoError(t, err)
	assert.Equal(t, "findme", u.Username)
}

func TestVerifyPassword(t *testing.T) {
	hashed, err := staff.Service.HashPassword("secret")
	require.NoError(t, err)

	assert.True(t, staff.Service.VerifyPassword(hashed, "secret"))
	assert.False(t, staff.Service.VerifyPassword(hashed, "wrong"))
}

func TestUpdatePassword(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "pwduser", Password: "old123", IsActive: true})

	err := staff.Service.UpdatePassword(u.ID, "old123", "new456")
	require.NoError(t, err)

	updated, _ := staff.Service.GetStaffByID(u.ID)
	assert.True(t, staff.Service.VerifyPassword(updated.HashedPassword, "new456"))
	assert.False(t, staff.Service.VerifyPassword(updated.HashedPassword, "old123"))
}

func TestUpdatePasswordWrongOld(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "pwdwrong", Password: "correct", IsActive: true})

	err := staff.Service.UpdatePassword(u.ID, "wrong", "new456")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "旧密码不正确")
}

func TestUpdateStaffWithForm(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "original", Password: "p", IsActive: true})

	boolTrue := true
	updated, err := staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{Username: "renamed", IsActive: &boolTrue, IsSuperuser: &boolTrue})
	require.NoError(t, err)
	assert.Equal(t, "renamed", updated.Username)
	assert.True(t, updated.IsSuperuser)
}

func TestUpdateStaffDuplicateUsername(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "a", Password: "p", IsActive: true})
	b, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "b", Password: "p", IsActive: true})

	_, err := staff.Service.UpdateStaffWithForm(b.ID, &staff.StaffUpdate{Username: "a"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已被使用")
}

func TestDeleteStaff(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	u, _ := staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "del", Password: "p", IsActive: true})

	err := staff.Service.DeleteStaff(u.ID)
	require.NoError(t, err)

	_, err = staff.Service.GetStaffByID(u.ID)
	assert.Error(t, err)
}

func TestGetStaffList(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	for i := 0; i < 15; i++ {
		staff.Service.CreateStaffWithForm(&staff.StaffCreate{
			Username: "listuser" + string(rune('a'+i)),
			Password: "p",
			IsActive: true,
		})
	}

	list, total, err := staff.Service.GetStaffList(1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, list, 10)

	list2, _, err := staff.Service.GetStaffList(2, 10, "", "", "")
	require.NoError(t, err)
	assert.Len(t, list2, 5)
}

func TestGetStaffListFilter(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })

	staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "alice", Password: "p", IsActive: true})
	staff.Service.CreateStaffWithForm(&staff.StaffCreate{Username: "bob", Password: "p", IsActive: true})

	list, total, err := staff.Service.GetStaffList(1, 10, "ali", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
	assert.Equal(t, "alice", list[0].Username)
}
