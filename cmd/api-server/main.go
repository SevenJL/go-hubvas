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
	natsgo "github.com/nats-io/nats.go"

	"github.com/hubvas/internal/application/auth"
	appCanvas "github.com/hubvas/internal/application/canvas"
	appCommunity "github.com/hubvas/internal/application/community"
	"github.com/hubvas/internal/domain/shared"
	infAuth "github.com/hubvas/internal/infrastructure/auth"
	infnats "github.com/hubvas/internal/infrastructure/messaging/nats"
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

	// ---- Snapshot repository (needed by canvas app service) ----
	snapshotRepo := postgres.NewSnapshotStore(pool)

	// ---- Application: Use Cases ----
	authAppSvc := auth.NewAuthApplicationService(userRepo, jwtSvc, pwdSvc)

	canvasAppSvc := appCanvas.NewCanvasApplicationService(
		canvasRepo,
		snapshotRepo,
		communityRepo,
		&snowflakeIDAdapter{sf: idGen},
	)

	communityAppSvc := appCommunity.NewCommunityApplicationService(
		communityRepo,
		canvasRepo,
		userRepo,
		&snowflakeIDAdapter{sf: idGen},
	)

	// ---- Event Bus (optional — NATS for cross-service events) ----
	var eventBus *infnats.EventBus
	if cfg.NATS.URL != "" {
		nc, err := natsgo.Connect(cfg.NATS.URL)
		if err != nil {
			log.Printf("WARNING: NATS unavailable — event bus in-process only: %v", err)
			eventBus = infnats.NewEventBus(nil)
		} else {
			eventBus = infnats.NewEventBus(nc)
			log.Println("Connected to NATS (event bus with cross-service delivery)")

			// Example cross-context subscription:
			// When a canvas is published, the community context can react.
			eventBus.Subscribe("CanvasPublished", func(e shared.DomainEvent) error {
				log.Printf("[eventbus] canvas published: %s", e.EventName())
				return nil
			})
		}
	} else {
		eventBus = infnats.NewEventBus(nil)
		log.Println("INFO: NATS not configured — event bus in-process only")
	}

	// ---- Interfaces: HTTP Handlers ----
	authHandler := handler.NewAuthHandler(authAppSvc)
	canvasHandler := handler.NewCanvasHandler(canvasAppSvc)
	communityHandler := handler.NewCommunityHandler(communityAppSvc)
	healthHandler := handler.NewHealthHandler(pool)

	// Snapshot: domain interface → infrastructure impl → application service → handler
	snapshotAppSvc := appCanvas.NewSnapshotApplicationService(canvasRepo, snapshotRepo)
	snapshotHandler := handler.NewSnapshotHandler(snapshotAppSvc)

	rateLimiter := middleware.NewRateLimiter(100, 200)

	// ---- HTTP Router ----
	gin.SetMode(gin.ReleaseMode)

	router := http.NewRouter(http.RouterConfig{
		AuthHandler:      authHandler,
		CanvasHandler:    canvasHandler,
		CommunityHandler: communityHandler,
		HealthHandler:    healthHandler,
		SnapshotHandler:  snapshotHandler,
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
	_ = eventBus
}

// loadConfig reads configuration from environment variables with defaults.
func loadConfig() config.Config {
	return config.Config{
		Server: config.ServerConfig{
			APIHost:      envOrDefault("API_HOST", "0.0.0.0"),
			APIPort:      8080,
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
