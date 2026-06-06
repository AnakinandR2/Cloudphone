package user

import (
	"fmt"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 注册的 ID 不连续：每次在自增基础上额外加随机 0..4，相邻间隔恒在 [1,5]。
func TestRegisterIDRandomGap(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	prev := 0
	for i := 0; i < 15; i++ {
		c, err := Service.Register(&RegisterRequest{Phone: fmt.Sprintf("138%08d", i), Password: "pass123"})
		require.NoError(t, err)
		gap := c.ID - prev
		assert.GreaterOrEqual(t, gap, 1, "ID 必须严格递增")
		assert.LessOrEqual(t, gap, 5, "相邻 ID 间隔应不超过 5")
		prev = c.ID
	}
}

func TestUserRegister(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	c, err := Service.Register(&RegisterRequest{Phone: "13800138000", Password: "pass123", Nickname: "小明"})
	require.NoError(t, err)
	assert.NotZero(t, c.ID)
	assert.Equal(t, "13800138000", c.Phone)
	assert.Equal(t, "小明", c.Nickname)
	assert.True(t, c.IsActive)
}

func TestUserRegisterInvalidPhone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	for _, bad := range []string{"", "123", "12345678901", "23800138000", "1380013800a"} {
		_, err := Service.Register(&RegisterRequest{Phone: bad, Password: "pass123"})
		assert.Error(t, err, "手机号 %q 应被拒绝", bad)
		assert.Contains(t, err.Error(), "手机号")
	}
}

func TestUserRegisterShortPassword(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	_, err := Service.Register(&RegisterRequest{Phone: "13812345678", Password: "12345"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "密码")
}

func TestUserRegisterDuplicatePhone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	_, err := Service.Register(&RegisterRequest{Phone: "13900139000", Password: "pass123"})
	require.NoError(t, err)

	_, err = Service.Register(&RegisterRequest{Phone: "13900139000", Password: "pass1234"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已注册")
}

func TestUserAuthenticate(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	_, err := Service.Register(&RegisterRequest{Phone: "13700137000", Password: "secret"})
	require.NoError(t, err)

	c, err := Service.Authenticate("13700137000", "secret")
	require.NoError(t, err)
	assert.Equal(t, "13700137000", c.Phone)

	_, err = Service.Authenticate("13700137000", "wrong")
	assert.Error(t, err)

	_, err = Service.Authenticate("00000000000", "secret")
	assert.Error(t, err)
}

func TestUserLogoutBumpsTokenVersion(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	created, err := Service.Register(&RegisterRequest{Phone: "13500135000", Password: "pass123"})
	require.NoError(t, err)
	assert.Equal(t, 0, created.TokenVersion)

	v, err := Service.CurrentTokenVersion(created.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, v)

	require.NoError(t, Service.Logout(created.ID))

	v, err = Service.CurrentTokenVersion(created.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, v, "登出应递增令牌版本")
}

func TestUserGetByID(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	created, err := Service.Register(&RegisterRequest{Phone: "13600136000", Password: "pass123"})
	require.NoError(t, err)

	got, err := Service.GetByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = Service.GetByID(999999)
	assert.Error(t, err)
}
