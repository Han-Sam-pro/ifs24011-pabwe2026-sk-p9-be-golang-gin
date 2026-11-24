package auth

import (
	"app/internal/database"

	"gorm.io/gorm"
)

// Repository adalah kontrak akses data pengguna.
type Repository interface {
	Create(u *database.User) error
	FindByID(id uint) (*database.User, error)
	FindByLogin(identity string) (*database.User, error)
	IsTaken(username, email string, excludeID uint) (bool, error)
	Update(u *database.User) error
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat repository GORM.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(u *database.User) error {
	return r.db.Create(u).Error
}

func (r *gormRepository) FindByID(id uint) (*database.User, error) {
	var u database.User
	if err := r.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) FindByLogin(identity string) (*database.User, error) {
	var u database.User
	if err := r.db.Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", identity, identity).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) IsTaken(username, email string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).
		Where("(LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)) AND id <> ?", username, email, excludeID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) Update(u *database.User) error {
	return r.db.Save(u).Error
}
