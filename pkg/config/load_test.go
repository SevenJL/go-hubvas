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
