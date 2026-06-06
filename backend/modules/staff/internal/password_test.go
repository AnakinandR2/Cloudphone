package staff_test

import (
	"testing"

	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := staff.HashPassword("mypassword")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mypassword", hash)

	assert.True(t, staff.CheckPassword("mypassword", hash))
	assert.False(t, staff.CheckPassword("wrong", hash))
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	h1, _ := staff.HashPassword("same")
	h2, _ := staff.HashPassword("same")
	assert.NotEqual(t, h1, h2, "bcrypt 每次应生成不同的 salt")
}

func TestCheckPasswordEmpty(t *testing.T) {
	hash, err := staff.HashPassword("")
	require.NoError(t, err)
	assert.True(t, staff.CheckPassword("", hash))
	assert.False(t, staff.CheckPassword("notempty", hash))
}
