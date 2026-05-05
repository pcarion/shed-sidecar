package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
