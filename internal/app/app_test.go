package app

import (
	"errors"
	"io"
	"testing"

	"app/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

func sqliteConfig() *config.Config {
	return &config.Config{
		AppEnv: "testing", AppPort: "0", AppKey: "k",
		AdminUsername: "admin", AdminPassword: "admin",
		DBConnection: "sqlite", DBDatabase: "file::memory:",
	}
}

func restore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	lc, c, m, s, st, f := LoadConfig, Connect, Migrate, Seed, StartServer, Fatal
	t.Cleanup(func() { LoadConfig, Connect, Migrate, Seed, StartServer, Fatal = lc, c, m, s, st, f })
}

func TestRunSuccess(t *testing.T) {
	restore(t)
	var addr string
	StartServer = func(r *gin.Engine, a string) error { addr = a; return nil }
	assert.NoError(t, Run(sqliteConfig()))
	assert.Equal(t, ":0", addr)
}

func TestRunConnectError(t *testing.T) {
	restore(t)
	cfg := sqliteConfig()
	cfg.DBDatabase = "/nonexistent-dir/x/y.db"
	assert.Error(t, Run(cfg))
}

func TestRunSetupError(t *testing.T) {
	restore(t)
	Seed = func(*gorm.DB, *config.Config) error { return errBoom }
	assert.ErrorIs(t, Run(sqliteConfig()), errBoom)
}

func TestSetupMigrateError(t *testing.T) {
	restore(t)
	Migrate = func(*gorm.DB) error { return errBoom }
	assert.ErrorIs(t, Setup(nil, sqliteConfig()), errBoom)
}

func TestMain_(t *testing.T) {
	restore(t)
	LoadConfig = sqliteConfig
	StartServer = func(*gin.Engine, string) error { return errBoom }
	var got []any
	Fatal = func(v ...any) { got = v }
	Main()
	assert.Equal(t, []any{errBoom}, got)

	got = nil
	StartServer = func(*gin.Engine, string) error { return nil }
	Main()
	assert.Nil(t, got)
}

func TestDefaultLoadConfig(t *testing.T) {
	assert.NotNil(t, LoadConfig())
}

func TestDefaultStartServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// alamat tidak valid -> r.Run langsung mengembalikan error (tanpa memblokir)
	assert.Error(t, StartServer(gin.New(), "invalid-address:-1"))
}
