package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8787 || cfg.Paths.ClaudeLogs == "" || cfg.Paths.CodexLogs == "" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not persisted: %v", err)
	}
}

func TestValidateRejectsUnsafePort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid port to fail validation")
	}
}

func TestAddressSupportsIPv6(t *testing.T) {
	server := ServerConfig{Host: "::1", Port: 8787}
	if got := server.Address(); got != "[::1]:8787" {
		t.Fatalf("address = %q", got)
	}
}

func TestDashboardURLUsesLocalhostForWildcardBind(t *testing.T) {
	server := ServerConfig{Host: "0.0.0.0", Port: 8787}
	if got := server.DashboardURL(); got != "http://localhost:8787" {
		t.Fatalf("dashboard URL = %q", got)
	}
}

func TestDashboardURLSupportsIPv6(t *testing.T) {
	server := ServerConfig{Host: "::1", Port: 8787}
	if got := server.DashboardURL(); got != "http://[::1]:8787" {
		t.Fatalf("dashboard URL = %q", got)
	}
}
