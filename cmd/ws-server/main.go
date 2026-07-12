package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	if p := os.Getenv("HUBVAS_CONFIG"); p != "" {
		return p
	}
	return "configs/config.yaml"
}
func required(cfg config.Config, name string, err error) {
	if err != nil && cfg.IsProduction() {
		log.Fatalf("required dependency %s unavailable: %v", name, err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas WebSocket Server...")
	cfg, err := config.Load(configPath())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	pool, err := postgres.NewPool(rootCtx, cfg.Database)
	if err != nil {
		log.Fatalf("failed to create database pool: %v", err)
	}
	defer pool.Close()
	dbErr := pool.Ping(rootCtx)
	required(cfg, "postgres", dbErr)
	canvasRepo := postgres.NewCanvasRepo(pool)
	userRepo := postgres.NewUserRepo(pool)
	jwtSvc := infAuth.NewJWTServiceWithClaims(cfg.Auth.AccessSecret, cfg.Auth.AccessTokenTTL, cfg.Auth.Issuer, cfg.Auth.Audience)
	permSvc := infAuth.NewCanvasPermissionService(canvasRepo)

	redisClient := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB, PoolSize: cfg.Redis.PoolSize})
	defer redisClient.Close()
	redisErr := redisClient.Ping(rootCtx).Err()
	required(cfg, "redis", redisErr)
	var presenceRepo collaboration.PresenceRepository
	var lockRepo collaboration.LockRepository
	if redisErr == nil {
		rp := infredis.NewPresenceRepo(redisClient)
		presenceRepo = rp
		lockRepo = rp
	} else {
		log.Printf("WARNING: Redis unavailable; distributed presence disabled: %v", redisErr)
	}

	var minioClient *minio.Client
	var storageErr error
	var snapshotRepo collaboration.SnapshotRepository = &memorySnapshotRepo{data: make(map[int64][]byte)}
	minioClient, storageErr = minio.New(cfg.Storage.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""), Secure: cfg.Storage.UseSSL})
	if storageErr == nil {
		var exists bool
		exists, storageErr = minioClient.BucketExists(rootCtx, cfg.Storage.Bucket)
		if storageErr == nil && !exists {
			storageErr = fmt.Errorf("snapshot bucket %q does not exist", cfg.Storage.Bucket)
		}
	}
	required(cfg, "object-storage", storageErr)
	if storageErr == nil {
		snapshotRepo = infminio.NewSnapshotRepo(minioClient, cfg.Storage.Bucket)
	} else {
		log.Printf("WARNING: object storage unavailable; using development memory snapshots: %v", storageErr)
	}

	var natsConn *natsgo.Conn
	var pubsub *infnats.PubSub
	natsConn, err = infnats.Connect(cfg.NATS.URL, cfg.NATS.Token)
	required(cfg, "nats", err)
	if err == nil {
		defer natsConn.Drain()
		pubsub = infnats.NewPubSub(natsConn)
	} else {
		natsConn = nil
		log.Printf("WARNING: NATS unavailable; cross-node sync disabled: %v", err)
	}

	throttleSvc := throttle.NewThrottleService()
	throttleCtx, cancelThrottle := context.WithCancel(rootCtx)
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
	opts := []ws.HubOption{}
	if pubsub != nil {
		opts = append(opts, ws.WithPubSub(pubsub))
	}
	if presenceRepo != nil {
		opts = append(opts, ws.WithPresenceRepository(presenceRepo))
	}
	if lockRepo != nil {
		opts = append(opts, ws.WithLockRepository(lockRepo))
	}
	hub := ws.NewHub(snapshotRepo, opts...)
	gateway := ws.NewGateway(hub, jwtSvc, permSvc, userRepo, throttleSvc)
	notificationGateway := ws.NewNotificationGateway(jwtSvc, userRepo, natsConn)
	defer notificationGateway.Close()
	mux := http.NewServeMux()
	mux.Handle("/ws/notifications", notificationGateway)
	mux.Handle("/ws", gateway)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","rooms":%d,"connections":%d}`, hub.ActiveRoomCount(), hub.ActiveConnectionCount())
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		ready := pool.Ping(ctx) == nil && redisClient.Ping(ctx).Err() == nil && natsConn != nil && natsConn.IsConnected()
		if minioClient == nil {
			ready = false
		} else {
			exists, e := minioClient.BucketExists(ctx, cfg.Storage.Bucket)
			ready = ready && e == nil && exists
		}
		w.Header().Set("Content-Type", "application/json")
		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"status":"not ready"}`)
			return
		}
		fmt.Fprint(w, `{"status":"ready"}`)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP hubvas_ws_rooms_active Active rooms\n# TYPE hubvas_ws_rooms_active gauge\nhubvas_ws_rooms_active %d\n# HELP hubvas_ws_connections_active Active WebSocket connections\n# TYPE hubvas_ws_connections_active gauge\nhubvas_ws_connections_active %d\n", hub.ActiveRoomCount(), hub.ActiveConnectionCount())
	})
	addr := fmt.Sprintf("%s:%d", cfg.Server.WSHost, cfg.Server.WSPort)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout}
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("WS server listening on %s", addr)
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			serverErr <- e
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Printf("Received signal %v, shutting down...", sig)
	case e := <-serverErr:
		log.Printf("server error: %v", e)
	}
	cancelThrottle()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
	hub.Shutdown()
	if pubsub != nil {
		pubsub.Close()
	}
}

type memorySnapshotRepo struct {
	mu   sync.RWMutex
	data map[int64][]byte
}

func (m *memorySnapshotRepo) Save(_ context.Context, id collaboration.RoomID, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[int64(id)] = append([]byte(nil), data...)
	return nil
}
func (m *memorySnapshotRepo) Load(_ context.Context, id collaboration.RoomID) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]byte(nil), m.data[int64(id)]...), nil
}
func (m *memorySnapshotRepo) Delete(_ context.Context, id collaboration.RoomID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, int64(id))
	return nil
}
