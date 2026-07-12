package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

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
	if !cfg.IsProduction() {
		if cfg.Auth.AccessSecret == "" {
			cfg.Auth.AccessSecret = "dev-access-secret"
		}
		if cfg.Auth.RefreshSecret == "" {
			cfg.Auth.RefreshSecret = "dev-refresh-secret"
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Defaults() Config {
	return Config{
		Environment: "development",
		Server:      ServerConfig{APIHost: "0.0.0.0", APIPort: 8080, WSHost: "0.0.0.0", WSPort: 8081, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute, ShutdownTimeout: 15 * time.Second, TrustedProxies: []string{"127.0.0.1/8", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}},
		Database:    DatabaseConfig{Host: "localhost", Port: 5432, User: "hubvas", DBName: "hubvas", SSLMode: "disable", MaxConns: 20, MinConns: 2, MaxConnLifetime: time.Hour, MaxConnIdleTime: 30 * time.Minute, HealthCheckPeriod: time.Minute},
		Redis:       RedisConfig{Addr: "localhost:6379", DB: 0, PoolSize: 10},
		NATS:        NATSConfig{URL: "nats://localhost:4222"},
		Storage:     StorageConfig{Endpoint: "localhost:9000", Bucket: "hubvas-snapshots", MediaBucket: "hubvas-media", PresignTTL: 15 * time.Minute, AvatarMaxBytes: 5 * 1024 * 1024},
		Auth:        AuthConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, AccessSecret: "dev-access-secret", RefreshSecret: "dev-refresh-secret", Issuer: "hubvas", Audience: "hubvas-web", BcryptCost: 12, CookieName: "hubvas_refresh", CookieSameSite: "strict"},
		ID:          IDConfig{NodeID: 1},
		WS:          WSConfig{IdleTimeout: 5 * time.Minute, GarbageInterval: time.Minute, SnapshotInterval: 30 * time.Second, MaxMessageSize: 512 * 1024, SendBufferSize: 256, InboundQueueSize: 1024},
	}
}

func (c Config) Validate() error {
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if c.ID.NodeID < 0 || c.ID.NodeID > 1023 {
		return fmt.Errorf("SNOWFLAKE_NODE_ID must be between 0 and 1023")
	}
	if c.Server.APIPort < 1 || c.Server.APIPort > 65535 || c.Server.WSPort < 1 || c.Server.WSPort > 65535 {
		return fmt.Errorf("server ports must be between 1 and 65535")
	}
	if c.Server.ReadHeaderTimeout <= 0 || c.Server.ReadTimeout <= 0 || c.Server.WriteTimeout <= 0 || c.Server.IdleTimeout <= 0 || c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server timeouts must be positive")
	}
	for _, proxy := range c.Server.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("invalid trusted proxy %q", proxy)
			}
		}
	}
	if c.Database.Host == "" || c.Database.User == "" || c.Database.DBName == "" || c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("database host, port, user, and name must be configured")
	}
	if c.Database.MaxConns <= 0 || c.Database.MinConns < 0 || c.Database.MinConns > c.Database.MaxConns {
		return fmt.Errorf("invalid database connection pool limits")
	}
	if c.Database.MaxConnLifetime <= 0 || c.Database.MaxConnIdleTime <= 0 || c.Database.HealthCheckPeriod <= 0 {
		return fmt.Errorf("database pool durations must be positive")
	}
	if c.Redis.Addr == "" || c.Redis.DB < 0 || c.Redis.PoolSize <= 0 {
		return fmt.Errorf("invalid Redis configuration")
	}
	if c.Storage.Endpoint == "" || c.Storage.Bucket == "" || c.Storage.MediaBucket == "" || c.Storage.PresignTTL <= 0 || c.Storage.AvatarMaxBytes <= 0 {
		return fmt.Errorf("invalid object-storage configuration")
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.RefreshTokenTTL <= 0 {
		return fmt.Errorf("token TTLs must be positive")
	}
	if c.Auth.Issuer == "" || c.Auth.Audience == "" || c.Auth.CookieName == "" || c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 31 {
		return fmt.Errorf("invalid authentication configuration")
	}
	sameSite := strings.ToLower(c.Auth.CookieSameSite)
	if sameSite != "strict" && sameSite != "lax" && sameSite != "none" {
		return fmt.Errorf("AUTH_COOKIE_SAME_SITE must be strict, lax, or none")
	}
	if sameSite == "none" && !c.Auth.CookieSecure {
		return fmt.Errorf("SameSite=None refresh cookies require AUTH_COOKIE_SECURE=true")
	}
	if c.WS.IdleTimeout <= 0 || c.WS.GarbageInterval <= 0 || c.WS.SnapshotInterval <= 0 || c.WS.MaxMessageSize <= 0 || c.WS.SendBufferSize <= 0 || c.WS.InboundQueueSize <= 0 {
		return fmt.Errorf("invalid WebSocket configuration")
	}
	if c.IsProduction() {
		var missing []string
		if len(c.Auth.AccessSecret) < 32 || strings.Contains(c.Auth.AccessSecret, "dev-") {
			missing = append(missing, "JWT_ACCESS_SECRET (minimum 32 non-development characters)")
		}
		if c.Auth.RefreshSecret != "" && (len(c.Auth.RefreshSecret) < 32 || strings.Contains(c.Auth.RefreshSecret, "dev-")) {
			missing = append(missing, "JWT_REFRESH_SECRET (remove it or use a strong rolling-upgrade value)")
		}
		if c.Database.Password == "" || c.Database.Password == "hubvas" {
			missing = append(missing, "DB_PASSWORD")
		}
		if c.Redis.Addr == "" || c.Redis.Password == "" {
			missing = append(missing, "REDIS_ADDR/REDIS_PASSWORD")
		}
		if c.NATS.URL == "" || c.NATS.Token == "" {
			missing = append(missing, "NATS_URL/NATS_TOKEN")
		}
		if c.Storage.Endpoint == "" || c.Storage.AccessKey == "" || c.Storage.SecretKey == "" || c.Storage.AccessKey == "minioadmin" || c.Storage.SecretKey == "minioadmin" {
			missing = append(missing, "production object-storage credentials")
		}
		if !c.Auth.CookieSecure {
			missing = append(missing, "AUTH_COOKIE_SECURE=true")
		}
		if len(c.Server.MetricsToken) < 32 {
			missing = append(missing, "METRICS_TOKEN (minimum 32 characters)")
		}
		if len(missing) > 0 {
			return fmt.Errorf("unsafe production configuration: %s", strings.Join(missing, ", "))
		}
	}
	return nil
}

