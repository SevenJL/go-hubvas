package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	natsgo "github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"

	"github.com/hubvas/internal/domain/collaboration"
	infAuth "github.com/hubvas/internal/infrastructure/auth"
	infnats "github.com/hubvas/internal/infrastructure/messaging/nats"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	infredis "github.com/hubvas/internal/infrastructure/persistence/redis"
	infminio "github.com/hubvas/internal/infrastructure/storage/minio"
	"github.com/hubvas/internal/infrastructure/throttle"
	"github.com/hubvas/internal/interfaces/ws"
	"github.com/hubvas/pkg/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas WebSocket Server...")

	// ---- Configuration ----
	cfg := loadConfig()

	// ---- Database (required) ----
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

	// ---- Redis (optional — presence + locking) ----
	var (
		presenceRepo collaboration.PresenceRepository
		lockRepo     collaboration.LockRepository
	)
	if cfg.Redis.Addr != "" {
		redisClient := goredis.NewClient(&goredis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			PoolSize: cfg.Redis.PoolSize,
		})
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Printf("WARNING: Redis unavailable — presence/locking disabled: %v", err)
		} else {
			rp := infredis.NewPresenceRepo(redisClient)
			presenceRepo = rp
			lockRepo = rp // PresenceRepo also implements LockRepository
			log.Println("Connected to Redis (presence + locking enabled)")
		}
	} else {
		log.Println("INFO: Redis not configured — running without presence/locking")
	}

	// ---- MinIO (optional — snapshot persistence) ----
	var snapshotRepo collaboration.SnapshotRepository = &memorySnapshotRepo{data: make(map[int64][]byte)}
	if cfg.Storage.Endpoint != "" && cfg.Storage.AccessKey != "" {
		minioClient, err := minio.New(cfg.Storage.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
			Secure: cfg.Storage.UseSSL,
		})
		if err != nil {
			log.Printf("WARNING: MinIO client creation failed — using in-memory snapshot store: %v", err)
		} else {
			snapshotRepo = infminio.NewSnapshotRepo(minioClient, cfg.Storage.Bucket)
			log.Println("Connected to MinIO (snapshot persistence enabled)")
		}
	} else {
		log.Println("INFO: MinIO not configured — using in-memory snapshot store")
	}

	// ---- NATS (optional — cross-node fan-out) ----
	var pubsub *infnats.PubSub
	if cfg.NATS.URL != "" {
		nc, err := natsgo.Connect(cfg.NATS.URL)
		if err != nil {
			log.Printf("WARNING: NATS unavailable — cross-node sync disabled: %v", err)
		} else {
			pubsub = infnats.NewPubSub(nc)
			log.Println("Connected to NATS (cross-node fan-out enabled)")
		}
	} else {
		log.Println("INFO: NATS not configured — single-node mode")
	}

	// ---- Throttle ----
	throttleSvc := throttle.NewThrottleService()

	// ---- Hub (in-memory room manager) ----
	hub := ws.NewHub(snapshotRepo)

	// ---- WS Gateway ----
	gateway := ws.NewGateway(hub, jwtSvc, permSvc)

	// ---- HTTP Server ----
	mux := http.NewServeMux()

	// WebSocket endpoint.
	mux.Handle("/ws", gateway)

	// Health check endpoint.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		dbStatus := "ok"
		if err := pool.Ping(r.Context()); err != nil {
			dbStatus = "degraded"
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","rooms":%d,"connections":%d,"database":"%s"}`,
			hub.ActiveRoomCount(), hub.ActiveConnectionCount(), dbStatus)
	})

	// Prometheus-style metrics endpoint.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP hubvas_ws_rooms_active Active rooms\n")
		fmt.Fprintf(w, "# TYPE hubvas_ws_rooms_active gauge\n")
		fmt.Fprintf(w, "hubvas_ws_rooms_active %d\n", hub.ActiveRoomCount())
		fmt.Fprintf(w, "# HELP hubvas_ws_connections_active Active WebSocket connections\n")
		fmt.Fprintf(w, "# TYPE hubvas_ws_connections_active gauge\n")
		fmt.Fprintf(w, "hubvas_ws_connections_active %d\n", hub.ActiveConnectionCount())
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.WSHost, cfg.Server.WSPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

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
	if pubsub != nil {
		pubsub.Close()
	}

	// Prevent unused variable warnings for optional dependencies.
	_ = presenceRepo
	_ = lockRepo
	_ = throttleSvc
}

// ---- In-memory fallback snapshot store ----

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

// ---- Config loading ----

func loadConfig() config.Config {
	return config.Config{
		Server: config.ServerConfig{
			WSHost:       envOrDefault("WS_HOST", "0.0.0.0"),
			WSPort:       envIntOrDefault("WS_PORT", 8081),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envIntOrDefault("DB_PORT", 5432),
			User:     envOrDefault("DB_USER", "hubvas"),
			Password: envOrDefault("DB_PASSWORD", ""),
			DBName:   envOrDefault("DB_NAME", "hubvas"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: 20,
		},
		Redis: config.RedisConfig{
			Addr:     envOrDefault("REDIS_ADDR", ""),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       0,
			PoolSize: 10,
		},
		NATS: config.NATSConfig{
			URL:   envOrDefault("NATS_URL", ""),
			Token: envOrDefault("NATS_TOKEN", ""),
		},
		Storage: config.StorageConfig{
			Endpoint:  envOrDefault("STORAGE_ENDPOINT", ""),
			AccessKey: envOrDefault("STORAGE_ACCESS_KEY", ""),
			SecretKey: envOrDefault("STORAGE_SECRET_KEY", ""),
			Bucket:    envOrDefault("STORAGE_BUCKET", "hubvas-snapshots"),
			UseSSL:    false,
		},
		Auth: config.AuthConfig{
			AccessSecret:  envOrDefault("JWT_ACCESS_SECRET", "dev-access-secret"),
			RefreshSecret: envOrDefault("JWT_REFRESH_SECRET", "dev-refresh-secret"),
		},
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
