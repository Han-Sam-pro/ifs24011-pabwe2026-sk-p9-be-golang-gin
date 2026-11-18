package auth

import (
	"errors"
	"net/http"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah HTTP handler fitur autentikasi.
type Handler struct{ svc Service }

// NewHandler membuat handler autentikasi.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

// Register godoc
// @Summary      Registrasi penulis baru
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Data registrasi"
// @Success      201 {object} response.Body{data=UserResponse}
// @Failure      400 {object} response.Body
// @Failure      409 {object} response.Body
// @Router       /api/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}
	user, err := h.svc.Register(req)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Registrasi berhasil", user)
}

// Login godoc
// @Summary      Login dan dapatkan token JWT
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Kredensial"
// @Success      200 {object} response.Body{data=LoginResponse}
// @Failure      400 {object} response.Body
// @Failure      401 {object} response.Body
// @Router       /api/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}
	res, err := h.svc.Login(req)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Login berhasil", res)
}

// Logout godoc
// @Summary      Logout (mencabut token aktif)
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Body
// @Failure      401 {object} response.Body
// @Router       /api/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	middleware.RevokeToken(c.GetString(middleware.CtxToken))
	response.Success(c, http.StatusOK, "Logout berhasil", nil)
}

// GetProfile godoc
// @Summary      Profil pengguna yang sedang login
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} response.Body{data=UserResponse}
// @Failure      401 {object} response.Body
// @Router       /api/auth/profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	user, err := h.svc.Profile(middleware.UserID(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil profil", user)
}

// UpdateProfile godoc
// @Summary      Perbarui profil pengguna yang sedang login
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body UpdateProfileRequest true "Data profil"
// @Success      200 {object} response.Body{data=UserResponse}
// @Failure      400 {object} response.Body
// @Failure      409 {object} response.Body
// @Router       /api/auth/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}
	user, err := h.svc.UpdateProfile(middleware.UserID(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diperbarui", user)
}