func applyEnvironment(cfg *Config) error {
	stringsMap := map[string]*string{
		"APP_ENV": &cfg.Environment, "API_HOST": &cfg.Server.APIHost, "WS_HOST": &cfg.Server.WSHost, "METRICS_TOKEN": &cfg.Server.MetricsToken,
		"DB_HOST": &cfg.Database.Host, "DB_USER": &cfg.Database.User, "DB_PASSWORD": &cfg.Database.Password, "DB_NAME": &cfg.Database.DBName, "DB_SSLMODE": &cfg.Database.SSLMode,
		"REDIS_ADDR": &cfg.Redis.Addr, "REDIS_PASSWORD": &cfg.Redis.Password, "NATS_URL": &cfg.NATS.URL, "NATS_TOKEN": &cfg.NATS.Token,
		"STORAGE_ENDPOINT": &cfg.Storage.Endpoint, "STORAGE_ACCESS_KEY": &cfg.Storage.AccessKey, "STORAGE_SECRET_KEY": &cfg.Storage.SecretKey, "STORAGE_BUCKET": &cfg.Storage.Bucket, "STORAGE_MEDIA_BUCKET": &cfg.Storage.MediaBucket, "STORAGE_PUBLIC_BASE_URL": &cfg.Storage.PublicBaseURL,
		"JWT_ACCESS_SECRET": &cfg.Auth.AccessSecret, "JWT_REFRESH_SECRET": &cfg.Auth.RefreshSecret, "JWT_ISSUER": &cfg.Auth.Issuer, "JWT_AUDIENCE": &cfg.Auth.Audience,
		"AUTH_COOKIE_NAME": &cfg.Auth.CookieName, "AUTH_COOKIE_DOMAIN": &cfg.Auth.CookieDomain, "AUTH_COOKIE_SAME_SITE": &cfg.Auth.CookieSameSite,
	}
	for k, p := range stringsMap {
		if v, ok := os.LookupEnv(k); ok {
			*p = v
		}
	}
	if value, ok := os.LookupEnv("TRUSTED_PROXIES"); ok {
		cfg.Server.TrustedProxies = nil
		for _, proxy := range strings.Split(value, ",") {
			if proxy = strings.TrimSpace(proxy); proxy != "" {
				cfg.Server.TrustedProxies = append(cfg.Server.TrustedProxies, proxy)
			}
		}
	}
	ints := map[string]*int{"API_PORT": &cfg.Server.APIPort, "WS_PORT": &cfg.Server.WSPort, "DB_PORT": &cfg.Database.Port, "REDIS_DB": &cfg.Redis.DB, "REDIS_POOL_SIZE": &cfg.Redis.PoolSize, "BCRYPT_COST": &cfg.Auth.BcryptCost, "WS_SEND_BUFFER_SIZE": &cfg.WS.SendBufferSize, "WS_INBOUND_QUEUE_SIZE": &cfg.WS.InboundQueueSize}
	for k, p := range ints {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.Atoi(v)
			if e != nil {
				return fmt.Errorf("invalid %s: %w", k, e)
			}
			*p = n
		}
	}
	int32s := map[string]*int32{"DB_MAX_CONNS": &cfg.Database.MaxConns, "DB_MIN_CONNS": &cfg.Database.MinConns}
	for k, p := range int32s {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.ParseInt(v, 10, 32)
			if e != nil {
				return fmt.Errorf("invalid %s: %w", k, e)
			}
			*p = int32(n)
		}
	}
	if v, ok := os.LookupEnv("SNOWFLAKE_NODE_ID"); ok {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return fmt.Errorf("invalid SNOWFLAKE_NODE_ID: %w", e)
		}
		cfg.ID.NodeID = n
	}
	bools := map[string]*bool{"STORAGE_USE_SSL": &cfg.Storage.UseSSL, "AUTH_COOKIE_SECURE": &cfg.Auth.CookieSecure}
	for k, p := range bools {
		if v, ok := os.LookupEnv(k); ok {
			b, e := strconv.ParseBool(v)
			if e != nil {
				return fmt.Errorf("invalid %s: %w", k, e)
			}
			*p = b
		}
	}
	if v, ok := os.LookupEnv("STORAGE_AVATAR_MAX_BYTES"); ok {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return fmt.Errorf("invalid STORAGE_AVATAR_MAX_BYTES: %w", e)
		}
		cfg.Storage.AvatarMaxBytes = n
	}
	if v, ok := os.LookupEnv("WS_MAX_MESSAGE_SIZE"); ok {
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return fmt.Errorf("invalid WS_MAX_MESSAGE_SIZE: %w", e)
		}
		cfg.WS.MaxMessageSize = n
	}
	durations := map[string]*time.Duration{"SERVER_READ_HEADER_TIMEOUT": &cfg.Server.ReadHeaderTimeout, "SERVER_READ_TIMEOUT": &cfg.Server.ReadTimeout, "SERVER_WRITE_TIMEOUT": &cfg.Server.WriteTimeout, "SERVER_IDLE_TIMEOUT": &cfg.Server.IdleTimeout, "SERVER_SHUTDOWN_TIMEOUT": &cfg.Server.ShutdownTimeout, "DB_MAX_CONN_LIFETIME": &cfg.Database.MaxConnLifetime, "DB_MAX_CONN_IDLE_TIME": &cfg.Database.MaxConnIdleTime, "DB_HEALTH_CHECK_PERIOD": &cfg.Database.HealthCheckPeriod, "JWT_ACCESS_TOKEN_TTL": &cfg.Auth.AccessTokenTTL, "JWT_REFRESH_TOKEN_TTL": &cfg.Auth.RefreshTokenTTL, "STORAGE_PRESIGN_TTL": &cfg.Storage.PresignTTL, "WS_IDLE_TIMEOUT": &cfg.WS.IdleTimeout, "WS_GC_INTERVAL": &cfg.WS.GarbageInterval, "WS_SNAPSHOT_INTERVAL": &cfg.WS.SnapshotInterval}
	for k, p := range durations {
		if v, ok := os.LookupEnv(k); ok {
			d, e := time.ParseDuration(v)
			if e != nil {
				return fmt.Errorf("invalid %s: %w", k, e)
			}
			*p = d
		}
	}
	return nil
}
