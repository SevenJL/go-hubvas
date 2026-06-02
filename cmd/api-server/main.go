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
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	}

	// TODO: Wire up dependencies using Google Wire or manual DI.
	// For the skeleton, we just demonstrate the startup sequence.
	//
	// Infrastructure:
	//   db, _   := sql.Open("postgres", cfg.Database.DSN())
	//   userRepo  := postgres.NewUserRepo(db)
	//   canvasRepo := postgres.NewCanvasRepo(db)
	//
	// Domain services:
	//   jwtSvc   := auth.NewJWTService(cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret, ...)
	//   pwdSvc   := auth.NewBcryptPasswordService(cfg.Auth.BcryptCost)
	//
	// Application services:
	//   authAppSvc := authApp.NewAuthApplicationService(userRepo, jwtSvc, pwdSvc)
	//
	// HTTP layer:
	//   authHandler := handler.NewAuthHandler(authAppSvc)
	//   router := http.NewRouter(...)
	//   router.Run(fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort))

	addr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	log.Printf("API server would listen on %s (skeleton — dependencies not wired)", addr)

	// Wait for shutdown signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)
}
