// Package middleware berisi CORS, autentikasi JWT Bearer, pemeriksaan role,
// dan rate limiter sederhana berbasis IP.
package middleware

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Kunci context Gin.
const (
	CtxUserID   = "user_id"
	CtxRole     = "role"
	CtxUsername = "username"
	CtxToken    = "token"
)

// TokenTTL adalah masa berlaku token JWT.
const TokenTTL = 24 * time.Hour

// Claims adalah payload JWT aplikasi.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// ErrTokenRevoked dikembalikan jika token sudah di-logout.
var ErrTokenRevoked = errors.New("token sudah tidak berlaku")

var revoked sync.Map // token -> waktu kedaluwarsa

// GenerateToken membuat token JWT HS256.
func GenerateToken(secret string, userID uint, username, role string, ttl time.Duration) string {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "delcom-blog",
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return token
}

// ParseToken memvalidasi token JWT dan mengembalikan claims-nya.
func ParseToken(secret, tokenString string) (*Claims, error) {
	if _, ok := revoked.Load(tokenString); ok {
		return nil, ErrTokenRevoked
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// RevokeToken menandai token sebagai tidak berlaku (logout).
func RevokeToken(tokenString string) {
	revoked.Store(tokenString, time.Now().Add(TokenTTL))
}

// CORS mengizinkan akses lintas origin dari frontend SPA.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memverifikasi header "Authorization: Bearer <token>".
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Fail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan")
			return
		}
		token := strings.TrimSpace(parts[1])
		claims, err := ParseToken(secret, token)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxToken, token)
		c.Next()
	}
}

// RequireRole membatasi akses hanya untuk role tertentu.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(CtxRole)
		for _, r := range roles {
			if r == role {
				c.Next()
				return
			}
		}
		response.Fail(c, http.StatusForbidden, "Anda tidak memiliki akses ke sumber daya ini")
	}
}

// UserID mengambil ID pengguna yang sedang login dari context.
func UserID(c *gin.Context) uint {
	return c.GetUint(CtxUserID)
}

type visitor struct {
	count int
	reset time.Time
}

// RateLimiter membatasi jumlah request per IP dalam satu jendela waktu.
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}
	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()

		mu.Lock()
		v, ok := visitors[ip]
		if !ok || now.After(v.reset) {
			v = &visitor{reset: now.Add(window)}
			visitors[ip] = v
		}
		v.count++
		exceeded := v.count > limit
		mu.Unlock()

		if exceeded {
			response.Fail(c, http.StatusTooManyRequests, "Terlalu banyak permintaan, coba lagi nanti")
			return
		}
		c.Next()
	}
}
