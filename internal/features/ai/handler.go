package ai

import (
	"errors"
	"net/http"

	"app/internal/features/post"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah HTTP handler AI Blog Generator.
type Handler struct{ svc Service }

// NewHandler membuat handler AI.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// Generate godoc
// @Summary      Generate draf artikel via AI + Search Engine
// @Description  Menelusuri topik ke Search Engine (opsional), lalu mengirim prompt + konteks ke OpenAI Chat Completions. Menghasilkan judul, kategori, excerpt, konten Markdown, dan daftar sumber rujukan.
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body GenerateRequest true "Topik artikel"
// @Success      200 {object} response.Body{data=GenerateResponse}
// @Failure      400 {object} response.Body
// @Router       /api/posts/ai-generate [post]
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}
	res, err := h.svc.Generate(c.Request.Context(), req)
	switch {
	case err == nil:
		response.Success(c, http.StatusOK, "Draf artikel berhasil dibuat", res)
	case errors.Is(err, post.ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}
