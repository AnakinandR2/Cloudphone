package framework_test

import (
	"os"
	"testing"

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
