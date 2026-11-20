// Delcom Blog REST API
//
// @title                       Delcom Blog API
// @version                     1.0
// @description                 REST API platform publikasi artikel multi-tier (Publik, Penulis, Admin) dengan AI Blog Generator terintegrasi Search Engine.
// @BasePath                    /
// @schemes                     http https
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Masukkan token dengan format: Bearer {token}
package main

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("gagal migrasi database: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("gagal seeding database: %v", err)
	}

	r := server.NewRouter(cfg, db)
	log.Printf("Delcom Blog API berjalan di http://localhost%s (Swagger: http://localhost%s/swagger/index.html)", cfg.Address(), cfg.Address())
	if err := r.Run(cfg.Address()); err != nil {
		log.Fatal(err)
	}
}
