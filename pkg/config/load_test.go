package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadYAMLThenEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  api_port: 9000\ndatabase:\n  host: yaml-db\nauth:\n  access_token_ttl: 20m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_PORT", "9100")
	t.Setenv("DB_HOST", "env-db")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.APIPort != 9100 || cfg.Database.Host != "env-db" {
		t.Fatalf("environment did not override YAML: %#v", cfg)
	}
	if cfg.Auth.AccessTokenTTL != 20*time.Minute {
		t.Fatalf("YAML duration not loaded: %v", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Server.WSPort != 8081 {
		t.Fatalf("default lost: %d", cfg.Server.WSPort)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("DB_PORT", "not-a-port")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid DB_PORT error")
	}
}

func TestLoadUsesSharedDevelopmentJWTSecrets(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.AccessSecret != "dev-access-secret" || cfg.Auth.RefreshSecret != "dev-refresh-secret" {
		t.Fatalf("unexpected development secrets: %#v", cfg.Auth)
	}
}

func TestProductionRejectsDevelopmentSecretsAndMissingDependencies(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "dev-access-secret")
	if _, err := Load(""); err == nil {
		t.Fatal("expected unsafe production configuration to fail")
	}
}

func TestProductionAcceptsExplicitSecureConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("JWT_REFRESH_SECRET", "")
	t.Setenv("DB_PASSWORD", "strong-database-password")
	t.Setenv("REDIS_PASSWORD", "strong-redis-password")
	t.Setenv("NATS_TOKEN", "strong-nats-token")
	t.Setenv("STORAGE_ACCESS_KEY", "hubvas-service")
	t.Setenv("STORAGE_SECRET_KEY", "strong-storage-secret")
	t.Setenv("AUTH_COOKIE_SECURE", "true")
	t.Setenv("METRICS_TOKEN", "0123456789abcdef0123456789abcdef")
	if _, err := Load(""); err != nil {
		t.Fatalf("secure production configuration rejected: %v", err)
	}
}

func TestRejectsInsecureSameSiteNoneCookie(t *testing.T) {
	t.Setenv("AUTH_COOKIE_SAME_SITE", "none")
	t.Setenv("AUTH_COOKIE_SECURE", "false")
	if _, err := Load(""); err == nil {
		t.Fatal("expected SameSite=None without Secure to fail")
	}
}

func TestRejectsInvalidOperationalLimits(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "0")
	if _, err := Load(""); err == nil {
		t.Fatal("expected invalid database pool size to fail")
	}
}

func TestProductionRequiresProtectedMetrics(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_ACCESS_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_PASSWORD", "strong-database-password")
	t.Setenv("REDIS_PASSWORD", "strong-redis-password")
	t.Setenv("NATS_TOKEN", "strong-nats-token")
	t.Setenv("STORAGE_ACCESS_KEY", "hubvas-service")
	t.Setenv("STORAGE_SECRET_KEY", "strong-storage-secret")
	t.Setenv("AUTH_COOKIE_SECURE", "true")
	t.Setenv("METRICS_TOKEN", "short")
	if _, err := Load(""); err == nil {
		t.Fatal("expected short metrics token to be rejected")
	}
}
