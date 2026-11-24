// Package auth menangani registrasi penulis, login JWT, logout, dan profil.
package auth

import (
	"time"

	"app/internal/database"
)

// RegisterRequest adalah payload pendaftaran penulis baru.
type RegisterRequest struct {
	Name     string `json:"name" binding:"omitempty,max=100" example:"Budi Santoso"`
	Username string `json:"username" binding:"required,min=3,max=50,excludesall= " example:"budi"`
	Email    string `json:"email" binding:"required,email,max=150" example:"budi@example.com"`
	Password string `json:"password" binding:"required,min=6,max=72" example:"rahasia123"`
}

// LoginRequest adalah payload login (username atau email).
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin"`
}

// UpdateProfileRequest adalah payload pembaruan profil.
type UpdateProfileRequest struct {
	Name     string `json:"name" binding:"omitempty,max=100" example:"Budi Santoso"`
	Email    string `json:"email" binding:"omitempty,email,max=150" example:"budi@example.com"`
	Password string `json:"password" binding:"omitempty,min=6,max=72" example:""`
}

// UserResponse adalah representasi publik data pengguna.
type UserResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse berisi token JWT dan data pengguna.
type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// ToUserResponse memetakan model User ke UserResponse.
func ToUserResponse(u *database.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}
