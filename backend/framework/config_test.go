package framework_test

import (
	"os"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
)

func saveConfig(t *testing.T) {
	t.Helper()
	orig := framework.AppConfig
	t.Cleanup(func() { framework.AppConfig = orig })
}

func TestLoadConfigDefaults(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	cfg := framework.LoadConfig()

	assert.Equal(t, "sqlite", cfg.DBType)
	assert.Equal(t, "127.0.0.1", cfg.DBHost)
	assert.Equal(t, 3306, cfg.DBPort)
	assert.Equal(t, "root", cfg.DBUser)
	assert.Equal(t, "", cfg.DBPassword)
	assert.Equal(t, "manage_system.db", cfg.DBName)
	assert.Equal(t, "9981", cfg.ServerPort)
	assert.Equal(t, "release", cfg.GinMode)
	assert.Equal(t, 15*24, cfg.JWTExpireHours)
	assert.Equal(t, "jwt_token", cfg.JWTCookieName)
	assert.False(t, cfg.JWTCookieSecure)
}

func TestLoadConfigFromEnv(t *testing.T) {
	saveConfig(t)
	os.Setenv("DB_HOST", "10.0.0.1")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_NAME", "testdb")
	os.Setenv("SERVER_PORT", "8080")
	os.Setenv("SSO_PROJECT_ID", "abc123")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	assert.Equal(t, "10.0.0.1", cfg.DBHost)
	assert.Equal(t, 5432, cfg.DBPort)
	assert.Equal(t, "testdb", cfg.DBName)
	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "abc123", cfg.SSOProjectID)
}

func TestLoadConfigSetsGlobal(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	cfg := framework.LoadConfig()
	assert.Equal(t, cfg, framework.AppConfig)
}

// STATEFUL / ENABLE_MIGRATIONS：默认值、环境变量覆盖、nil 语义与 ShouldRunSetup 真值表。
func TestStatefulAndMigrationsDefaults(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	cfg := framework.LoadConfig()
	// 默认都为 true：单实例 / 本地开发开箱即用。
	assert.True(t, cfg.Stateful)
	assert.True(t, cfg.EnableMigrations)
	assert.True(t, framework.IsStateful())
	assert.True(t, framework.MigrationsEnabled())
	assert.True(t, framework.ShouldRunSetup())
}

func TestStatefulAndMigrationsFromEnv(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	os.Setenv("STATEFUL", "false")
	os.Setenv("ENABLE_MIGRATIONS", "false")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	assert.False(t, cfg.Stateful)
	assert.False(t, cfg.EnableMigrations)
	assert.False(t, framework.IsStateful())
	assert.False(t, framework.MigrationsEnabled())
}

// AppConfig 为 nil 时（如仅测纯函数的单测）保留「默认承担」语义。
func TestStatefulHelpersNilConfig(t *testing.T) {
	saveConfig(t)
	framework.AppConfig = nil
	assert.True(t, framework.IsStateful())
	assert.True(t, framework.MigrationsEnabled())
	assert.True(t, framework.ShouldRunSetup())
}

// ShouldRunSetup = Stateful && EnableMigrations 的 4 组合真值表。
func TestShouldRunSetupTruthTable(t *testing.T) {
	saveConfig(t)
	cases := []struct {
		stateful, migrations, want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, c := range cases {
		framework.AppConfig = &framework.Config{Stateful: c.stateful, EnableMigrations: c.migrations}
		assert.Equal(t, c.want, framework.ShouldRunSetup(),
			"Stateful=%v EnableMigrations=%v", c.stateful, c.migrations)
	}
}

// getEnvAsDuration 经 LoadConfig 间接覆盖（函数本身未导出）：
// 合法值解析、空值/非法值/<=0 三种情形回退默认。
func TestLoadConfigPresignTTL_Defaults(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	cfg := framework.LoadConfig()
	// 未设置 → 默认 GET 15m / PUT 30m。
	assert.Equal(t, 15*time.Minute, cfg.S3PresignGetTTL)
	assert.Equal(t, 30*time.Minute, cfg.S3PresignPutTTL)
	// S3LibraryBucket 默认空。
	assert.Equal(t, "", cfg.S3LibraryBucket)
}

func TestLoadConfigPresignTTL_ValidParsed(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	os.Setenv("S3_PRESIGN_GET_TTL", "5m")
	os.Setenv("S3_PRESIGN_PUT_TTL", "1h30m")
	os.Setenv("S3_LIBRARY_BUCKET", "my-private-library")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	assert.Equal(t, 5*time.Minute, cfg.S3PresignGetTTL)
	assert.Equal(t, 90*time.Minute, cfg.S3PresignPutTTL)
	assert.Equal(t, "my-private-library", cfg.S3LibraryBucket)
}

func TestLoadConfigPresignTTL_InvalidFallsBack(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	os.Setenv("S3_PRESIGN_GET_TTL", "not-a-duration")
	os.Setenv("S3_PRESIGN_PUT_TTL", "")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	// 非法值回退默认 15m；空值回退默认 30m。
	assert.Equal(t, 15*time.Minute, cfg.S3PresignGetTTL)
	assert.Equal(t, 30*time.Minute, cfg.S3PresignPutTTL)
}

func TestLoadConfigPresignTTL_NonPositiveFallsBack(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	os.Setenv("S3_PRESIGN_GET_TTL", "0s")
	os.Setenv("S3_PRESIGN_PUT_TTL", "-10m")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	// <=0 回退默认。
	assert.Equal(t, 15*time.Minute, cfg.S3PresignGetTTL)
	assert.Equal(t, 30*time.Minute, cfg.S3PresignPutTTL)
}

// S3_LIBRARY_* 是一套完全独立的私有 S3 配置：各字段独立解析，不与公开 S3_* 交叉回退。
func TestLoadConfigLibraryS3Independent(t *testing.T) {
	saveConfig(t)
	os.Clearenv()
	// 公开 S3 一套；素材库私有 S3 另一套完全不同的值。
	os.Setenv("S3_ENDPOINT", "https://public.example.com")
	os.Setenv("S3_ACCESS_KEY_ID", "pub-ak")
	os.Setenv("S3_BUCKET", "public-bucket")
	os.Setenv("S3_LIBRARY_ENDPOINT", "https://private.example.com")
	os.Setenv("S3_LIBRARY_REGION", "cn-north-1")
	os.Setenv("S3_LIBRARY_ACCESS_KEY_ID", "lib-ak")
	os.Setenv("S3_LIBRARY_SECRET_ACCESS_KEY", "lib-sk")
	os.Setenv("S3_LIBRARY_BUCKET", "private-library")
	os.Setenv("S3_LIBRARY_USE_PATH_STYLE", "true")
	defer os.Clearenv()

	cfg := framework.LoadConfig()
	// 私有 S3 取自 S3_LIBRARY_*，与公开 S3_* 完全独立。
	assert.Equal(t, "https://private.example.com", cfg.S3LibraryEndpoint)
	assert.Equal(t, "cn-north-1", cfg.S3LibraryRegion)
	assert.Equal(t, "lib-ak", cfg.S3LibraryAccessKeyID)
	assert.Equal(t, "lib-sk", cfg.S3LibrarySecretAccessKey)
	assert.Equal(t, "private-library", cfg.S3LibraryBucket)
	assert.True(t, cfg.S3LibraryUsePathStyle)
	// 不交叉：公开端点/桶不污染私有字段。
	assert.NotEqual(t, cfg.S3Endpoint, cfg.S3LibraryEndpoint)
	assert.NotEqual(t, cfg.S3Bucket, cfg.S3LibraryBucket)
}
