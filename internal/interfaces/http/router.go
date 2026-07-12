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
	SocialHandler    *handler.SocialHandler
	MediaHandler     *handler.MediaHandler
	WSGateway        *ws.Gateway
	TokenSvc         middleware.TokenValidator
	RateLimiter      *middleware.RateLimiter
	UserLookup       middleware.AccountLookup
	TrustedProxies   []string
}

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		panic("invalid trusted proxy configuration: " + err.Error())
	}
	loginLimit := cfg.RateLimiter.Scoped(0.2, 8)
	uploadLimit := cfg.RateLimiter.Scoped(0.1, 5)
	followLimit := cfg.RateLimiter.Scoped(0.5, 12)
	commentLimit := cfg.RateLimiter.Scoped(0.25, 8)
	reportLimit := cfg.RateLimiter.Scoped(0.05, 3)
	adminLimit := cfg.RateLimiter.Scoped(0.5, 10)

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
		auth.POST("/register", loginLimit.KeyedMiddleware("register", false), cfg.AuthHandler.Register)
		auth.POST("/login", loginLimit.KeyedMiddleware("login", false), cfg.AuthHandler.Login)
		auth.POST("/refresh", loginLimit.KeyedMiddleware("refresh", false), cfg.AuthHandler.Refresh)
		auth.POST("/logout", cfg.AuthHandler.Logout)
	}

	// Public user profiles and community read access.
	r.GET("/api/users/:identifier", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.SocialHandler.Profile)
	r.GET("/api/users/:identifier/canvases", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.SocialHandler.UserCanvases)

	// Community — public read access.
	r.GET("/api/community", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.Browse)
	r.GET("/api/community/:id", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.GetPublished)

	// Static canvas routes must be registered before the public /:id wildcard.
	// Otherwise Gin routes /api/canvases/shared to CanvasHandler.Get with id="shared".
	r.GET("/api/canvases/shared", middleware.AuthMiddleware(cfg.TokenSvc), cfg.CanvasHandler.ListShared)

	// Canvas detail — public (for published canvases).
	r.GET("/api/canvases/:id", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CanvasHandler.Get)
	r.GET("/api/canvases/:id/comments", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.GetComments)
	r.GET("/api/canvases/:id/like-status", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.CommunityHandler.LikeStatus)
	r.GET("/api/canvases/:id/snapshot", middleware.OptionalAuthMiddleware(cfg.TokenSvc), cfg.SnapshotHandler.Load)

	// ---- Protected routes (JWT required) ----

	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(cfg.TokenSvc), middleware.ActiveAccountMiddleware(cfg.UserLookup))
	{
		// Auth and profile media
		api.GET("/auth/me", cfg.AuthHandler.Me)
		api.POST("/auth/logout-all", cfg.AuthHandler.LogoutAll)
		api.PUT("/auth/profile", cfg.AuthHandler.UpdateProfile)
		api.PATCH("/auth/profile", cfg.AuthHandler.UpdateProfile)
		api.DELETE("/auth/avatar", cfg.MediaHandler.Remove)
		api.POST("/media/avatars/presign", uploadLimit.KeyedMiddleware("avatar", true), cfg.MediaHandler.Presign)
		api.POST("/media/avatars/complete", uploadLimit.KeyedMiddleware("avatar", true), cfg.MediaHandler.Complete)
		api.POST("/media/avatars", uploadLimit.KeyedMiddleware("avatar", true), cfg.MediaHandler.Upload)

		// Social graph, personalized feed, notifications and safety
		api.POST("/users/:identifier/follow", followLimit.KeyedMiddleware("follow", true), cfg.SocialHandler.Follow)
		api.DELETE("/users/:identifier/follow", followLimit.KeyedMiddleware("follow", true), cfg.SocialHandler.Unfollow)
		api.GET("/users/:identifier/followers", cfg.SocialHandler.Followers)
		api.GET("/users/:identifier/following", cfg.SocialHandler.Following)
		api.POST("/users/:identifier/block", cfg.SocialHandler.Block)
		api.DELETE("/users/:identifier/block", cfg.SocialHandler.Unblock)
		api.GET("/blocks", cfg.SocialHandler.Blocks)
		api.GET("/community/following", cfg.SocialHandler.FollowingFeed)
		api.GET("/notifications", cfg.SocialHandler.Notifications)
		api.GET("/notifications/unread-count", cfg.SocialHandler.UnreadCount)
		api.PATCH("/notifications/:id/read", cfg.SocialHandler.MarkRead)
		api.POST("/notifications/read-all", cfg.SocialHandler.MarkAllRead)
		api.POST("/reports", reportLimit.KeyedMiddleware("report", true), cfg.SocialHandler.Report)
		api.GET("/admin/reports", adminLimit.KeyedMiddleware("admin", true), cfg.SocialHandler.AdminReports)
		api.PATCH("/admin/reports/:id", adminLimit.KeyedMiddleware("admin", true), cfg.SocialHandler.ReviewReport)
		api.PATCH("/admin/users/:identifier/status", adminLimit.KeyedMiddleware("admin", true), cfg.SocialHandler.UserStatus)
		api.PATCH("/admin/comments/:id/moderation", adminLimit.KeyedMiddleware("admin", true), cfg.SocialHandler.ModerateComment)
		api.PATCH("/admin/canvases/:id/moderation", adminLimit.KeyedMiddleware("admin", true), cfg.SocialHandler.ModerateCanvas)

		// Canvases (write operations)
		api.POST("/canvases", cfg.CanvasHandler.Create)
		api.GET("/canvases", cfg.CanvasHandler.ListMine)
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
		api.POST("/canvases/:id/comments", commentLimit.KeyedMiddleware("comment", true), cfg.CommunityHandler.PostComment)
		api.DELETE("/comments/:id", cfg.CommunityHandler.DeleteComment)

		// Snapshots (save requires auth)
		api.PUT("/canvases/:id/snapshot", cfg.SnapshotHandler.Save)
	}

	// WebSocket (JWT via Sec-WebSocket-Protocol, verified in the gateway).
	// The API-only process intentionally leaves WSGateway nil.
	if cfg.WSGateway != nil {
		r.GET("/ws", func(c *gin.Context) {
			cfg.WSGateway.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
