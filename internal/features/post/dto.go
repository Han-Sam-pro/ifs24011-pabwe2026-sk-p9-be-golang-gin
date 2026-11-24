// Package post menangani CRUD artikel, slug unik, reading time,
// statistik penulis, galeri publik, dan monitoring admin.
package post

import (
	"time"

	"app/internal/database"
)

// PostRequest adalah payload pembuatan/pembaruan artikel.
type PostRequest struct {
	Title          string `json:"title" binding:"required,max=255" example:"Mengenal Clean Architecture di Golang"`
	Category       string `json:"category" binding:"omitempty,max=100" example:"Teknologi"`
	Excerpt        string `json:"excerpt" example:"Ringkasan singkat artikel."`
	Content        string `json:"content" binding:"required" example:"# Judul\n\nIsi artikel dalam **Markdown**."`
	CoverImagePath string `json:"cover_image_path" example:"https://picsum.photos/1200/600"`
	IsPublished    bool   `json:"is_published" example:"false"`
}

// ListFilter adalah parameter query pencarian artikel.
type ListFilter struct {
	Q        string `form:"q"`
	Category string `form:"category"`
}

// AuthorResponse adalah informasi singkat penulis artikel.
type AuthorResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// PostResponse adalah representasi artikel pada respons API.
type PostResponse struct {
	ID             uint            `json:"id"`
	UserID         uint            `json:"user_id,omitempty"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Category       string          `json:"category"`
	Excerpt        string          `json:"excerpt"`
	Content        string          `json:"content"`
	CoverImagePath string          `json:"cover_image_path"`
	IsPublished    bool            `json:"is_published"`
	PublishedAt    *time.Time      `json:"published_at"`
	ReadingTime    int             `json:"reading_time" example:"3"`
	Author         *AuthorResponse `json:"author,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// StatsResponse adalah ringkasan 3 kartu statistik penulis.
type StatsResponse struct {
	Total     int64 `json:"total" example:"10"`
	Published int64 `json:"published" example:"7"`
	Draft     int64 `json:"draft" example:"3"`

	// Nama kunci 3 kartu statistik: Total Artikel, Total Selesai, Belum Selesai.
	TotalArticles int64 `json:"total_articles" example:"10"`
	TotalSelesai  int64 `json:"total_selesai" example:"7"`
	BelumSelesai  int64 `json:"belum_selesai" example:"3"`
}

// UserSummary adalah ringkasan pengguna untuk monitoring admin.
type UserSummary struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	TotalPosts     int64  `json:"total_posts"`
	PublishedPosts int64  `json:"published_posts"`
	TotalArticles  int64  `json:"total_articles" gorm:"-"`
	TotalSelesai   int64  `json:"total_selesai" gorm:"-"`
	BelumSelesai   int64  `json:"belum_selesai" gorm:"-"`
}

// ToPostResponse memetakan model Post ke PostResponse.
func ToPostResponse(p *database.Post) PostResponse {
	res := PostResponse{
		ID:             p.ID,
		UserID:         p.UserID,
		Title:          p.Title,
		Slug:           p.Slug,
		Category:       p.Category,
		Excerpt:        p.Excerpt,
		Content:        p.Content,
		CoverImagePath: p.CoverImagePath,
		IsPublished:    p.IsPublished,
		PublishedAt:    p.PublishedAt,
		ReadingTime:    ReadingTime(p.Content),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	if p.User.ID != 0 {
		res.Author = &AuthorResponse{ID: p.User.ID, Name: p.User.Name, Username: p.User.Username}
	}
	return res
}

// ToPostResponses memetakan slice Post.
func ToPostResponses(posts []database.Post) []PostResponse {
	out := make([]PostResponse, 0, len(posts))
	for i := range posts {
		out = append(out, ToPostResponse(&posts[i]))
	}
	return out
}
