package post

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"app/internal/database"

	"gorm.io/gorm"
)

// Error domain artikel.
var (
	ErrPostNotFound    = errors.New("artikel tidak ditemukan")
	ErrUserNotFound    = errors.New("pengguna tidak ditemukan")
	ErrInvalidCategory = errors.New("kategori tidak valid")
)

// WordsPerMinute adalah kecepatan baca rata-rata untuk reading time.
const WordsPerMinute = 200

const maxSlugLength = 200

// now dapat diganti saat pengujian.
var now = time.Now

// Slugify mengubah judul menjadi slug URL, contoh "Halo Dunia!" -> "halo-dunia".
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugLength {
		slug = strings.Trim(slug[:maxSlugLength], "-")
	}
	if slug == "" {
		slug = "artikel"
	}
	return slug
}

// ReadingTime menghitung estimasi waktu baca (menit) = jumlah kata / 200 wpm.
func ReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := (words + WordsPerMinute - 1) / WordsPerMinute
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}

// NormalizeCategory mengisi kategori default dan memvalidasinya.
func NormalizeCategory(category string) (string, error) {
	c := strings.TrimSpace(category)
	if c == "" {
		return database.DefaultCategory, nil
	}
	for _, valid := range database.Categories {
		if strings.EqualFold(valid, c) {
			return valid, nil
		}
	}
	return "", ErrInvalidCategory
}

func setPublished(p *database.Post, published bool) {
	if published && !p.IsPublished {
		t := now()
		p.PublishedAt = &t
	}
	if !published {
		p.PublishedAt = nil
	}
	p.IsPublished = published
}

// Service adalah kontrak logika bisnis artikel.
type Service interface {
	Categories() []string
	ListPublic(f ListFilter) ([]PostResponse, error)
	GetPublicBySlug(slug string) (*PostResponse, error)
	ListMine(userID uint, f ListFilter) ([]PostResponse, error)
	GetMine(userID, id uint) (*PostResponse, error)
	Create(userID uint, req PostRequest) (*PostResponse, error)
	Update(userID, id uint, req PostRequest) (*PostResponse, error)
	TogglePublish(userID, id uint) (*PostResponse, error)
	Delete(userID, id uint) error
	Stats(userID uint) (*StatsResponse, error)
	AdminUsers() ([]UserSummary, error)
	AdminPosts(f ListFilter) ([]PostResponse, error)
	AdminUserPosts(userID uint) ([]PostResponse, error)
}

type service struct{ repo Repository }

// NewService membuat service artikel.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPostNotFound
	}
	return err
}

func (s *service) uniqueSlug(title string, excludeID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *service) Categories() []string {
	return database.Categories
}

func (s *service) ListPublic(f ListFilter) ([]PostResponse, error) {
	posts, err := s.repo.ListPublished(f)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *service) GetPublicBySlug(slug string) (*PostResponse, error) {
	p, err := s.repo.FindPublishedBySlug(slug)
	if err != nil {
		return nil, mapNotFound(err)
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *service) ListMine(userID uint, f ListFilter) ([]PostResponse, error) {
	posts, err := s.repo.ListByUser(userID, f)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *service) GetMine(userID, id uint) (*PostResponse, error) {
	p, err := s.repo.FindByIDForUser(id, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *service) Create(userID uint, req PostRequest) (*PostResponse, error) {
	category, err := NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	slug, err := s.uniqueSlug(req.Title, 0)
	if err != nil {
		return nil, err
	}
	p := &database.Post{
		UserID:         userID,
		Title:          strings.TrimSpace(req.Title),
		Slug:           slug,
		Category:       category,
		Excerpt:        strings.TrimSpace(req.Excerpt),
		Content:        req.Content,
		CoverImagePath: strings.TrimSpace(req.CoverImagePath),
	}
	setPublished(p, req.IsPublished)
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *service) Update(userID, id uint, req PostRequest) (*PostResponse, error) {
	p, err := s.repo.FindByIDForUser(id, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	category, err := NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title != p.Title {
		slug, err := s.uniqueSlug(title, p.ID)
		if err != nil {
			return nil, err
		}
		p.Slug = slug
	}
	p.Title = title
	p.Category = category
	p.Excerpt = strings.TrimSpace(req.Excerpt)
	p.Content = req.Content
	p.CoverImagePath = strings.TrimSpace(req.CoverImagePath)
	setPublished(p, req.IsPublished)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *service) TogglePublish(userID, id uint) (*PostResponse, error) {
	p, err := s.repo.FindByIDForUser(id, userID)
	if err != nil {
		return nil, mapNotFound(err)
	}
	setPublished(p, !p.IsPublished)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *service) Delete(userID, id uint) error {
	p, err := s.repo.FindByIDForUser(id, userID)
	if err != nil {
		return mapNotFound(err)
	}
	return s.repo.Delete(p)
}

func (s *service) Stats(userID uint) (*StatsResponse, error) {
	total, published, err := s.repo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	return &StatsResponse{Total: total, Published: published, Draft: total - published}, nil
}

func (s *service) AdminUsers() ([]UserSummary, error) {
	return s.repo.ListUsersWithStats()
}

func (s *service) AdminPosts(f ListFilter) ([]PostResponse, error) {
	posts, err := s.repo.ListAll(f)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *service) AdminUserPosts(userID uint) ([]PostResponse, error) {
	exists, err := s.repo.UserExists(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}
	return s.ListMine(userID, ListFilter{})
}
