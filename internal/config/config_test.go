package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"APP_ENV", "APP_PORT", "OPENAI_KEY", "DB_HOST"} {
		t.Setenv(k, "")
	}
	cfg := Load(filepath.Join(t.TempDir(), "missing.env"))
	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "8081", cfg.AppPort)
	assert.Equal(t, "", cfg.OpenAIKey)
	assert.Equal(t, ":8081", cfg.Address())
	assert.False(t, cfg.IsProduction())
	assert.Contains(t, cfg.DSN(), "host=127.0.0.1")
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".env")
	assert.NoError(t, os.WriteFile(file, []byte("APP_ENV=production\nAPP_PORT=9999\nDB_DATABASE=blogx\n"), 0o600))
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_PORT", "")
	t.Setenv("DB_DATABASE", "")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("APP_PORT")
	os.Unsetenv("DB_DATABASE")

	cfg := Load(file)
	assert.Equal(t, "production", cfg.AppEnv)
	assert.True(t, cfg.IsProduction())
	assert.Equal(t, ":9999", cfg.Address())
	assert.Contains(t, cfg.DSN(), "dbname=blogx")
}

func TestLoadDefaultFileName(t *testing.T) {
	wd, _ := os.Getwd()
	dir := t.TempDir()
	assert.NoError(t, os.Chdir(dir))
	defer os.Chdir(wd)
	t.Setenv("APP_PORT", "")
	cfg := Load()
	assert.Equal(t, "8081", cfg.AppPort)
}

func TestGet(t *testing.T) {
	t.Setenv("X_TEST_KEY", "value")
	assert.Equal(t, "value", Get("X_TEST_KEY", "def"))
	assert.Equal(t, "def", Get("X_TEST_KEY_NOT_SET", "def"))
}

func TestDatabaseURLAndPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://u:p@ep-x.neon.tech/db?sslmode=require")
	t.Setenv("APP_PORT", "")
	t.Setenv("PORT", "9000")
	cfg := Load(filepath.Join(t.TempDir(), "missing.env"))
	assert.Equal(t, "postgresql://u:p@ep-x.neon.tech/db?sslmode=require", cfg.DSN())
	assert.Equal(t, ":9000", cfg.Address())
}
