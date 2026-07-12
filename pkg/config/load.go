package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
)

// Load reads the YAML configuration and then applies environment overrides.
// A missing file is allowed so containers can be configured exclusively by env.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, fmt.Errorf("parse config %q: %w", path, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read config %q: %w", path, err)
		}
	}
	if err := applyEnvironment(&cfg); err != nil {
		return Config{}, err
	}
	// Both processes must share JWT secrets. Stable development fallbacks avoid
	// API-issued tokens being rejected by the separately running WS process.
	if cfg.Auth.AccessSecret == "" {
		cfg.Auth.AccessSecret = "dev-access-secret"
	}
	if cfg.Auth.RefreshSecret == "" {
		cfg.Auth.RefreshSecret = "dev-refresh-secret"
	}
	return cfg, nil
}

// Defaults returns the shared defaults used by both API and WS processes.
func Defaults() Config {
	return Config{
		Server:   ServerConfig{APIHost: "0.0.0.0", APIPort: 8080, WSHost: "0.0.0.0", WSPort: 8081, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second},
		Database: DatabaseConfig{Host: "localhost", Port: 5432, User: "hubvas", DBName: "hubvas", SSLMode: "disable", MaxConns: 20},
		Redis:    RedisConfig{Addr: "localhost:6379", DB: 0, PoolSize: 10},
		NATS:     NATSConfig{URL: "nats://localhost:4222"},
		Storage:  StorageConfig{Endpoint: "localhost:9000", Bucket: "hubvas-snapshots", MediaBucket: "hubvas-media", PresignTTL: 15 * time.Minute, AvatarMaxBytes: 5 * 1024 * 1024},
		Auth:     AuthConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, AccessSecret: "dev-access-secret", RefreshSecret: "dev-refresh-secret", BcryptCost: 12},
		WS:       WSConfig{IdleTimeout: 5 * time.Minute, GarbageInterval: time.Minute, SnapshotInterval: 30 * time.Second, MaxMessageSize: 512 * 1024, SendBufferSize: 256, InboundQueueSize: 1024},
	}
}

func applyEnvironment(cfg *Config) error {
	stringOverrides := map[string]*string{
		"API_HOST": &cfg.Server.APIHost, "WS_HOST": &cfg.Server.WSHost,
		"DB_HOST": &cfg.Database.Host, "DB_USER": &cfg.Database.User, "DB_PASSWORD": &cfg.Database.Password, "DB_NAME": &cfg.Database.DBName, "DB_SSLMODE": &cfg.Database.SSLMode,
		"REDIS_ADDR": &cfg.Redis.Addr, "REDIS_PASSWORD": &cfg.Redis.Password,
		"NATS_URL": &cfg.NATS.URL, "NATS_TOKEN": &cfg.NATS.Token,
		"STORAGE_ENDPOINT": &cfg.Storage.Endpoint, "STORAGE_ACCESS_KEY": &cfg.Storage.AccessKey, "STORAGE_SECRET_KEY": &cfg.Storage.SecretKey, "STORAGE_BUCKET": &cfg.Storage.Bucket, "STORAGE_MEDIA_BUCKET": &cfg.Storage.MediaBucket, "STORAGE_PUBLIC_BASE_URL": &cfg.Storage.PublicBaseURL,
		"JWT_ACCESS_SECRET": &cfg.Auth.AccessSecret, "JWT_REFRESH_SECRET": &cfg.Auth.RefreshSecret,
	}
	for key, target := range stringOverrides {
		if value, ok := os.LookupEnv(key); ok {
			*target = value
		}
	}

	intOverrides := map[string]*int{
		"API_PORT": &cfg.Server.APIPort, "WS_PORT": &cfg.Server.WSPort,
		"DB_PORT": &cfg.Database.Port, "DB_MAX_CONNS": &cfg.Database.MaxConns,
		"REDIS_DB": &cfg.Redis.DB, "REDIS_POOL_SIZE": &cfg.Redis.PoolSize,
		"BCRYPT_COST": &cfg.Auth.BcryptCost, "WS_SEND_BUFFER_SIZE": &cfg.WS.SendBufferSize, "WS_INBOUND_QUEUE_SIZE": &cfg.WS.InboundQueueSize,
	}
	for key, target := range intOverrides {
		if value, ok := os.LookupEnv(key); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", key, err)
			}
			*target = parsed
		}
	}

	if value, ok := os.LookupEnv("STORAGE_USE_SSL"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid STORAGE_USE_SSL: %w", err)
		}
		cfg.Storage.UseSSL = parsed
	}
	if value, ok := os.LookupEnv("STORAGE_AVATAR_MAX_BYTES"); ok {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid STORAGE_AVATAR_MAX_BYTES: %w", err)
		}
		cfg.Storage.AvatarMaxBytes = parsed
	}
	if value, ok := os.LookupEnv("WS_MAX_MESSAGE_SIZE"); ok {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid WS_MAX_MESSAGE_SIZE: %w", err)
		}
		cfg.WS.MaxMessageSize = parsed
	}

	durations := map[string]*time.Duration{
		"SERVER_READ_TIMEOUT": &cfg.Server.ReadTimeout, "SERVER_WRITE_TIMEOUT": &cfg.Server.WriteTimeout,
		"JWT_ACCESS_TOKEN_TTL": &cfg.Auth.AccessTokenTTL, "JWT_REFRESH_TOKEN_TTL": &cfg.Auth.RefreshTokenTTL,
		"STORAGE_PRESIGN_TTL": &cfg.Storage.PresignTTL, "WS_IDLE_TIMEOUT": &cfg.WS.IdleTimeout, "WS_GC_INTERVAL": &cfg.WS.GarbageInterval, "WS_SNAPSHOT_INTERVAL": &cfg.WS.SnapshotInterval,
	}
	for key, target := range durations {
		if value, ok := os.LookupEnv(key); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", key, err)
			}
			*target = parsed
		}
	}
	return nil
}
