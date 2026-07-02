package framework

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// F2 回归：连真实库(非 sqlite)时默认 JWT 密钥必须 fail-fast；sqlite 开发态放行。
func TestValidateProdSecrets(t *testing.T) {
	// sqlite（本地开发默认）：默认密钥仅告警放行，保留 go run . 开箱即用。
	require.NoError(t, validateProdSecrets(&Config{DBType: "sqlite", JWTSecret: defaultJWTSecret}))

	// 生产（mysql/postgres）+ 默认密钥 → fail-fast。
	require.Error(t, validateProdSecrets(&Config{DBType: "mysql", JWTSecret: defaultJWTSecret}))
	require.Error(t, validateProdSecrets(&Config{DBType: "postgres", JWTSecret: defaultJWTSecret}))

	// 生产 + 强随机密钥 → 放行。
	require.NoError(t, validateProdSecrets(&Config{DBType: "mysql", JWTSecret: "a-strong-random-secret-1234567890"}))
}
