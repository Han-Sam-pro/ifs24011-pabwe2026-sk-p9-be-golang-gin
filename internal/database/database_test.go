package database

import (
	"strings"
	"testing"

	"app/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func memDB(t *testing.T) *gorm.DB {
	db, err := ConnectSQLite("file::memory:")
	require.NoError(t, err)
	return db
}

func TestIsValidCategory(t *testing.T) {
	assert.True(t, IsValidCategory("Teknologi"))
	assert.False(t, IsValidCategory("Olahraga"))
}

func TestDialector(t *testing.T) {
	assert.Equal(t, "sqlite", Dialector(&config.Config{DBConnection: "sqlite", DBDatabase: ":memory:"}).Name())
	assert.Equal(t, "postgres", Dialector(&config.Config{DBConnection: "pgsql"}).Name())
}

func TestConnect(t *testing.T) {
	db, err := Connect(&config.Config{DBConnection: "sqlite", DBDatabase: "file::memory:"})
	require.NoError(t, err)
	assert.NotNil(t, db)

	db, err = Connect(&config.Config{AppEnv: "production", DBConnection: "sqlite", DBDatabase: "/nonexistent-dir/x/y.db"})
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestConnectSQLiteError(t *testing.T) {
	_, err := ConnectSQLite("/nonexistent-dir/x/y.db")
	assert.Error(t, err)
}

func TestPasswordHelpers(t *testing.T) {
	hash, err := HashPassword("secret")
	require.NoError(t, err)
	assert.True(t, CheckPassword(hash, "secret"))
	assert.False(t, CheckPassword(hash, "wrong"))
}

func TestSeed(t *testing.T) {
	g := memDB(t)
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: "admin"}
	require.NoError(t, Seed(g, cfg))
	require.NoError(t, Seed(g, cfg)) // idempotent

	var admin User
	require.NoError(t, g.Where("username = ?", "admin").First(&admin).Error)
	assert.Equal(t, RoleAdmin, admin.Role)
	assert.True(t, CheckPassword(admin.Password, "admin"))
}

func TestSeedHashError(t *testing.T) {
	g := memDB(t)
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: strings.Repeat("x", 100)}
	assert.Error(t, Seed(g, cfg))
}

func TestSeedCreateError(t *testing.T) {
	g := memDB(t)
	require.NoError(t, g.Create(&User{Name: "x", Username: "other", Email: "root@delcom.org", Password: "x"}).Error)
	assert.Error(t, Seed(g, &config.Config{AdminUsername: "root", AdminPassword: "pw"}))
}

func TestSeedQueryError(t *testing.T) {
	g := memDB(t)
	sqlDB, _ := g.DB()
	sqlDB.Close()
	assert.Error(t, Seed(g, &config.Config{AdminUsername: "admin", AdminPassword: "pw"}))
}
