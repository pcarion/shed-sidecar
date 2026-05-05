package passwords

import (
	"context"
	"database/sql"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

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

func TestStoreReadReturnsNewestPasswordForServiceAndName(t *testing.T) {
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

	value, ok, err := store.Read(context.Background(), "svc", "admin")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if !ok {
		t.Fatal("Read did not find password")
	}
	if value != second.Value || value == first.Value {
		t.Fatalf("Read returned %q, want newest %q", value, second.Value)
	}
}

func TestStoreReadReturnsNotFound(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	value, ok, err := store.Read(context.Background(), "svc", "missing")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if ok || value != "" {
		t.Fatalf("Read = %q, %v; want empty false", value, ok)
	}
}

func TestStoreListReturnsServiceSortedEntries(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	if _, err := store.Get(context.Background(), "zsvc", "zpass", 16, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER); err != nil {
		t.Fatalf("Get zsvc returned error: %v", err)
	}
	if _, err := store.Get(context.Background(), "asvc", "apass", 16, sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER); err != nil {
		t.Fatalf("Get asvc returned error: %v", err)
	}

	entries, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Service != "asvc" || entries[0].Name != "apass" {
		t.Fatalf("entries were not sorted by service/name: %+v", entries)
	}
	if entries[1].Service != "zsvc" || entries[1].Name != "zpass" {
		t.Fatalf("entries were not sorted by service/name: %+v", entries)
	}
}

func TestStoreNetworkPortGetIsIdempotent(t *testing.T) {
	store := openTestNetworkStore(t, 20000, 20002)
	defer store.Close()

	first, isNew, err := store.NetworkPortGet(context.Background(), "svc", "http")
	if err != nil {
		t.Fatalf("first NetworkPortGet returned error: %v", err)
	}
	if !isNew {
		t.Fatal("first network port was not marked new")
	}
	if first.Port != 20000 {
		t.Fatalf("first port = %d, want 20000", first.Port)
	}

	second, isNew, err := store.NetworkPortGet(context.Background(), "svc", "http")
	if err != nil {
		t.Fatalf("second NetworkPortGet returned error: %v", err)
	}
	if isNew {
		t.Fatal("second network port was marked new")
	}
	if second.Port != first.Port {
		t.Fatalf("port changed for same service/name: %d != %d", second.Port, first.Port)
	}
}

func TestStoreNetworkPortGetSkipsUsedAndUnavailablePorts(t *testing.T) {
	store := openTestNetworkStore(t, 20000, 20002)
	defer store.Close()
	store.portAvailable = func(port int) bool {
		return port != 20001
	}

	first, _, err := store.NetworkPortGet(context.Background(), "svc-a", "http")
	if err != nil {
		t.Fatalf("first NetworkPortGet returned error: %v", err)
	}
	second, _, err := store.NetworkPortGet(context.Background(), "svc-b", "http")
	if err != nil {
		t.Fatalf("second NetworkPortGet returned error: %v", err)
	}
	if first.Port != 20000 {
		t.Fatalf("first port = %d, want 20000", first.Port)
	}
	if second.Port != 20002 {
		t.Fatalf("second port = %d, want 20002", second.Port)
	}
}

func TestStoreNetworkPortGetReturnsExistingPortWithoutAvailabilityCheck(t *testing.T) {
	store := openTestNetworkStore(t, 20000, 20002)
	defer store.Close()

	first, _, err := store.NetworkPortGet(context.Background(), "svc", "http")
	if err != nil {
		t.Fatalf("first NetworkPortGet returned error: %v", err)
	}
	if first.Port != 20000 {
		t.Fatalf("first port = %d, want 20000", first.Port)
	}

	store.portAvailable = func(port int) bool {
		return port != 20000
	}

	second, isNew, err := store.NetworkPortGet(context.Background(), "svc", "http")
	if err != nil {
		t.Fatalf("second NetworkPortGet returned error: %v", err)
	}
	if isNew {
		t.Fatal("existing network port was marked new")
	}
	if second.Port != 20000 {
		t.Fatalf("existing port = %d, want 20000", second.Port)
	}

	entries, err := store.NetworkList(context.Background())
	if err != nil {
		t.Fatalf("NetworkList returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Port != 20000 {
		t.Fatalf("unexpected persisted entries: %+v", entries)
	}
}

func TestStoreNetworkList(t *testing.T) {
	store := openTestNetworkStore(t, 20000, 20002)
	defer store.Close()

	if _, _, err := store.NetworkPortGet(context.Background(), "zsvc", "http"); err != nil {
		t.Fatalf("NetworkPortGet zsvc returned error: %v", err)
	}
	if _, _, err := store.NetworkPortGet(context.Background(), "asvc", "http"); err != nil {
		t.Fatalf("NetworkPortGet asvc returned error: %v", err)
	}

	entries, err := store.NetworkList(context.Background())
	if err != nil {
		t.Fatalf("NetworkList returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Service != "asvc" || entries[0].Port != 20001 {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Service != "zsvc" || entries[1].Port != 20000 {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
}

func TestStoreNetworkPortGetReturnsErrorWhenRangeExhausted(t *testing.T) {
	store := openTestNetworkStore(t, 20000, 20000)
	defer store.Close()

	if _, _, err := store.NetworkPortGet(context.Background(), "svc-a", "http"); err != nil {
		t.Fatalf("NetworkPortGet svc-a returned error: %v", err)
	}
	if _, _, err := store.NetworkPortGet(context.Background(), "svc-b", "http"); err == nil {
		t.Fatal("NetworkPortGet returned nil error for exhausted range")
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

func TestStoreCreatesNetworkPortsTable(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()

	rows, err := store.db.QueryContext(context.Background(), `PRAGMA table_info(network_ports)`)
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
	for _, name := range []string{"service", "name", "port", "generationDate"} {
		if !got[name] {
			t.Fatalf("missing column %q in network_ports table", name)
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

func TestGenerateLowercaseOnly(t *testing.T) {
	value, err := Generate(64, sidecarv1.PasswordType_PASSWORD_TYPE_LOWERCASE)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	for _, r := range value {
		if r < 'a' || r > 'z' {
			t.Fatalf("generated value contains non-lowercase character %q in %q", r, value)
		}
	}
}

func TestGenerateUppercaseIncludesLowercaseAndUppercase(t *testing.T) {
	value, err := Generate(64, sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !containsLower(value) || !containsUpper(value) {
		t.Fatalf("generated value does not contain lower and upper case: %q", value)
	}
	for _, r := range value {
		if !unicode.IsLower(r) && !unicode.IsUpper(r) {
			t.Fatalf("generated value contains unexpected character %q in %q", r, value)
		}
	}
}

func TestGenerateSymbolPolicy(t *testing.T) {
	value, err := Generate(64, sidecarv1.PasswordType_PASSWORD_TYPE_SYMBOL)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !containsLower(value) || !containsUpper(value) || !containsAny(value, symbolAlphabet) {
		t.Fatalf("generated value does not contain lower, upper, and symbol: %q", value)
	}
	if strings.ContainsAny(value, `$/\()`) {
		t.Fatalf("generated value contains excluded special character: %q", value)
	}
}

func TestGenerateRequiredSetLengthValidation(t *testing.T) {
	if _, err := Generate(1, sidecarv1.PasswordType_PASSWORD_TYPE_UPPERCASE); err == nil {
		t.Fatal("Generate returned nil error for uppercase policy length 1")
	}
	if _, err := Generate(2, sidecarv1.PasswordType_PASSWORD_TYPE_SYMBOL); err == nil {
		t.Fatal("Generate returned nil error for symbol policy length 2")
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

func openTestNetworkStore(t *testing.T, minPort, maxPort int) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "sidecar.db"), NetworkPortRange{
		Min: minPort,
		Max: maxPort,
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	store.portAvailable = func(port int) bool { return true }
	return store
}

func containsLower(value string) bool {
	for _, r := range value {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func containsUpper(value string) bool {
	for _, r := range value {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func containsAny(value, chars string) bool {
	for _, r := range value {
		if strings.ContainsRune(chars, r) {
			return true
		}
	}
	return false
}
