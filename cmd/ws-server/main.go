package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hubvas/internal/domain/collaboration"
	infAuth "github.com/hubvas/internal/infrastructure/auth"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	"github.com/hubvas/internal/interfaces/ws"
	"github.com/hubvas/pkg/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas WebSocket Server...")

	// ---- Configuration ----
	cfg := loadConfig()

	// ---- Database ----
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Printf("WARNING: database ping failed: %v", err)
	} else {
		log.Println("Connected to PostgreSQL")
	}

	// ---- Repositories ----
	canvasRepo := postgres.NewCanvasRepo(pool)

	// ---- Domain Services ----
	jwtSvc := infAuth.NewJWTService(
		cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret,
		cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL,
	)
	permSvc := infAuth.NewCanvasPermissionService(canvasRepo)

	// ---- Snapshot repo (no-op memory for dev) ----
	snapshotRepo := &memorySnapshotRepo{data: make(map[int64][]byte)}

	// ---- Hub (in-memory room manager) ----
	hub := ws.NewHub(snapshotRepo)

	// ---- WS Gateway ----
	gateway := ws.NewGateway(hub, jwtSvc, permSvc)

	// ---- HTTP Server ----
	mux := http.NewServeMux()
	mux.Handle("/ws", gateway)

	addr := fmt.Sprintf("%s:%d", cfg.Server.WSHost, cfg.Server.WSPort)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("WS server listening on %s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("WS server error: %v", err)
		}
	}()

	// ---- Graceful Shutdown ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	hub.Shutdown()
}

func loadConfig() config.Config {
	return config.Config{
		Server: config.ServerConfig{
			WSHost: "0.0.0.0",
			WSPort: 8081,
		},
		Database: config.DatabaseConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     5432,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   os.Getenv("DB_NAME"),
			SSLMode:  "disable",
		},
		Auth: config.AuthConfig{
			AccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
			RefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		},
	}
}

// memorySnapshotRepo is an in-memory snapshot store for development.
type memorySnapshotRepo struct {
	data map[int64][]byte
}

func (m *memorySnapshotRepo) Save(ctx context.Context, canvasID collaboration.RoomID, data []byte) error {
	m.data[int64(canvasID)] = data
	return nil
}
func (m *memorySnapshotRepo) Load(ctx context.Context, canvasID collaboration.RoomID) ([]byte, error) {
	return m.data[int64(canvasID)], nil
}
func (m *memorySnapshotRepo) Delete(ctx context.Context, canvasID collaboration.RoomID) error {
	delete(m.data, int64(canvasID))
	return nil
}
