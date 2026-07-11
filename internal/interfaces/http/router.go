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
	HealthHandler    *handler.HealthHandler
	SnapshotHandler  *handler.SnapshotHandler
	WSGateway        *ws.Gateway
	TokenSvc         middleware.TokenValidator
	RateLimiter      *middleware.RateLimiter
}

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()

	// Global middleware.
	r.Use(middleware.Recovery())
	r.Use(cfg.RateLimiter.Middleware())
	r.Use(gin.Logger())

	// ---- Health check (no auth) ----
	r.GET("/health", cfg.HealthHandler.Health)
	r.GET("/ready", cfg.HealthHandler.Ready)

	// ---- Public routes (no auth required) ----

	auth := r.Group("/api/auth")
	{
		auth.POST("/register", cfg.AuthHandler.Register)
		auth.POST("/login", cfg.AuthHandler.Login)
		auth.POST("/refresh", cfg.AuthHandler.Refresh)
	}

	// Community — public read access.
	r.GET("/api/community", cfg.CommunityHandler.Browse)
	r.GET("/api/community/:id", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.GetPublished)

	// Canvas detail — public (for published canvases).
	r.GET("/api/canvases/:id", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CanvasHandler.Get)
	r.GET("/api/canvases/:id/comments", cfg.CommunityHandler.GetComments)
	r.GET("/api/canvases/:id/like-status", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.LikeStatus)
	r.GET("/api/canvases/:id/snapshot", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.SnapshotHandler.Load)

	// ---- Protected routes (JWT required) ----

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(cfg.TokenSvc))
	{
		// Auth
		api.GET("/auth/me", cfg.AuthHandler.Me)
		api.PUT("/auth/profile", cfg.AuthHandler.UpdateProfile)

		// Canvases (write operations)
		api.POST("/canvases", cfg.CanvasHandler.Create)
		api.GET("/canvases", cfg.CanvasHandler.ListMine)
		api.GET("/canvases/shared", cfg.CanvasHandler.ListShared)
		api.GET("/canvases/:id/members", cfg.CanvasHandler.ListMembers)
		api.POST("/canvases/:id/members", cfg.CanvasHandler.AddMember)
		api.PUT("/canvases/:id/members/:userId", cfg.CanvasHandler.UpdateMemberRole)
		api.DELETE("/canvases/:id/members/:userId", cfg.CanvasHandler.RemoveMember)
		api.POST("/canvases/:id/publish", cfg.CanvasHandler.Publish)
		api.POST("/canvases/:id/fork", cfg.CanvasHandler.Fork)
		api.DELETE("/canvases/:id", cfg.CanvasHandler.Delete)

		// Community (write operations)
		api.POST("/canvases/:id/like", cfg.CommunityHandler.Like)
		api.DELETE("/canvases/:id/like", cfg.CommunityHandler.Unlike)
		api.POST("/canvases/:id/comments", cfg.CommunityHandler.PostComment)

		// Snapshots (save requires auth)
		api.PUT("/canvases/:id/snapshot", cfg.SnapshotHandler.Save)
	}

	// WebSocket (JWT via query param, verified in the gateway).
	// The API-only process intentionally leaves WSGateway nil.
	if cfg.WSGateway != nil {
		r.GET("/ws", func(c *gin.Context) {
			cfg.WSGateway.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
