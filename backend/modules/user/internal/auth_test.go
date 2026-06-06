package user

import (
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserJWTSecretFallbackAndIsolation(t *testing.T) {
	orig := framework.AppConfig.UserJWTSecret
	t.Cleanup(func() { framework.AppConfig.UserJWTSecret = orig })

	// 未配置独立密钥 → 回退到后台 JWTSecret
	framework.AppConfig.UserJWTSecret = ""
	assert.Equal(t, framework.AppConfig.JWTSecret, jwtSecret())

	// 配置独立密钥 → 使用独立密钥；用后台密钥无法验签（纵深防御）
	framework.AppConfig.UserJWTSecret = "user-only-secret"
	assert.Equal(t, "user-only-secret", jwtSecret())

	tok, err := auth.Generate(1, "13800138000", auth.ScopeUser, 0, jwtSecret(), 1)
	require.NoError(t, err)

	_, err = auth.Parse(tok, framework.AppConfig.JWTSecret)
	assert.Error(t, err, "前台令牌不应能用后台密钥验签")

	claims, err := auth.Parse(tok, jwtSecret())
	require.NoError(t, err)
	assert.Equal(t, auth.ScopeUser, claims.Scope)
}
