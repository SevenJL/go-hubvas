package config

import (
	"fmt"
	"time"
)

// Config is the top-level application configuration.
type Config struct {
	Environment string         `yaml:"environment"`
	Server      ServerConfig   `yaml:"server"`
	Database    DatabaseConfig `yaml:"database"`
	Redis       RedisConfig    `yaml:"redis"`
	NATS        NATSConfig     `yaml:"nats"`
	Storage     StorageConfig  `yaml:"storage"`
	Auth        AuthConfig     `yaml:"auth"`
	ID          IDConfig       `yaml:"id"`
	WS          WSConfig       `yaml:"ws"`
}

func (c Config) IsProduction() bool { return c.Environment == "production" }

type ServerConfig struct {
	APIHost           string        `yaml:"api_host"`
	APIPort           int           `yaml:"api_port"`
	WSHost            string        `yaml:"ws_host"`
	WSPort            int           `yaml:"ws_port"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	TrustedProxies    []string      `yaml:"trusted_proxies"`
	MetricsToken      string        `yaml:"metrics_token"`
}

type DatabaseConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	User              string        `yaml:"user"`
	Password          string        `yaml:"password"`
	DBName            string        `yaml:"dbname"`
	SSLMode           string        `yaml:"sslmode"`
	MaxConns          int32         `yaml:"max_conns"`
	MinConns          int32         `yaml:"min_conns"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `yaml:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

type NATSConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type StorageConfig struct {
	Endpoint       string        `yaml:"endpoint"`
	AccessKey      string        `yaml:"access_key"`
	SecretKey      string        `yaml:"secret_key"`
	Bucket         string        `yaml:"bucket"`
	MediaBucket    string        `yaml:"media_bucket"`
	PublicBaseURL  string        `yaml:"public_base_url"`
	PresignTTL     time.Duration `yaml:"presign_ttl"`
	AvatarMaxBytes int64         `yaml:"avatar_max_bytes"`
	UseSSL         bool          `yaml:"use_ssl"`
}

type AuthConfig struct {
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
	AccessSecret    string        `yaml:"access_secret"`
	RefreshSecret   string        `yaml:"refresh_secret"` // retained for rolling-upgrade compatibility
	Issuer          string        `yaml:"issuer"`
	Audience        string        `yaml:"audience"`
	BcryptCost      int           `yaml:"bcrypt_cost"`
	CookieName      string        `yaml:"cookie_name"`
	CookieDomain    string        `yaml:"cookie_domain"`
	CookieSecure    bool          `yaml:"cookie_secure"`
	CookieSameSite  string        `yaml:"cookie_same_site"`
}

type IDConfig struct {
	NodeID int64 `yaml:"node_id"`
}

type WSConfig struct {
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
	GarbageInterval  time.Duration `yaml:"gc_interval"`
	SnapshotInterval time.Duration `yaml:"snapshot_interval"`
	MaxMessageSize   int64         `yaml:"max_message_size"`
	SendBufferSize   int           `yaml:"send_buffer_size"`
	InboundQueueSize int           `yaml:"inbound_queue_size"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}
