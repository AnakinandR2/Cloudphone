package framework

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// F2 回归：连真实库(非 sqlite)的生产启动用默认 JWT 密钥必须 fail-fast；sqlite 与测试态放行。
func TestValidateProdSecrets(t *testing.T) {
	// sqlite（本地开发默认）：默认密钥放行，保留 go run . 开箱即用。
	require.NoError(t, validateProdSecrets(&Config{DBType: "sqlite", GinMode: "release", JWTSecret: defaultJWTSecret}))

	// 测试态(GIN_MODE=test)即便连 mysql/postgres 也放行——不误杀 DB_TYPE=mysql go test。
	require.NoError(t, validateProdSecrets(&Config{DBType: "mysql", GinMode: "test", JWTSecret: defaultJWTSecret}))

	// 生产(release + mysql/postgres) + 默认密钥 → fail-fast。
	require.Error(t, validateProdSecrets(&Config{DBType: "mysql", GinMode: "release", JWTSecret: defaultJWTSecret}))
	require.Error(t, validateProdSecrets(&Config{DBType: "postgres", GinMode: "release", JWTSecret: defaultJWTSecret}))

	// 生产 + 强随机密钥 → 放行。
	require.NoError(t, validateProdSecrets(&Config{DBType: "mysql", GinMode: "release", JWTSecret: "a-strong-random-secret-1234567890"}))
}
