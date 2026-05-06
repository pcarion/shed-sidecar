package keyvalue

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

func TestConfigureSkipsExistingActiveKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	original := []byte("# port 5432\nport = 5432\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigureKeyValueConfRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Entries: []*sidecarv1.KeyValueEntry{
			entry("port", "5433", sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER),
		},
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if isNew {
		t.Fatal("existing active key was marked new")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("file changed:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "archive")); !os.IsNotExist(err) {
		t.Fatalf("archive dir exists for unchanged file: %v", err)
	}
}

func TestConfigureInsertsAfterMatchingCommentAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	original := []byte("# listen_address controls bind address\n# port controls listener\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigureKeyValueConfRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Entries: []*sidecarv1.KeyValueEntry{
			entry("listen_address", `host "a"`, sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING),
			entry("port", "5432", sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER),
		},
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if !isNew {
		t.Fatal("Configure did not mark new entries")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	want := "# listen_address controls bind address\nlisten_address = \"host \\\"a\\\"\"\n# port controls listener\nport = 5432\n"
	if string(data) != want {
		t.Fatalf("file =\n%s\nwant=\n%s", data, want)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d backups, want 1", len(entries))
	}
	if !regexp.MustCompile(`^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_app\.conf$`).MatchString(entries[0].Name()) {
		t.Fatalf("unexpected backup name: %s", entries[0].Name())
	}
	backup, err := os.ReadFile(filepath.Join(dir, "archive", entries[0].Name()))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != string(original) {
		t.Fatalf("backup = %q, want %q", backup, original)
	}
}

func TestConfigureAppendsWhenNoComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	isNew, err := Configure(&sidecarv1.ConfigureKeyValueConfRequest{
		FilePath: "app.conf",
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_COLON,
		Entries: []*sidecarv1.KeyValueEntry{
			entry("enabled", "true", sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING),
		},
	}, dir)
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if !isNew {
		t.Fatal("Configure did not append")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	if string(data) != "enabled : \"true\"\n" {
		t.Fatalf("file = %q", data)
	}
}

func TestGetIgnoresCommentsAndReturnsValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte("# port = 1111\nport=5432\nname = \"db \\\"main\\\"\"\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	result, err := Get(&sidecarv1.ConfigureGetKeyValueRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Key:      "name",
	}, dir)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !result.Found || result.Value != `db "main"` || result.Type != sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte("# missing = value\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	result, err := Get(&sidecarv1.ConfigureGetKeyValueRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Key:      "missing",
	}, dir)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if result.Found {
		t.Fatalf("commented key was returned: %+v", result)
	}
}

func TestSpaceFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	if _, err := Configure(&sidecarv1.ConfigureKeyValueConfRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_SPACE,
		Entries: []*sidecarv1.KeyValueEntry{
			entry("workers", "4", sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER),
		},
	}, dir); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	if strings.TrimSpace(string(data)) != "workers 4" {
		t.Fatalf("file = %q", data)
	}
}

func entry(key, value string, valueType sidecarv1.KeyValueValueType) *sidecarv1.KeyValueEntry {
	return &sidecarv1.KeyValueEntry{Key: key, Value: value, Type: &valueType}
}
