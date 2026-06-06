package staff_test

import (
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/staff/internal"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	framework.SetupRouter(r)
	return r
}

func createTestUser(t *testing.T, username, password string, superuser bool) *staff.Staff {
	t.Helper()
	u, err := staff.Service.CreateStaffWithForm(&staff.StaffCreate{
		Username:    username,
		Password:    password,
		IsActive:    true,
		IsSuperuser: superuser,
	})
	require.NoError(t, err)
	return u
}

func getToken(t *testing.T, userID int, username string) string {
	t.Helper()
	token, err := staff.GenerateToken(userID, username, framework.AppConfig.JWTSecret, framework.AppConfig.JWTExpireHours)
	require.NoError(t, err)
	return token
}

// ensureReadonlyUser 确保 readonly 用户和"示例只读"角色关联存在（其他测试的 Cleanup 会清空 users 表）
func ensureReadonlyUser(t *testing.T) *staff.Staff {
	t.Helper()
	u, err := staff.Service.GetStaffByUsername("readonly")
	if err == nil {
		return u
	}
	u, err = staff.Service.CreateStaffWithForm(&staff.StaffCreate{
		Username: "readonly",
		Password: "readonly123",
		IsActive: true,
	})
	require.NoError(t, err)

	list, _, _ := staff.RoleService.GetList(1, 100, "示例只读", "", "")
	for _, r := range list {
		if r.Name == "示例只读" {
			staff.Service.UpdateStaffWithForm(u.ID, &staff.StaffUpdate{
				Username: u.Username,
				RoleIDs:  &[]int{r.ID},
			})
			break
		}
	}
	u, _ = staff.Service.GetStaffByUsername("readonly")
	return u
}
