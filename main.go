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

import "app/internal/app"

func main() {
	app.Main()
}
