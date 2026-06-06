package user

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminListAndFilter(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	_, err := Service.Register(&RegisterRequest{Phone: "13800000001", Password: "pass123", Nickname: "甲"})
	require.NoError(t, err)
	_, err = Service.Register(&RegisterRequest{Phone: "13800000002", Password: "pass123", Nickname: "乙"})
	require.NoError(t, err)
	disabled, err := Service.Register(&RegisterRequest{Phone: "13900000003", Password: "pass123"})
	require.NoError(t, err)
	_, err = Service.SetActive(disabled.ID, false)
	require.NoError(t, err)

	// 全部
	all, total, err := Service.AdminList(1, 50, "", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, all, 3)

	// 手机号模糊
	_, total, err = Service.AdminList(1, 50, "138000", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// 按启用状态
	active := true
	_, total, err = Service.AdminList(1, 50, "", &active)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	inactive := false
	_, total, err = Service.AdminList(1, 50, "", &inactive)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestSetActiveDisablesAndForcesLogout(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	c, err := Service.Register(&RegisterRequest{Phone: "13700000001", Password: "pass123"})
	require.NoError(t, err)
	v0, _ := Service.CurrentTokenVersion(c.ID)

	// 禁用：is_active=false 且令牌版本递增（强制下线）
	got, err := Service.SetActive(c.ID, false)
	require.NoError(t, err)
	assert.False(t, got.IsActive)
	v1, _ := Service.CurrentTokenVersion(c.ID)
	assert.Equal(t, v0+1, v1, "禁用应递增令牌版本")

	// 被禁用账号无法登录
	_, err = Service.Authenticate("13700000001", "pass123")
	assert.Error(t, err)

	// 重新启用：不再递增版本
	got, err = Service.SetActive(c.ID, true)
	require.NoError(t, err)
	assert.True(t, got.IsActive)
	v2, _ := Service.CurrentTokenVersion(c.ID)
	assert.Equal(t, v1, v2, "启用不应改变令牌版本")

	// 启用后可登录
	_, err = Service.Authenticate("13700000001", "pass123")
	require.NoError(t, err)
}

func TestSetActiveNotFound(t *testing.T) {
	_, err := Service.SetActive(999999, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}
