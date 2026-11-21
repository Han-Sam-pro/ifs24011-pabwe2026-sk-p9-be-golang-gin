// Package app merakit dan menjalankan aplikasi: load config -> koneksi DB ->
// migrasi -> seed admin -> router Gin. Dipakai oleh main.go (root) maupun
// cmd/server/main.go agar entry point tetap tipis dan teruji.
package app

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/server"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Variabel berikut dapat diganti saat pengujian.
var (
	LoadConfig  = func() *config.Config { return config.Load() }
	Connect     = database.Connect
	Migrate     = database.Migrate
	Seed        = database.Seed
	StartServer = func(r *gin.Engine, addr string) error { return r.Run(addr) }
	Fatal       = log.Fatal
)

// Run menyiapkan database dan menjalankan server HTTP.
func Run(cfg *config.Config) error {
	db, err := Connect(cfg)
	if err != nil {
		return err
	}
	if err := Setup(db, cfg); err != nil {
		return err
	}
	log.Printf("Delcom Blog API berjalan di http://localhost%s (Swagger: http://localhost%s/swagger/index.html)", cfg.Address(), cfg.Address())
	return StartServer(server.NewRouter(cfg, db), cfg.Address())
}

// Setup menjalankan migrasi tabel dan seeding akun admin.
func Setup(db *gorm.DB, cfg *config.Config) error {
	if err := Migrate(db); err != nil {
		return err
	}
	return Seed(db, cfg)
}

// Main adalah isi fungsi main(): menghentikan proses jika terjadi error.
func Main() {
	if err := Run(LoadConfig()); err != nil {
		Fatal(err)
	}
}
