package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func run(fn func(c *gin.Context)) (*httptest.ResponseRecorder, map[string]any) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fn(c)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestSuccess(t *testing.T) {
	w, body := run(func(c *gin.Context) { Success(c, http.StatusOK, "ok", gin.H{"a": 1}) })
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "ok", body["message"])
	assert.NotNil(t, body["data"])
}

func TestSuccessWithoutData(t *testing.T) {
	_, body := run(func(c *gin.Context) { Success(c, http.StatusOK, "ok", nil) })
	_, has := body["data"]
	assert.False(t, has)
}

func TestFail(t *testing.T) {
	w, body := run(func(c *gin.Context) { Fail(c, http.StatusBadRequest, "bad") })
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "fail", body["status"])
}

func TestError(t *testing.T) {
	w, body := run(func(c *gin.Context) { Error(c, http.StatusInternalServerError, "boom") })
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, "boom", body["message"])
}
