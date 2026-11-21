package main

import (
	"io"
	"testing"

	"app/internal/app"
	"app/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMain_(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	lc, st := app.LoadConfig, app.StartServer
	defer func() { app.LoadConfig, app.StartServer = lc, st }()

	app.LoadConfig = func() *config.Config {
		return &config.Config{AppPort: "0", AppKey: "k", AdminUsername: "admin", AdminPassword: "admin",
			DBConnection: "sqlite", DBDatabase: "file::memory:"}
	}
	started := false
	app.StartServer = func(*gin.Engine, string) error { started = true; return nil }
	main()
	assert.True(t, started)
}
