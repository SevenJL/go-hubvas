// Package main is the entry point for the REST API server.
//
// The API server handles:
//   - User authentication (register, login)
//   - Canvas CRUD (create, read, update, delete, publish, fork)
//   - Community interactions (browse, like, comment)
//   - File export (PNG, SVG, PDF)
//
// WebSocket connections for real-time collaboration are handled by
// the separate ws-server binary, though both can be merged into a
// single process during development.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hubvas/pkg/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas API Server...")

	// Load configuration.
	cfg := config.Config{
		Server: config.ServerConfig{
			APIHost: "0.0.0.0",
			APIPort: 8080,
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "hubvas",
			Password: "",
			DBName:   "hubvas",
			SSLMode:  "disable",
			MaxConns: 20,
		},
	}

	// ---- Wiring (manual DI — replace with Wire once all repos are implemented) ----

	// 1. Infrastructure: Database connection pool.
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL")

	// 2. Infrastructure: Repositories (more coming as they are implemented).
	// userRepo    := postgres.NewUserRepo(pool)
	// canvasRepo  := postgres.NewCanvasRepo(...)
	// communityRepo := postgres.NewCommunityRepo(...)

	// 3. Infrastructure: Services.
	// jwtSvc := infAuth.NewJWTService(
	//     cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret,
	//     cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL,
	// )
	// pwdSvc := infAuth.NewBcryptPasswordService(cfg.Auth.BcryptCost)

	// 4. Application: Use-case orchestrators.
	// authAppSvc := authApp.NewAuthApplicationService(userRepo, jwtSvc, pwdSvc)
	// canvasAppSvc := canvasApp.NewCanvasApplicationService(canvasRepo, idGen)
	// communityAppSvc := communityApp.NewCommunityApplicationService(communityRepo, canvasRepo, idGen)

	// 5. Interfaces: HTTP handlers.
	// authHandler      := handler.NewAuthHandler(authAppSvc)
	// canvasHandler    := handler.NewCanvasHandler(canvasAppSvc)
	// communityHandler := handler.NewCommunityHandler(communityAppSvc)

	// 6. HTTP Router.
	// router := httpd.NewRouter(httpd.RouterConfig{
	//     AuthHandler:      authHandler,
	//     CanvasHandler:    canvasHandler,
	//     CommunityHandler: communityHandler,
	//     TokenSvc:         jwtSvc,
	//     RateLimiter:      middleware.NewRateLimiter(100, 200),
	// })
	// router.Run(fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort))

	addr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	log.Printf("API server would listen on %s (skeleton — dependencies not wired)", addr)

	// Wait for shutdown signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	_ = pool
}
