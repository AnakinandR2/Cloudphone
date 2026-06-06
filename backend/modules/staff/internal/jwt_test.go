package staff_test

import (
	"testing"
	"time"

	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndParseToken(t *testing.T) {
	token, err := staff.GenerateToken(42, "alice", "test-secret", 24)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := staff.ParseToken(token, "test-secret")
	require.NoError(t, err)
	assert.Equal(t, 42, claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.True(t, claims.ExpiresAt.Time.After(time.Now()))
}

func TestParseTokenWrongSecret(t *testing.T) {
	token, _ := staff.GenerateToken(1, "bob", "secret-a", 1)
	_, err := staff.ParseToken(token, "secret-b")
	assert.Error(t, err)
}

func TestParseTokenInvalid(t *testing.T) {
	_, err := staff.ParseToken("not-a-jwt", "secret")
	assert.Error(t, err)
}

func TestTokenExpiry(t *testing.T) {
	token, err := staff.GenerateToken(1, "user", "s", 1)
	require.NoError(t, err)

	claims, err := staff.ParseToken(token, "s")
	require.NoError(t, err)
	assert.True(t, claims.ExpiresAt.Time.Before(time.Now().Add(2*time.Hour)))
}
