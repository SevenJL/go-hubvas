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

func configPath() string {
	if path := os.Getenv("HUBVAS_CONFIG"); path != "" {
		return path
	}
	return "configs/config.yaml"
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas WebSocket Server...")

	// ---- Configuration ----
	cfg, err := config.Load(configPath())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

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
	userRepo := postgres.NewUserRepo(pool)

	// ---- Domain Services ----
	jwtSvc := infAuth.NewJWTService(
		cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret,
		cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL,
	)
	permSvc := infAuth.NewCanvasPermissionService(canvasRepo)

	// ---- Redis (optional — distributed presence) ----
	var presenceRepo collaboration.PresenceRepository
	var lockRepo collaboration.LockRepository
	if cfg.Redis.Addr != "" {
		redisClient := goredis.NewClient(&goredis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			PoolSize: cfg.Redis.PoolSize,
		})
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Printf("WARNING: Redis unavailable — presence disabled: %v", err)
		} else {
			rp := infredis.NewPresenceRepo(redisClient)
			presenceRepo = rp
			lockRepo = rp
			log.Println("Connected to Redis (distributed presence and object locks enabled)")
		}
	} else {
		log.Println("INFO: Redis not configured — running without distributed presence")
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
	var natsConn *natsgo.Conn
	if cfg.NATS.URL != "" {
		nc, err := natsgo.Connect(cfg.NATS.URL, natsgo.Token(cfg.NATS.Token), natsgo.RetryOnFailedConnect(true), natsgo.MaxReconnects(-1), natsgo.ReconnectWait(2*time.Second))
		if err != nil {
			log.Printf("WARNING: NATS unavailable — cross-node sync disabled: %v", err)
		} else {
			natsConn = nc
			pubsub = infnats.NewPubSub(nc)
			log.Println("Connected to NATS (cross-node fan-out enabled)")
		}
	} else {
		log.Println("INFO: NATS not configured — single-node mode")
	}

	// ---- Throttle ----
	throttleSvc := throttle.NewThrottleService()
	throttleCtx, cancelThrottle := context.WithCancel(context.Background())
	defer cancelThrottle()
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-throttleCtx.Done():
				return
			case <-ticker.C:
				throttleSvc.CleanupExpired()
			}
		}
	}()

	// ---- Hub (in-memory room manager) ----
	hubOptions := make([]ws.HubOption, 0, 3)
	if pubsub != nil {
		hubOptions = append(hubOptions, ws.WithPubSub(pubsub))
	}
	if presenceRepo != nil {
		hubOptions = append(hubOptions, ws.WithPresenceRepository(presenceRepo))
	}
	if lockRepo != nil {
		hubOptions = append(hubOptions, ws.WithLockRepository(lockRepo))
	}
	hub := ws.NewHub(snapshotRepo, hubOptions...)

	// ---- WS Gateway ----
	gateway := ws.NewGateway(hub, jwtSvc, permSvc, userRepo, throttleSvc)

	// ---- HTTP Server ----
	mux := http.NewServeMux()

	// WebSocket endpoints. Notifications are user-scoped and separate from canvas collaboration.
	notificationGateway := ws.NewNotificationGateway(jwtSvc, userRepo, natsConn)
	defer notificationGateway.Close()
	mux.Handle("/ws/notifications", notificationGateway)
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
