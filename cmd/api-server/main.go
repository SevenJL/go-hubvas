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
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	natsgo "github.com/nats-io/nats.go"

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
	"github.com/hubvas/internal/interfaces/http"
	"github.com/hubvas/internal/interfaces/http/handler"
	"github.com/hubvas/internal/interfaces/http/middleware"
	"github.com/hubvas/pkg/config"
	"github.com/hubvas/pkg/idgen"
)

func configPath() string {
	if path := os.Getenv("HUBVAS_CONFIG"); path != "" {
		return path
	}
	return "configs/config.yaml"
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Hubvas API Server...")

	// ---- Configuration ----
	cfg, err := config.Load(configPath())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

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
	socialRepo := postgres.NewSocialRepo(pool)
	mediaRepo := postgres.NewMediaRepo(pool)

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

	// ---- Infrastructure: public media object storage ----
	minioClient, err := miniogo.New(cfg.Storage.Endpoint, &miniogo.Options{Creds: credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""), Secure: cfg.Storage.UseSSL})
	if err != nil {
		log.Fatalf("failed to create media storage client: %v", err)
	}
	if exists, checkErr := minioClient.BucketExists(context.Background(), cfg.Storage.MediaBucket); checkErr != nil {
		log.Printf("WARNING: cannot verify media bucket: %v", checkErr)
	} else if !exists {
		if makeErr := minioClient.MakeBucket(context.Background(), cfg.Storage.MediaBucket, miniogo.MakeBucketOptions{}); makeErr != nil {
			log.Printf("WARNING: cannot create media bucket: %v", makeErr)
		}
	}
	avatarStore := miniostore.NewAvatarStore(minioClient, cfg.Storage.MediaBucket, cfg.Storage.PublicBaseURL)

	// ---- Application: Use Cases ----
	authAppSvc := auth.NewAuthApplicationService(userRepo, jwtSvc, pwdSvc)
	socialAppSvc := appSocial.NewService(socialRepo, userRepo)
	mediaAppSvc := appMedia.NewService(mediaRepo, avatarStore, cfg.Storage.PresignTTL, cfg.Storage.AvatarMaxBytes)
	mediaCleanupCtx, cancelMediaCleanup := context.WithCancel(context.Background())
	defer cancelMediaCleanup()
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-mediaCleanupCtx.Done():
				return
			case <-ticker.C:
				if err := mediaAppSvc.CleanupExpired(mediaCleanupCtx, 100); err != nil {
					log.Printf("[media] cleanup failed: %v", err)
				}
			}
		}
	}()

	canvasAppSvc := appCanvas.NewCanvasApplicationService(
		canvasRepo,
		snapshotRepo,
		communityRepo,
		userRepo,
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
	var natsConn *natsgo.Conn
	if cfg.NATS.URL != "" {
		nc, err := natsgo.Connect(cfg.NATS.URL, natsgo.Token(cfg.NATS.Token), natsgo.RetryOnFailedConnect(true), natsgo.MaxReconnects(-1), natsgo.ReconnectWait(2*time.Second))
		if err != nil {
			log.Printf("WARNING: NATS unavailable — event bus in-process only: %v", err)
			eventBus = infnats.NewEventBus(nil)
		} else {
			natsConn = nc
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
	socialHandler := handler.NewSocialHandler(socialAppSvc)
	mediaHandler := handler.NewMediaHandler(mediaAppSvc)

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
		SocialHandler:    socialHandler,
		MediaHandler:     mediaHandler,
		WSGateway:        nil, // WS server runs in separate process
		TokenSvc:         jwtSvc,
		RateLimiter:      rateLimiter,
		UserLookup:       userRepo,
	})

	// ---- Reliable notification outbox dispatcher ----
	dispatchCtx, cancelDispatch := context.WithCancel(context.Background())
	defer cancelDispatch()
	if natsConn != nil {
		go infnats.NewNotificationDispatcher(pool, natsConn).Run(dispatchCtx)
	}

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

// snowflakeIDAdapter adapts Snowflake to the application.IDGenerator interface.
type snowflakeIDAdapter struct {
	sf *idgen.Snowflake
}

func (a *snowflakeIDAdapter) NextID() int64 {
	return a.sf.NextID()
}
