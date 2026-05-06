package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPort(t *testing.T) {
	if got, want := Default().Port, 50051; got != want {
		t.Fatalf("Default().Port = %d, want %d", got, want)
	}
	if got, want := Default().TCPAddress(), "127.0.0.1:50051"; got != want {
		t.Fatalf("Default().TCPAddress() = %q, want %q", got, want)
	}
}

func TestDefaultNetworkPortRange(t *testing.T) {
	if got, want := Default().NetworkPortMin, 20000; got != want {
		t.Fatalf("Default().NetworkPortMin = %d, want %d", got, want)
	}
	if got, want := Default().NetworkPortMax, 29999; got != want {
		t.Fatalf("Default().NetworkPortMax = %d, want %d", got, want)
	}
}

func TestLoadResolvesRelativeDatabasePathAgainstConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`database_path = "data/sidecar.db"`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join(dir, "data", "sidecar.db")
	if cfg.DatabasePath != want {
		t.Fatalf("DatabasePath = %q, want %q", cfg.DatabasePath, want)
	}
	if cfg.ConfigDir != dir {
		t.Fatalf("ConfigDir = %q, want %q", cfg.ConfigDir, dir)
	}
}

func TestLoadResolvesDefaultDatabasePathWhenConfigIsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := filepath.Join(dir, "sidecar.db")
	if cfg.DatabasePath != want {
		t.Fatalf("DatabasePath = %q, want %q", cfg.DatabasePath, want)
	}
}

func TestLoadRejectsInvalidNetworkPortRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("network_port_min = 30000\nnetwork_port_max = 20000\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil error for invalid network port range")
	}
}
