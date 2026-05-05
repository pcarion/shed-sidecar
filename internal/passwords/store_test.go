package passwords

import (
	"context"
	"database/sql"
	"path/filepath"
	"regexp"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

func TestStoreGetIsIdempotentForSameKey(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	first, err := store.Get(context.Background(), "svc", "admin", 16, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	if !first.IsNew {
		t.Fatal("first password was not marked new")
	}

	second, err := store.Get(context.Background(), "svc", "admin", 16, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if second.IsNew {
		t.Fatal("second password was marked new")
	}
	if first.Value != second.Value {
		t.Fatalf("password changed for same key: %q != %q", first.Value, second.Value)
	}
}

func TestStoreGetRegeneratesWhenLengthChanges(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	first, err := store.Get(context.Background(), "svc", "admin", 16, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER)
	if err != nil {
		t.Fatalf("first Get returned error: %v", err)
	}
	second, err := store.Get(context.Background(), "svc", "admin", 20, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if !second.IsNew {
		t.Fatal("password with changed length was not marked new")
	}
	if first.Value == second.Value {
		t.Fatalf("password did not change when length changed: %q", first.Value)
	}
	if len(second.Value) != 20 {
		t.Fatalf("password length = %d, want 20", len(second.Value))
	}
}

func TestStoreCreatesExpectedColumns(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	rows, err := store.db.QueryContext(context.Background(), `PRAGMA table_info(passwords)`)
	if err != nil {
		t.Fatalf("table_info returned error: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		got[name] = true
	}
	for _, name := range []string{"service", "name", "value", "generationDate", "length", "type"} {
		if !got[name] {
			t.Fatalf("missing column %q in passwords table", name)
		}
	}
}

func TestGenerateUUIDV7(t *testing.T) {
	value, err := Generate(0, sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value) {
		t.Fatalf("generated UUID is not UUIDv7-shaped: %q", value)
	}
}

func TestGenerateUUIDV7RejectsNonUUIDLength(t *testing.T) {
	if _, err := Generate(20, sidecarv1.PasswordType_PASSWORD_TYPE_UUID_V7); err == nil {
		t.Fatal("Generate returned nil error for invalid UUID length")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "sidecar.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return store
}
