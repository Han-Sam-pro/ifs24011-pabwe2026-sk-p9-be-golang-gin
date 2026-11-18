package post

import (
	"strings"

	"app/internal/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository adalah kontrak akses data artikel.
type Repository interface {
	ListPublished(f ListFilter) ([]database.Post, error)
	FindPublishedBySlug(slug string) (*database.Post, error)
	ListByUser(userID uint, f ListFilter) ([]database.Post, error)
	FindByIDForUser(id, userID uint) (*database.Post, error)
	Create(p *database.Post) error
	Update(p *database.Post) error
	Delete(p *database.Post) error
	SlugExists(slug string, excludeID uint) (bool, error)
	CountByUser(userID uint) (total, published int64, err error)
	ListAll(f ListFilter) ([]database.Post, error)
	ListUsersWithStats() ([]UserSummary, error)
	UserExists(id uint) (bool, error)
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat repository GORM untuk artikel.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func applyFilter(q *gorm.DB, f ListFilter) *gorm.DB {
	if kw := strings.ToLower(strings.TrimSpace(f.Q)); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(LOWER(posts.title) LIKE ? OR LOWER(posts.excerpt) LIKE ? OR LOWER(posts.content) LIKE ?)", like, like, like)
	}
	if c := strings.TrimSpace(f.Category); c != "" && !strings.EqualFold(c, "semua") {
		q = q.Where("posts.category = ?", c)
	}
	return q
}

func (r *gormRepository) ListPublished(f ListFilter) ([]database.Post, error) {
	var posts []database.Post
	err := applyFilter(r.db.Preload("User").Where("posts.is_published = ?", true), f).
		Order("posts.published_at DESC").Order("posts.id DESC").
		Find(&posts).Error
	return posts, err
}

func (r *gormRepository) FindPublishedBySlug(slug string) (*database.Post, error) {
	var p database.Post
	if err := r.db.Preload("User").Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) ListByUser(userID uint, f ListFilter) ([]database.Post, error) {
	var posts []database.Post
	err := applyFilter(r.db.Preload("User").Where("posts.user_id = ?", userID), f).
		Order("posts.updated_at DESC").Order("posts.id DESC").
		Find(&posts).Error
	return posts, err
}

func (r *gormRepository) FindByIDForUser(id, userID uint) (*database.Post, error) {
	var p database.Post
	if err := r.db.Preload("User").Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) Create(p *database.Post) error {
	return r.db.Omit(clause.Associations).Create(p).Error
}

func (r *gormRepository) Update(p *database.Post) error {
	return r.db.Omit(clause.Associations).Save(p).Error
}

func (r *gormRepository) Delete(p *database.Post) error {
	return r.db.Delete(&database.Post{}, p.ID).Error
}

func (r *gormRepository) SlugExists(slug string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.Post{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) CountByUser(userID uint) (int64, int64, error) {
	var total, published int64
	if err := r.db.Model(&database.Post{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err := r.db.Model(&database.Post{}).Where("user_id = ? AND is_published = ?", userID, true).Count(&published).Error
	return total, published, err
}

func (r *gormRepository) ListAll(f ListFilter) ([]database.Post, error) {
	var posts []database.Post
	err := applyFilter(r.db.Preload("User"), f).Order("posts.created_at DESC").Order("posts.id DESC").Find(&posts).Error
	return posts, err
}

func (r *gormRepository) ListUsersWithStats() ([]UserSummary, error) {
	users := []UserSummary{}
	err := r.db.Table("users").
		Select(`users.id, users.name, users.username, users.email, users.role,
			COUNT(posts.id) AS total_posts,
			COALESCE(SUM(CASE WHEN posts.is_published THEN 1 ELSE 0 END), 0) AS published_posts`).
		Joins("LEFT JOIN posts ON posts.user_id = users.id").
		Group("users.id, users.name, users.username, users.email, users.role").
		Order("users.id ASC").
		Scan(&users).Error
	return users, err
}

func (r *gormRepository) UserExists(id uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.User{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}
