package pghba

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

func TestConfigureReturnsExistingRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("# comment\nhost all app 10.0.0.0/24 scram-sha-256\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigurePgHbaConfRequest{
		FilePath: path,
		Type:     sidecarv1.PgHbaType_PG_HBA_TYPE_HOST,
		Database: "all",
		Users:    []string{"app"},
		Address:  ptr("10.0.0.0/24"),
		Method:   "scram-sha-256",
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if isNew {
		t.Fatal("Configure marked existing rule as new")
	}
	if _, err := os.Stat(filepath.Join(dir, "archive")); !os.IsNotExist(err) {
		t.Fatalf("archive dir exists for unchanged file: %v", err)
	}
}

func TestConfigureBacksUpAndAppendsMissingRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pg_hba.conf")
	original := []byte("local all postgres peer")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigurePgHbaConfRequest{
		FilePath: path,
		Type:     sidecarv1.PgHbaType_PG_HBA_TYPE_HOST,
		Database: "appdb",
		Users:    []string{"app", "migrator"},
		Address:  ptr("10.0.0.0/24"),
		Method:   "scram-sha-256",
		Options:  ptr("clientcert=verify-full"),
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if !isNew {
		t.Fatal("Configure did not mark appended rule as new")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba.conf: %v", err)
	}
	if !strings.Contains(string(data), "\nhost\tappdb\tapp,migrator\t10.0.0.0/24\tscram-sha-256\tclientcert=verify-full\n") {
		t.Fatalf("pg_hba.conf did not contain appended rule:\n%s", data)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatalf("read archive dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d archive entries, want 1", len(entries))
	}
	if !regexp.MustCompile(`^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_pg_hba\.conf$`).MatchString(entries[0].Name()) {
		t.Fatalf("unexpected archive name: %s", entries[0].Name())
	}
	backup, err := os.ReadFile(filepath.Join(dir, "archive", entries[0].Name()))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != string(original) {
		t.Fatalf("backup = %q, want %q", backup, original)
	}
}

func TestConfigureResolvesRelativeFilePathAgainstConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pg_hba.conf"), []byte(""), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigurePgHbaConfRequest{
		FilePath: "pg_hba.conf",
		Type:     sidecarv1.PgHbaType_PG_HBA_TYPE_LOCAL,
		Database: "all",
		Users:    []string{"postgres"},
		Method:   "peer",
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if !isNew {
		t.Fatal("Configure did not append relative file rule")
	}
}

func ptr(value string) *string {
	return &value
}
