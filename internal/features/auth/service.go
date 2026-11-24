package auth

import (
	"errors"
	"strings"

	"app/internal/database"
	"app/internal/middleware"

	"gorm.io/gorm"
)

// Error domain autentikasi.
var (
	ErrConflict           = errors.New("username atau email sudah digunakan")
	ErrInvalidCredentials = errors.New("username atau password salah")
	ErrUserNotFound       = errors.New("pengguna tidak ditemukan")
)

// Service adalah kontrak logika bisnis autentikasi.
type Service interface {
	Register(req RegisterRequest) (*LoginResponse, error)
	Login(req LoginRequest) (*LoginResponse, error)
	Profile(userID uint) (*UserResponse, error)
	UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error)
}

type service struct {
	repo   Repository
	secret string
}

// NewService membuat service autentikasi.
func NewService(repo Repository, secret string) Service {
	return &service{repo: repo, secret: secret}
}

func (s *service) Register(req RegisterRequest) (*LoginResponse, error) {
	// Username & email disimpan apa adanya; pencocokan tidak peka huruf besar/kecil.
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		// Nama opsional: gunakan username jika tidak diisi.
		name = username
	}

	taken, err := s.repo.IsTaken(username, email, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrConflict
	}
	hash, err := database.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &database.User{
		Name:     name,
		Username: username,
		Email:    email,
		Password: hash,
		Role:     database.RoleMember,
	}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	// Registrasi langsung mengembalikan token JWT (langsung login).
	return s.issueToken(user), nil
}

func (s *service) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindByLogin(strings.TrimSpace(req.Username))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !database.CheckPassword(user.Password, req.Password) {
		return nil, ErrInvalidCredentials
	}
	return s.issueToken(user), nil
}

func (s *service) issueToken(user *database.User) *LoginResponse {
	token := middleware.GenerateToken(s.secret, user.ID, user.Username, user.Role, middleware.TokenTTL)
	return &LoginResponse{Token: token, User: ToUserResponse(user)}
}

func (s *service) findUser(userID uint) (*database.User, error) {
	user, err := s.repo.FindByID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (s *service) Profile(userID uint) (*UserResponse, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}
	res := ToUserResponse(user)
	return &res, nil
}

func (s *service) UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = user.Email // field kosong = tidak diubah
	}
	taken, err := s.repo.IsTaken(user.Username, email, user.ID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrConflict
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		user.Name = name
	}
	user.Email = email
	if req.Password != "" {
		hash, err := database.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.Password = hash
	}
	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	res := ToUserResponse(user)
	return &res, nil
}
