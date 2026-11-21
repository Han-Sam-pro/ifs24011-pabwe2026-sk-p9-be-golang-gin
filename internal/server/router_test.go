package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/config"
	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type apiResp struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type client struct {
	t *testing.T
	r http.Handler
}

func (c client) do(method, path string, body any, token string) (int, apiResp) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, req)
	var res apiResp
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w.Code, res
}

func (c client) login(username, password string) string {
	code, res := c.do("POST", "/api/auth/login", gin.H{"username": username, "password": password}, "")
	require.Equal(c.t, http.StatusOK, code, res.Message)
	var data struct{ Token string }
	_ = json.Unmarshal(res.Data, &data)
	return data.Token
}

func setup(t *testing.T) client {
	gin.DefaultWriter = io.Discard
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[{"title":"Sumber A","url":"https://a.example","snippet":"fakta"}]}`)
	}))
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := json.Marshal(`{"title":"Artikel AI","category":"Teknologi","excerpt":"Ringkasan","content":"## Isi\n\nFakta [1]."}`)
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s}}]}`, content)
	}))
	t.Cleanup(search.Close)
	t.Cleanup(llm.Close)

	cfg := &config.Config{
		AppEnv: "testing", AppKey: "test-key", AdminUsername: "admin", AdminPassword: "admin",
		SearchEngineEndpoint: search.URL, OpenAIEndpoint: llm.URL, OpenAIKey: "k", OpenAIModel: "m",
	}
	db, err := database.ConnectSQLite("file::memory:")
	require.NoError(t, err)
	require.NoError(t, database.Seed(db, cfg))
	return client{t: t, r: NewRouter(cfg, db)}
}

func TestDocsAndNotFound(t *testing.T) {
	c := setup(t)
	code, res := c.do("GET", "/", nil, "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "success", res.Status)

	code, res = c.do("GET", "/api/unknown", nil, "")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "Endpoint not found", res.Message)

	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, httptest.NewRequest("GET", "/api.json", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Delcom Blog API")

	w = httptest.NewRecorder()
	c.r.ServeHTTP(w, httptest.NewRequest("GET", "/swagger/index.html", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/api/posts", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	c.r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestFullFlow(t *testing.T) {
	c := setup(t)

	code, _ := c.do("GET", "/api/public/categories", nil, "")
	assert.Equal(t, http.StatusOK, code)

	code, _ = c.do("POST", "/api/auth/register", gin.H{"name": "Budi", "username": "budi", "email": "budi@x.com", "password": "rahasia"}, "")
	require.Equal(t, http.StatusCreated, code)
	code, _ = c.do("POST", "/api/auth/register", gin.H{"name": "Budi", "username": "budi", "email": "budi@x.com", "password": "rahasia"}, "")
	assert.Equal(t, http.StatusConflict, code)

	token := c.login("budi", "rahasia")
	code, _ = c.do("GET", "/api/auth/profile", nil, token)
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("PUT", "/api/auth/profile", gin.H{"name": "Budi S", "email": "budi@x.com"}, token)
	assert.Equal(t, http.StatusOK, code)

	code, res := c.do("POST", "/api/posts", gin.H{"title": "Halo Dunia", "category": "Teknologi", "content": "isi artikel", "is_published": true}, token)
	require.Equal(t, http.StatusCreated, code, res.Message)
	var created struct {
		ID   uint
		Slug string
	}
	_ = json.Unmarshal(res.Data, &created)
	assert.Equal(t, "halo-dunia", created.Slug)

	code, _ = c.do("GET", "/api/public/posts?q=halo", nil, "")
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("GET", "/api/public/posts/halo-dunia", nil, "")
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("GET", "/api/posts/stats", nil, token)
	assert.Equal(t, http.StatusOK, code)
	id := fmt.Sprint(created.ID)
	code, _ = c.do("PATCH", "/api/posts/"+id+"/toggle-publish", nil, token)
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("GET", "/api/public/posts/halo-dunia", nil, "")
	assert.Equal(t, http.StatusNotFound, code)

	code, res = c.do("POST", "/api/posts/ai-generate", gin.H{"topic": "Tren AI 2026", "include_web_search": true}, token)
	require.Equal(t, http.StatusOK, code, res.Message)
	assert.Contains(t, string(res.Data), "https://a.example")

	// member tidak boleh mengakses admin
	code, _ = c.do("GET", "/api/admin/users", nil, token)
	assert.Equal(t, http.StatusForbidden, code)

	admin := c.login("admin", "admin")
	code, res = c.do("GET", "/api/admin/users", nil, admin)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(res.Data), "budi")
	code, _ = c.do("GET", "/api/admin/posts", nil, admin)
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("GET", "/api/admin/users/2/posts", nil, admin)
	assert.Equal(t, http.StatusOK, code)

	code, _ = c.do("DELETE", "/api/posts/"+id, nil, token)
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("POST", "/api/auth/logout", nil, token)
	assert.Equal(t, http.StatusOK, code)
	code, _ = c.do("GET", "/api/posts", nil, token)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestProductionMode(t *testing.T) {
	defer gin.SetMode(gin.TestMode)
	db, err := database.ConnectSQLite("file::memory:")
	require.NoError(t, err)
	NewRouter(&config.Config{AppEnv: "production"}, db)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())
}

func TestForceHTTPSAndJWTSecret(t *testing.T) {
	gin.DefaultWriter = io.Discard
	db, err := database.ConnectSQLite("file::memory:")
	require.NoError(t, err)
	cfg := &config.Config{AppEnv: "testing", AppKey: "app-key", JWTSecret: "jwt-secret", ForceHTTPS: true, AdminUsername: "admin", AdminPassword: "admin"}
	require.NoError(t, database.Seed(db, cfg))
	c := client{t: t, r: NewRouter(cfg, db)}
	defer NewRouter(&config.Config{}, db) // kembalikan skema swagger default

	w := httptest.NewRecorder()
	c.r.ServeHTTP(w, httptest.NewRequest("GET", "/api.json", nil))
	assert.Contains(t, w.Body.String(), `"https"`)
	assert.NotContains(t, w.Body.String(), `"http",`)

	token := c.login("admin", "admin")
	_, err = middleware.ParseToken("jwt-secret", token)
	assert.NoError(t, err)
}
