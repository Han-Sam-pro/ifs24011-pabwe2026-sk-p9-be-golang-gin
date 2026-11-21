// Package server mengonfigurasi router Gin, seluruh rute API, Swagger UI,
// dan handler 404.
package server

import (
	"net/http"
	"time"

	"app/docs"
	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// NewRouter merakit dependency (repository -> service -> handler) dan
// mendaftarkan seluruh rute API.
func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(db), cfg.TokenSecret()))
	postHandler := post.NewHandler(post.NewService(post.NewRepository(db)))
	aiHandler := ai.NewHandler(ai.NewService(
		ai.NewSearchClient(cfg.SearchEngineEndpoint),
		ai.NewOpenAIClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel),
	))

	// Dokumentasi: di balik reverse proxy HTTPS, Swagger UI harus memakai https
	// agar request "Try it out" tidak diblokir browser (mixed content).
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
	if cfg.ForceHTTPS {
		docs.SwaggerInfo.Schemes = []string{"https"}
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
	})
	r.GET("/", func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Delcom Blog API berjalan", gin.H{"docs": "/swagger/index.html"})
	})

	api := r.Group("/api")

	// Publik
	public := api.Group("/public")
	public.GET("/categories", postHandler.Categories)
	public.GET("/posts", postHandler.ListPublic)
	public.GET("/posts/:slug", postHandler.GetPublic)

	// Autentikasi
	authLimiter := middleware.RateLimiter(60, time.Minute)
	requireAuth := middleware.Auth(cfg.TokenSecret())
	authGroup := api.Group("/auth")
	authGroup.POST("/register", authLimiter, authHandler.Register)
	authGroup.POST("/login", authLimiter, authHandler.Login)
	authGroup.POST("/logout", requireAuth, authHandler.Logout)
	authGroup.GET("/profile", requireAuth, authHandler.GetProfile)
	authGroup.PUT("/profile", requireAuth, authHandler.UpdateProfile)

	// Penulis (member & admin dapat menulis artikel)
	posts := api.Group("/posts", requireAuth, middleware.RequireRole(database.RoleMember, database.RoleAdmin))
	posts.GET("", postHandler.ListMine)
	posts.POST("", postHandler.Create)
	posts.GET("/stats", postHandler.Stats)
	posts.POST("/ai-generate", middleware.RateLimiter(20, time.Minute), aiHandler.Generate)
	posts.GET("/:id", postHandler.GetMine)
	posts.PUT("/:id", postHandler.Update)
	posts.PATCH("/:id/toggle-publish", postHandler.TogglePublish)
	posts.DELETE("/:id", postHandler.Delete)

	// Admin
	admin := api.Group("/admin", requireAuth, middleware.RequireRole(database.RoleAdmin))
	admin.GET("/users", postHandler.AdminUsers)
	admin.GET("/posts", postHandler.AdminPosts)
	admin.GET("/users/:id/posts", postHandler.AdminUserPosts)

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})
	return r
}
