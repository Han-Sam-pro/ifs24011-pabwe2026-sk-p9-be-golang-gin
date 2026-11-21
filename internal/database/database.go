// Package database mengelola koneksi PostgreSQL (runtime), SQLite in-memory
// (testing), model GORM, migrasi, dan seeding akun admin.
package database

import (
	"fmt"
	"time"

	"app/internal/config"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Role pengguna.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Categories adalah daftar kategori artikel yang valid.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// DefaultCategory dipakai jika kategori tidak diisi.
const DefaultCategory = "Umum"

// IsValidCategory memeriksa apakah kategori termasuk daftar yang valid.
func IsValidCategory(category string) bool {
	for _, c := range Categories {
		if c == category {
			return true
		}
	}
	return false
}

// User merepresentasikan tabel users.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Username  string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Role      string    `gorm:"size:20;not null;default:member" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post merepresentasikan tabel posts.
type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	User           User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Title          string     `gorm:"type:varchar(255);not null" json:"title"`
	Slug           string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"type:varchar(100);not null;default:Umum" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text" json:"content"`
	CoverImagePath string     `gorm:"type:text" json:"cover_image_path"`
	IsPublished    bool       `gorm:"not null;default:false" json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Dialector membuat dialector GORM sesuai DB_CONNECTION.
func Dialector(cfg *config.Config) gorm.Dialector {
	if cfg.DBConnection == "sqlite" {
		return sqlite.Open(cfg.DBDatabase)
	}
	// PreferSimpleProtocol: tanpa prepared statement bernama, agar aman di
	// belakang PgBouncer mode transaction (platform deploy) dan pooler Neon.
	return postgres.New(postgres.Config{DSN: cfg.DSN(), PreferSimpleProtocol: true})
}

// Connect membuka koneksi database untuk runtime.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	level := logger.Warn
	if cfg.IsProduction() {
		level = logger.Error
	}
	db, err := gorm.Open(Dialector(cfg), &gorm.Config{Logger: logger.Default.LogMode(level)})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	return db, nil
}

// ConnectSQLite membuka database SQLite (mis. "file::memory:" untuk TDD)
// lalu langsung menjalankan migrasi.
func ConnectSQLite(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	return db, Migrate(db)
}

// Migrate menjalankan auto migration tabel users & posts.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Post{})
}

// HashPassword meng-hash password menggunakan bcrypt.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword membandingkan hash bcrypt dengan password plain.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Seed membuat akun admin default jika belum ada.
func Seed(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&User{}).Where("username = ?", cfg.AdminUsername).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	admin := User{
		Name:     "Administrator",
		Username: cfg.AdminUsername,
		Email:    cfg.AdminUsername + "@delcom.org",
		Password: hash,
		Role:     RoleAdmin,
	}
	return db.Create(&admin).Error
}
