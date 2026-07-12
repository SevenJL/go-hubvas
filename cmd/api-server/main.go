package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	natsgo "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/hubvas/internal/application/auth"
	appCanvas "github.com/hubvas/internal/application/canvas"
	appCommunity "github.com/hubvas/internal/application/community"
	appMedia "github.com/hubvas/internal/application/media"
	appSocial "github.com/hubvas/internal/application/social"
	"github.com/hubvas/internal/domain/shared"
	infAuth "github.com/hubvas/internal/infrastructure/auth"
	infnats "github.com/hubvas/internal/infrastructure/messaging/nats"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	miniostore "github.com/hubvas/internal/infrastructure/storage/minio"
	httpapi "github.com/hubvas/internal/interfaces/http"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/pkg/config"
	"github.com/hubvas/pkg/idgen"
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
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas API Server...")
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
	if err = pool.Ping(rootCtx); err != nil {
		required(cfg, "postgres", err)
		log.Printf("WARNING: database unavailable: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB, PoolSize: cfg.Redis.PoolSize})
	defer rdb.Close()
	redisErr := rdb.Ping(rootCtx).Err()
	required(cfg, "redis", redisErr)
	if redisErr != nil {
		log.Printf("WARNING: Redis unavailable; using process-local rate limiting: %v", redisErr)
	}

	minioClient, err := miniogo.New(cfg.Storage.Endpoint, &miniogo.Options{Creds: credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""), Secure: cfg.Storage.UseSSL})
	if err != nil {
		log.Fatalf("failed to create media storage client: %v", err)
	}
	mediaBucketExists, storageErr := minioClient.BucketExists(rootCtx, cfg.Storage.MediaBucket)
	if storageErr == nil && !mediaBucketExists {
		storageErr = fmt.Errorf("media bucket %q does not exist", cfg.Storage.MediaBucket)
	}
	required(cfg, "object-storage", storageErr)
	if storageErr != nil {
		log.Printf("WARNING: media bucket check failed: %v", storageErr)
	}
	avatarStore := miniostore.NewAvatarStore(minioClient, cfg.Storage.MediaBucket, cfg.Storage.PublicBaseURL)

	var natsConn *natsgo.Conn
	if cfg.NATS.URL != "" {
		natsConn, err = infnats.Connect(cfg.NATS.URL, cfg.NATS.Token)
		required(cfg, "nats", err)
		if err != nil {
			log.Printf("WARNING: NATS unavailable: %v", err)
			natsConn = nil
		} else {
			defer natsConn.Drain()
		}
	}
	eventBus := infnats.NewEventBus(natsConn)
	eventBus.Subscribe("CanvasPublished", func(e shared.DomainEvent) error {
		log.Printf("[eventbus] canvas published: %s", e.EventName())
		return nil
	})

	idGen := idgen.NewSnowflake(cfg.ID.NodeID)
	userRepo := postgres.NewUserRepo(pool)
	canvasRepo := postgres.NewCanvasRepo(pool)
	communityRepo := postgres.NewCommunityRepo(pool)
	socialRepo := postgres.NewSocialRepo(pool)
	mediaRepo := postgres.NewMediaRepo(pool)
	sessionRepo := postgres.NewAuthSessionRepo(pool)
	accountRepo := postgres.NewAccountRepo(pool)
	snapshotRepo := postgres.NewSnapshotStore(pool)
	jwtSvc := infAuth.NewJWTServiceWithClaims(cfg.Auth.AccessSecret, cfg.Auth.AccessTokenTTL, cfg.Auth.Issuer, cfg.Auth.Audience)
	pwdSvc := infAuth.NewBcryptPasswordService(cfg.Auth.BcryptCost)
	permSvc := infAuth.NewCanvasPermissionService(canvasRepo)
	authAppSvc := auth.NewAuthApplicationService(userRepo, sessionRepo, accountRepo, jwtSvc, pwdSvc, cfg.Auth.RefreshTokenTTL)
	socialAppSvc := appSocial.NewService(socialRepo, userRepo)
	mediaAppSvc := appMedia.NewService(mediaRepo, avatarStore, cfg.Storage.PresignTTL, cfg.Storage.AvatarMaxBytes)
	canvasAppSvc := appCanvas.NewCanvasApplicationService(canvasRepo, snapshotRepo, communityRepo, userRepo, &snowflakeIDAdapter{sf: idGen})
	communityAppSvc := appCommunity.NewCommunityApplicationService(communityRepo, canvasRepo, userRepo, &snowflakeIDAdapter{sf: idGen})

	mediaCtx, cancelMedia := context.WithCancel(rootCtx)
	defer cancelMedia()
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-mediaCtx.Done():
				return
			case <-ticker.C:
				if err := mediaAppSvc.CleanupExpired(mediaCtx, 100); err != nil {
					log.Printf("[media] cleanup failed: %v", err)
				}
				if _, err := sessionRepo.CleanupExpired(mediaCtx, time.Now().UTC(), 1000); err != nil {
					log.Printf("[auth] session cleanup failed: %v", err)
				}
				if _, err := pool.Exec(mediaCtx, `DELETE FROM idempotency_keys WHERE (user_id,scope,idempotency_key) IN (SELECT user_id,scope,idempotency_key FROM idempotency_keys WHERE expires_at<now() ORDER BY expires_at LIMIT 1000)`); err != nil {
					log.Printf("[idempotency] cleanup failed: %v", err)
				}
				if _, err := pool.Exec(mediaCtx, `DELETE FROM notification_outbox WHERE published_at<now()-interval '7 days'`); err != nil {
					log.Printf("[notifications] published outbox cleanup failed: %v", err)
				}
			}
		}
	}()
	dispatchCtx, cancelDispatch := context.WithCancel(rootCtx)
	defer cancelDispatch()
	if natsConn != nil {
		go infnats.NewNotificationDispatcher(pool, natsConn).Run(dispatchCtx)
	}

	checks := []handler.DependencyCheck{{Name: "redis", Check: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }}, {Name: "object_storage", Check: func(ctx context.Context) error {
		exists, e := minioClient.BucketExists(ctx, cfg.Storage.MediaBucket)
		if e == nil && !exists {
			return fmt.Errorf("media bucket %q does not exist", cfg.Storage.MediaBucket)
		}
		return e
	}}}
	if natsConn != nil || cfg.IsProduction() {
		checks = append(checks, handler.DependencyCheck{Name: "nats", Check: func(ctx context.Context) error {
			if natsConn == nil || !natsConn.IsConnected() {
				return fmt.Errorf("not connected")
			}
			return natsConn.FlushWithContext(ctx)
		}})
	}
	authHandler := handler.NewAuthHandler(authAppSvc, cfg.Auth)
	canvasHandler := handler.NewCanvasHandler(canvasAppSvc)
	communityHandler := handler.NewCommunityHandler(communityAppSvc)
	healthHandler := handler.NewHealthHandler(pool, checks...)
	socialHandler := handler.NewSocialHandler(socialAppSvc)
	mediaHandler := handler.NewMediaHandler(mediaAppSvc)
	snapshotHandler := handler.NewSnapshotHandler(appCanvas.NewSnapshotApplicationService(canvasRepo, snapshotRepo))
	var rateLimiter *middleware.RateLimiter
	if redisErr == nil {
		rateLimiter = middleware.NewRedisRateLimiter(rdb, 100, 200)
	} else {
		rateLimiter = middleware.NewRateLimiter(100, 200)
	}
	gin.SetMode(gin.ReleaseMode)
	router := httpapi.NewRouter(httpapi.RouterConfig{AuthHandler: authHandler, CanvasHandler: canvasHandler, CommunityHandler: communityHandler, HealthHandler: healthHandler, SnapshotHandler: snapshotHandler, SocialHandler: socialHandler, MediaHandler: mediaHandler, TokenSvc: jwtSvc, RateLimiter: rateLimiter, UserLookup: userRepo, TrustedProxies: cfg.Server.TrustedProxies, DBPool: pool})

	addr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	srv := &http.Server{Addr: addr, Handler: router, ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout}
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("API server listening on %s", addr)
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
	cancelDispatch()
	cancelMedia()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown incomplete: %v", err)
	}
	_ = permSvc
	_ = eventBus
}

type snowflakeIDAdapter struct{ sf *idgen.Snowflake }

func (a *snowflakeIDAdapter) NextID() int64 { return a.sf.NextID() }
