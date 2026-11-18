package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret"

func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw...)
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": UserID(c), "role": c.GetString(CtxRole)})
	}
	r.GET("/x", handler)
	r.OPTIONS("/x", handler)
	return r
}

func do(r http.Handler, method, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/x", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestTokenRoundTrip(t *testing.T) {
	tok := GenerateToken(secret, 7, "budi", "member", time.Hour)
	claims, err := ParseToken(secret, tok)
	require.NoError(t, err)
	assert.Equal(t, uint(7), claims.UserID)
	assert.Equal(t, "budi", claims.Username)
	assert.Equal(t, "member", claims.Role)

	_, err = ParseToken("other", tok)
	assert.Error(t, err)

	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: 1}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	_, err = ParseToken(secret, none)
	assert.Error(t, err)

	expired := GenerateToken(secret, 1, "a", "member", -time.Hour)
	_, err = ParseToken(secret, expired)
	assert.Error(t, err)
}

func TestRevokeToken(t *testing.T) {
	tok := GenerateToken(secret, 9, "x", "member", time.Hour)
	RevokeToken(tok)
	_, err := ParseToken(secret, tok)
	assert.ErrorIs(t, err, ErrTokenRevoked)
}

func TestCORS(t *testing.T) {
	r := newEngine(CORS())
	w := do(r, http.MethodOptions, "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:5174", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestAuth(t *testing.T) {
	r := newEngine(Auth(secret))
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "Basic abc").Code)
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "Bearer  ").Code)
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "Bearer invalid").Code)

	tok := GenerateToken(secret, 3, "c", "admin", time.Hour)
	w := do(r, http.MethodGet, "Bearer "+tok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"user_id":3`)
}

func TestRequireRole(t *testing.T) {
	r := newEngine(Auth(secret), RequireRole("admin"))
	member := GenerateToken(secret, 1, "m", "member", time.Hour)
	admin := GenerateToken(secret, 2, "a", "admin", time.Hour)
	assert.Equal(t, http.StatusForbidden, do(r, http.MethodGet, "Bearer "+member).Code)
	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "Bearer "+admin).Code)
}

func TestRateLimiter(t *testing.T) {
	r := newEngine(RateLimiter(2, 50*time.Millisecond))
	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, do(r, http.MethodGet, "").Code)
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "").Code)
}
