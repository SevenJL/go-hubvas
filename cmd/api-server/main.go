package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hubvas/internal/application/auth"
	appCanvas "github.com/hubvas/internal/application/canvas"
	appCommunity "github.com/hubvas/internal/application/community"
	infAuth "github.com/hubvas/internal/infrastructure/auth"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	"github.com/hubvas/internal/interfaces/http"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/pkg/config"
	"github.com/hubvas/pkg/idgen"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas API Server...")

	// ---- Configuration ----
	cfg := loadConfig()

	// ---- Infrastructure: Database ----
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Printf("WARNING: database ping failed: %v (server will start anyway)", err)
	} else {
		log.Println("Connected to PostgreSQL")
	}

	// ---- Infrastructure: ID Generator ----
	idGen := idgen.NewSnowflake(1) // machine ID from config in production

	// ---- Infrastructure: Repositories ----
	userRepo := postgres.NewUserRepo(pool)
	canvasRepo := postgres.NewCanvasRepo(pool)
	communityRepo := postgres.NewCommunityRepo(pool)

	// ---- Infrastructure: Domain Services ----
	jwtSvc := infAuth.NewJWTService(
		cfg.Auth.AccessSecret,
		cfg.Auth.RefreshSecret,
		cfg.Auth.AccessTokenTTL,
		cfg.Auth.RefreshTokenTTL,
	)
	pwdSvc := infAuth.NewBcryptPasswordService(cfg.Auth.BcryptCost)
	permSvc := infAuth.NewCanvasPermissionService(canvasRepo)

	// ---- Application: Use Cases ----
	authAppSvc := auth.NewAuthApplicationService(userRepo, jwtSvc, pwdSvc)

	canvasAppSvc := appCanvas.NewCanvasApplicationService(
		canvasRepo,
		&snowflakeIDAdapter{sf: idGen},
	)

	communityAppSvc := appCommunity.NewCommunityApplicationService(
		communityRepo,
		canvasRepo,
		&snowflakeIDAdapter{sf: idGen},
	)

	// ---- Interfaces: HTTP Handlers ----
	authHandler := handler.NewAuthHandler(authAppSvc)
	canvasHandler := handler.NewCanvasHandler(canvasAppSvc)
	communityHandler := handler.NewCommunityHandler(communityAppSvc)

	rateLimiter := middleware.NewRateLimiter(100, 200)

	// ---- HTTP Router ----
	gin.SetMode(gin.ReleaseMode)

	router := http.NewRouter(http.RouterConfig{
		AuthHandler:      authHandler,
		CanvasHandler:    canvasHandler,
		CommunityHandler: communityHandler,
		WSGateway:        nil, // WS server runs in separate process
		TokenSvc:         jwtSvc,
		RateLimiter:      rateLimiter,
	})

	// ---- Start Server ----
	addr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	log.Printf("API server listening on %s", addr)

	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// ---- Graceful Shutdown ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	// Prevent unused variable warnings during development.
	_ = permSvc
	_ = canvasAppSvc
	_ = communityAppSvc
}

// loadConfig reads configuration from environment variables with defaults.
func loadConfig() config.Config {
	return config.Config{
		Server: config.ServerConfig{
			APIHost:    envOrDefault("API_HOST", "0.0.0.0"),
			APIPort:    8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     5432,
			User:     envOrDefault("DB_USER", "hubvas"),
			Password: envOrDefault("DB_PASSWORD", ""),
			DBName:   envOrDefault("DB_NAME", "hubvas"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: 20,
		},
		Auth: config.AuthConfig{
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 30 * 24 * time.Hour,
			AccessSecret:    envOrDefault("JWT_ACCESS_SECRET", "dev-access-secret"),
			RefreshSecret:   envOrDefault("JWT_REFRESH_SECRET", "dev-refresh-secret"),
			BcryptCost:      12,
		},
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// snowflakeIDAdapter adapts Snowflake to the application.IDGenerator interface.
type snowflakeIDAdapter struct {
	sf *idgen.Snowflake
}

func (a *snowflakeIDAdapter) NextID() int64 {
	return a.sf.NextID()
}
