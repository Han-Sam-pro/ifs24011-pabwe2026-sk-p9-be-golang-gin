// Package config memuat konfigurasi aplikasi dari file .env / environment.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi aplikasi (mirip .env Laravel).
type Config struct {
	AppEnv  string
	AppPort string
	AppKey  string

	AdminUsername string
	AdminPassword string

	DatabaseURL  string
	DBConnection string
	DBHost       string
	DBPort       string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBSSLMode    string

	SearchEngineEndpoint string
	OpenAIEndpoint       string
	OpenAIKey            string
	OpenAIModel          string
}

// Load membaca file .env (jika ada) lalu environment variable.
// files bersifat opsional; jika kosong maka ".env" yang dibaca.
func Load(files ...string) *Config {
	if len(files) == 0 {
		files = []string{".env"}
	}
	for _, f := range files {
		_ = godotenv.Load(f)
	}

	return &Config{
		AppEnv:  Get("APP_ENV", "development"),
		AppPort: Get("APP_PORT", Get("PORT", "8081")),
		AppKey:  Get("APP_KEY", "delcom_blog_default_secret_key"),

		AdminUsername: Get("ADMIN_USERNAME", "admin"),
		AdminPassword: Get("ADMIN_PASSWORD", "admin"),

		DatabaseURL:  Get("DATABASE_URL", ""),
		DBConnection: Get("DB_CONNECTION", "pgsql"),
		DBHost:       Get("DB_HOST", "127.0.0.1"),
		DBPort:       Get("DB_PORT", "5432"),
		DBDatabase:   Get("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:   Get("DB_USERNAME", "postgres"),
		DBPassword:   Get("DB_PASSWORD", "postgres"),
		DBSSLMode:    Get("DB_SSLMODE", "disable"),

		SearchEngineEndpoint: Get("SEARCH_ENGINE_ENDPOINT", "https://delcom.org/api/v1/search"),
		OpenAIEndpoint:       Get("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:            Get("OPENAI_KEY", ""),
		OpenAIModel:          Get("OPENAI_MODEL", "gpt-4o-mini"),
	}
}

// Get mengembalikan nilai environment variable atau nilai default.
func Get(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// DSN menghasilkan connection string PostgreSQL. Jika DATABASE_URL diisi
// (format Neon: postgresql://user:pass@host/db?sslmode=require), nilai itu
// dipakai langsung dan variabel DB_* diabaikan.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.DBHost, c.DBPort, c.DBUsername, c.DBPassword, c.DBDatabase, c.DBSSLMode,
	)
}

// Address mengembalikan alamat listen server Gin, contoh ":8081".
func (c *Config) Address() string {
	return ":" + c.AppPort
}

// IsProduction bernilai true jika APP_ENV=production.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}
