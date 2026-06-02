package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/internal/interfaces/ws"
)

// RouterConfig holds all dependencies needed to build the HTTP router.
type RouterConfig struct {
	AuthHandler      *handler.AuthHandler
	CanvasHandler    *handler.CanvasHandler
	CommunityHandler *handler.CommunityHandler
	WSGateway        *ws.Gateway
	TokenSvc         middleware.TokenValidator
	RateLimiter      *middleware.RateLimiter
}

// NewRouter creates and configures the Gin router with all routes.
// Routes matching the spec (§7):
//
//	POST   /api/auth/register
//	POST   /api/auth/login
//	GET    /api/auth/me
//	POST   /api/canvases
//	GET    /api/canvases/:id
//	GET    /api/canvases             (list mine)
//	POST   /api/canvases/:id/publish
//	POST   /api/canvases/:id/fork
//	DELETE /api/canvases/:id
//	GET    /api/community
//	POST   /api/canvases/:id/like
//	DELETE /api/canvases/:id/like
//	POST   /api/canvases/:id/comments
//	GET    /api/canvases/:id/comments
//	GET    /ws                        (WebSocket upgrade)
func NewRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()

	// Global middleware.
	r.Use(middleware.Recovery())
	r.Use(cfg.RateLimiter.Middleware())
	r.Use(gin.Logger())

	// Public routes (no auth required).
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", cfg.AuthHandler.Register)
		auth.POST("/login", cfg.AuthHandler.Login)
	}

	// Protected routes (JWT required).
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(cfg.TokenSvc))
	{
		// Auth
		api.GET("/auth/me", cfg.AuthHandler.Me)

		// Canvases
		api.POST("/canvases", cfg.CanvasHandler.Create)
		api.GET("/canvases", cfg.CanvasHandler.ListMine)
		api.GET("/canvases/:id", cfg.CanvasHandler.Get)
		api.POST("/canvases/:id/publish", cfg.CanvasHandler.Publish)
		api.POST("/canvases/:id/fork", cfg.CanvasHandler.Fork)
		api.DELETE("/canvases/:id", cfg.CanvasHandler.Delete)

		// Community
		api.GET("/community", cfg.CommunityHandler.Browse)
		api.POST("/canvases/:id/like", cfg.CommunityHandler.Like)
		api.DELETE("/canvases/:id/like", cfg.CommunityHandler.Unlike)
		api.POST("/canvases/:id/comments", cfg.CommunityHandler.PostComment)
		api.GET("/canvases/:id/comments", cfg.CommunityHandler.GetComments)
	}

	// WebSocket (JWT via query param, verified in the gateway).
	r.GET("/ws", func(c *gin.Context) {
		cfg.WSGateway.ServeHTTP(c.Writer, c.Request)
	})

	return r
}
