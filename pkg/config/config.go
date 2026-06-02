package config

import (
	"time"
)

// Config is the top-level application configuration.
// It maps to config.yaml and environment variable overrides.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	NATS     NATSConfig     `yaml:"nats"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	WS       WSConfig       `yaml:"ws"`
}

// ServerConfig holds HTTP and WS server settings.
type ServerConfig struct {
	APIHost    string `yaml:"api_host" env:"API_HOST" default:"0.0.0.0"`
	APIPort    int    `yaml:"api_port" env:"API_PORT" default:"8080"`
	WSHost     string `yaml:"ws_host" env:"WS_HOST" default:"0.0.0.0"`
	WSPort     int    `yaml:"ws_port" env:"WS_PORT" default:"8081"`
	ReadTimeout  time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout time.Duration `yaml:"write_timeout" default:"30s"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string `yaml:"host" env:"DB_HOST" default:"localhost"`
	Port     int    `yaml:"port" env:"DB_PORT" default:"5432"`
	User     string `yaml:"user" env:"DB_USER" default:"hubvas"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	DBName   string `yaml:"dbname" env:"DB_NAME" default:"hubvas"`
	SSLMode  string `yaml:"sslmode" default:"disable"`
	MaxConns int    `yaml:"max_conns" default:"20"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `yaml:"addr" env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `yaml:"password" env:"REDIS_PASSWORD"`
	DB       int    `yaml:"db" default:"0"`
	PoolSize int    `yaml:"pool_size" default:"10"`
}

// NATSConfig holds NATS connection settings.
type NATSConfig struct {
	URL   string `yaml:"url" env:"NATS_URL" default:"nats://localhost:4222"`
	Token string `yaml:"token" env:"NATS_TOKEN"`
}

// StorageConfig holds object storage (MinIO/S3) settings.
type StorageConfig struct {
	Endpoint  string `yaml:"endpoint" env:"STORAGE_ENDPOINT" default:"localhost:9000"`
	AccessKey string `yaml:"access_key" env:"STORAGE_ACCESS_KEY"`
	SecretKey string `yaml:"secret_key" env:"STORAGE_SECRET_KEY"`
	Bucket    string `yaml:"bucket" default:"hubvas-snapshots"`
	UseSSL    bool   `yaml:"use_ssl" default:"false"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl" default:"15m"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl" default:"720h"` // 30 days
	AccessSecret    string        `yaml:"access_secret" env:"JWT_ACCESS_SECRET"`
	RefreshSecret   string        `yaml:"refresh_secret" env:"JWT_REFRESH_SECRET"`
	BcryptCost      int           `yaml:"bcrypt_cost" default:"12"`
}

// WSConfig holds WebSocket-specific settings.
type WSConfig struct {
	IdleTimeout      time.Duration `yaml:"idle_timeout" default:"5m"`
	GarbageInterval  time.Duration `yaml:"gc_interval" default:"1m"`
	SnapshotInterval time.Duration `yaml:"snapshot_interval" default:"30s"`
	MaxMessageSize   int64         `yaml:"max_message_size" default:"524288"` // 512 KB
	SendBufferSize   int           `yaml:"send_buffer_size" default:"256"`
	InboundQueueSize int           `yaml:"inbound_queue_size" default:"1024"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + itoa(d.Port) +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.DBName +
		" sslmode=" + d.SSLMode
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
