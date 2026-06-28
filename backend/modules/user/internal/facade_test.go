package user

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IDByPhone：命中（精确手机号）/ 未命中（不存在手机号）/ 空串。
func TestServiceIDByPhone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	u, err := Service.Register(&RegisterRequest{Phone: "13966660001", Password: "pass123"})
	require.NoError(t, err)

	// 命中。
	id, ok, err := Service.IDByPhone("13966660001")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uint(u.ID), id)

	// 未命中（手机号不存在）→ (0,false,nil)，非错误。
	id, ok, err = Service.IDByPhone("13900000000")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, uint(0), id)

	// 空串 → (0,false,nil)，不查库。
	id, ok, err = Service.IDByPhone("")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, uint(0), id)
}

// PhonesByIDs：多 id / 含不存在 id（缺失不在 map 中）/ 空 ids（空 map）。
func TestServicePhonesByIDs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })

	u1, err := Service.Register(&RegisterRequest{Phone: "13977770001", Password: "pass123"})
	require.NoError(t, err)
	u2, err := Service.Register(&RegisterRequest{Phone: "13977770002", Password: "pass123"})
	require.NoError(t, err)

	// 多 id + 含一个不存在的 id（999999）。
	m, err := Service.PhonesByIDs([]uint{uint(u1.ID), uint(u2.ID), 999999})
	require.NoError(t, err)
	assert.Equal(t, "13977770001", m[uint(u1.ID)])
	assert.Equal(t, "13977770002", m[uint(u2.ID)])
	_, present := m[999999]
	assert.False(t, present, "不存在的 ID 不应出现在 map 中")
	assert.Len(t, m, 2)

	// 空 ids → 空 map（非 nil），不查库。
	empty, err := Service.PhonesByIDs(nil)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)
}
