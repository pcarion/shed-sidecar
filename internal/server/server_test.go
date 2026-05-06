package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/passwords"
	"github.com/pcarion/shed-sidecar/internal/systemd"
)

type fakeSystemd struct{}
type fakePasswords struct{}
type fakeDocker struct{}

func (fakeSystemd) Status(_ context.Context, service string, includeRaw bool) (systemd.Status, error) {
	if service == "bad.service" {
		return systemd.Status{Name: service}, errors.New("dbus failed")
	}
	return systemd.Status{
		Name:        service,
		LoadState:   "loaded",
		ActiveState: "active",
		SubState:    "running",
		Description: "Good Service",
		MainPID:     42,
	}, nil
}

func (fakePasswords) Get(_ context.Context, service, name string, length int32, passwordType sidecarv1.PasswordType) (passwords.Record, error) {
	return passwords.Record{Value: "secret", IsNew: true}, nil
}

func (fakePasswords) Read(_ context.Context, service, name string) (string, bool, error) {
	if name == "missing" {
		return "", false, nil
	}
	return "secret", true, nil
}

func (fakePasswords) List(_ context.Context) ([]passwords.Entry, error) {
	return []passwords.Entry{
		{Service: "svc-a", Name: "admin", Value: "secret-a"},
		{Service: "svc-b", Name: "admin", Value: "secret-b"},
	}, nil
}

func (fakePasswords) NetworkPortGet(_ context.Context, service, name string) (passwords.NetworkEntry, bool, error) {
	return passwords.NetworkEntry{Service: service, Name: name, Port: 20000}, true, nil
}

func (fakePasswords) NetworkList(_ context.Context) ([]passwords.NetworkEntry, error) {
	return []passwords.NetworkEntry{
		{Service: "svc-a", Name: "http", Port: 20000},
		{Service: "svc-b", Name: "http", Port: 20001},
	}, nil
}

func (fakePasswords) ParamSet(_ context.Context, service, name, value string) error {
	return nil
}

func (fakePasswords) ParamGet(_ context.Context, service, name string) (string, bool, error) {
	if name == "missing" {
		return "", false, nil
	}
	return "value", true, nil
}

func (fakePasswords) ParamList(_ context.Context) ([]passwords.ParamEntry, error) {
	return []passwords.ParamEntry{
		{Service: "svc-a", Name: "api-url", Value: "https://a.example"},
		{Service: "svc-b", Name: "api-url", Value: "https://b.example"},
	}, nil
}

func (fakeDocker) Status(_ context.Context) ([]*sidecarv1.ContainerStatus, error) {
	return []*sidecarv1.ContainerStatus{
		{Name: "app", State: "running", Status: "Up 2 hours", Image: "postgres:16", Created: 42, Id: "abcdef012345"},
	}, nil
}

func TestServiceStatusReturnsPerServiceErrors(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.ServiceStatus(context.Background(), &sidecarv1.ServiceStatusRequest{
		Services: []string{"good.service", "bad.service"},
	})
	if err != nil {
		t.Fatalf("ServiceStatus returned RPC error: %v", err)
	}
	if len(resp.GetStatuses()) != 2 {
		t.Fatalf("got %d statuses, want 2", len(resp.GetStatuses()))
	}
	if resp.GetStatuses()[0].GetActiveState() != "active" {
		t.Fatalf("good service was not returned: %+v", resp.GetStatuses()[0])
	}
	if resp.GetStatuses()[1].GetLoadState() != "error" {
		t.Fatalf("bad service did not return per-service error: %+v", resp.GetStatuses()[1])
	}
}

func TestServiceStatusAllowsBareNameWhenAllowedListUsesServiceSuffix(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), []string{"good.service"})

	resp, err := srv.ServiceStatus(context.Background(), &sidecarv1.ServiceStatusRequest{
		Services: []string{"good"},
	})
	if err != nil {
		t.Fatalf("ServiceStatus returned RPC error: %v", err)
	}
	if len(resp.GetStatuses()) != 1 {
		t.Fatalf("got %d statuses, want 1", len(resp.GetStatuses()))
	}
	if resp.GetStatuses()[0].GetLoadState() == "error" {
		t.Fatalf("bare service name was rejected: %+v", resp.GetStatuses()[0])
	}
}

func TestDockerStatusReturnsContainers(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, fakeDocker{})

	resp, err := srv.DockerStatus(context.Background(), &sidecarv1.DockerStatusRequest{})
	if err != nil {
		t.Fatalf("DockerStatus returned error: %v", err)
	}
	if len(resp.GetContainers()) != 1 {
		t.Fatalf("got %d containers, want 1", len(resp.GetContainers()))
	}
	container := resp.GetContainers()[0]
	if container.GetName() != "app" || container.GetState() != "running" || container.GetStatus() != "Up 2 hours" || container.GetImage() != "postgres:16" || container.GetCreated() != 42 || container.GetId() != "abcdef012345" {
		t.Fatalf("unexpected container: %+v", container)
	}
}

func TestDockerStatusRequiresClient(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	if _, err := srv.DockerStatus(context.Background(), &sidecarv1.DockerStatusRequest{}); err == nil {
		t.Fatal("DockerStatus returned nil error without docker client")
	}
}

func TestPasswordGetReturnsStoredPassword(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.PasswordGet(context.Background(), &sidecarv1.PasswordGetRequest{
		ServiceName: "svc",
		Name:        "admin",
		Length:      16,
		Type:        sidecarv1.PasswordType_PASSWORD_TYPE_HEX_LOWER,
	})
	if err != nil {
		t.Fatalf("PasswordGet returned error: %v", err)
	}
	if resp.GetPassword() != "secret" || !resp.GetIsNew() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestPasswordReadReturnsStoredPassword(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.PasswordRead(context.Background(), &sidecarv1.PasswordReadRequest{
		ServiceName: "svc",
		Name:        "admin",
	})
	if err != nil {
		t.Fatalf("PasswordRead returned error: %v", err)
	}
	if resp.GetPassword() != "secret" || !resp.GetIsOk() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestPasswordReadReturnsNotFound(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.PasswordRead(context.Background(), &sidecarv1.PasswordReadRequest{
		ServiceName: "svc",
		Name:        "missing",
	})
	if err != nil {
		t.Fatalf("PasswordRead returned error: %v", err)
	}
	if resp.GetPassword() != "" || resp.GetIsOk() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestPasswordListReturnsGroupedPasswords(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.PasswordList(context.Background(), &sidecarv1.PasswordListRequest{})
	if err != nil {
		t.Fatalf("PasswordList returned error: %v", err)
	}
	if len(resp.GetServices()) != 2 {
		t.Fatalf("got %d services, want 2", len(resp.GetServices()))
	}
	if got := resp.GetServices()[0].GetPasswords()[0].GetPassword(); got != "secret-a" {
		t.Fatalf("first password = %q, want secret-a", got)
	}
}

func TestNetworkPortGetReturnsStoredPort(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.NetworkPortGet(context.Background(), &sidecarv1.NetworkPortGetRequest{
		ServiceName: "svc",
		Name:        "http",
	})
	if err != nil {
		t.Fatalf("NetworkPortGet returned error: %v", err)
	}
	if resp.GetPort() != 20000 || !resp.GetIsNew() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestNetworkListReturnsPorts(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.NetworkList(context.Background(), &sidecarv1.NetworkListRequest{})
	if err != nil {
		t.Fatalf("NetworkList returned error: %v", err)
	}
	if len(resp.GetNetworks()) != 2 {
		t.Fatalf("got %d networks, want 2", len(resp.GetNetworks()))
	}
	if got := resp.GetNetworks()[0].GetPort(); got != 20000 {
		t.Fatalf("first network port = %d, want 20000", got)
	}
}

func TestParamSetReturnsOK(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	if _, err := srv.ParamSet(context.Background(), &sidecarv1.ParamSetRequest{
		ServiceName: "svc",
		Name:        "api-url",
		Value:       "https://example.test",
	}); err != nil {
		t.Fatalf("ParamSet returned error: %v", err)
	}
}

func TestParamGetReturnsStoredValue(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.ParamGet(context.Background(), &sidecarv1.ParamGetRequest{
		ServiceName: "svc",
		Name:        "api-url",
	})
	if err != nil {
		t.Fatalf("ParamGet returned error: %v", err)
	}
	if resp.GetValue() != "value" {
		t.Fatalf("ParamGet value = %q, want value", resp.GetValue())
	}
}

func TestParamGetReturnsNotFound(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	if _, err := srv.ParamGet(context.Background(), &sidecarv1.ParamGetRequest{
		ServiceName: "svc",
		Name:        "missing",
	}); err == nil {
		t.Fatal("ParamGet returned nil error for missing param")
	}
}

func TestParamListReturnsGroupedParams(t *testing.T) {
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil)

	resp, err := srv.ParamList(context.Background(), &sidecarv1.ParamListRequest{})
	if err != nil {
		t.Fatalf("ParamList returned error: %v", err)
	}
	if len(resp.GetServices()) != 2 {
		t.Fatalf("got %d services, want 2", len(resp.GetServices()))
	}
	if got := resp.GetServices()[0].GetParams()[0].GetValue(); got != "https://a.example" {
		t.Fatalf("first param = %q, want https://a.example", got)
	}
}

func TestConfigurePgHbaConfReturnsExistingRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("host all app 10.0.0.0/24 scram-sha-256\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, dir)

	resp, err := srv.ConfigurePgHbaConf(context.Background(), &sidecarv1.ConfigurePgHbaConfRequest{
		FilePath: path,
		Type:     sidecarv1.PgHbaType_PG_HBA_TYPE_HOST,
		Database: "all",
		Users:    []string{"app"},
		Address:  ptr("10.0.0.0/24"),
		Method:   "scram-sha-256",
	})
	if err != nil {
		t.Fatalf("ConfigurePgHbaConf returned error: %v", err)
	}
	if !resp.GetIsValid() || resp.GetIsNew() {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestConfigurePgHbaConfAppendsRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pg_hba.conf")
	if err := os.WriteFile(path, []byte("local all postgres peer\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, dir)

	resp, err := srv.ConfigurePgHbaConf(context.Background(), &sidecarv1.ConfigurePgHbaConfRequest{
		FilePath: path,
		Type:     sidecarv1.PgHbaType_PG_HBA_TYPE_HOST,
		Database: "all",
		Users:    []string{"app"},
		Address:  ptr("10.0.0.0/24"),
		Method:   "scram-sha-256",
	})
	if err != nil {
		t.Fatalf("ConfigurePgHbaConf returned error: %v", err)
	}
	if !resp.GetIsValid() || !resp.GetIsNew() {
		t.Fatalf("unexpected response: %+v", resp)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pg_hba.conf: %v", err)
	}
	if !strings.Contains(string(data), "host\tall\tapp\t10.0.0.0/24\tscram-sha-256\n") {
		t.Fatalf("pg_hba.conf missing appended rule:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "archive")); err != nil {
		t.Fatalf("archive directory was not created: %v", err)
	}
}

func TestConfigureKeyValueConfAppendsRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte("# port controls listener\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	valueType := sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_NUMBER
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, dir)

	resp, err := srv.ConfigureKeyValueConf(context.Background(), &sidecarv1.ConfigureKeyValueConfRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Entries: []*sidecarv1.KeyValueEntry{
			{Key: "port", Value: "5432", Type: &valueType},
		},
	})
	if err != nil {
		t.Fatalf("ConfigureKeyValueConf returned error: %v", err)
	}
	if !resp.GetIsValid() || !resp.GetIsNew() {
		t.Fatalf("unexpected response: %+v", resp)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conf: %v", err)
	}
	if !strings.Contains(string(data), "port = 5432\n") {
		t.Fatalf("conf missing key:\n%s", data)
	}
}

func TestConfigureGetKeyValueReturnsValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte("name = \"db\"\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, dir)

	resp, err := srv.ConfigureGetKeyValue(context.Background(), &sidecarv1.ConfigureGetKeyValueRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Key:      "name",
	})
	if err != nil {
		t.Fatalf("ConfigureGetKeyValue returned error: %v", err)
	}
	if !resp.GetIsValid() || resp.GetValue() != "db" || resp.GetType() != sidecarv1.KeyValueValueType_KEY_VALUE_VALUE_TYPE_STRING {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestConfigureGetKeyValueReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(path, []byte("# name = \"db\"\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	srv := New(fakeSystemd{}, fakePasswords{}, slog.Default(), nil, dir)

	resp, err := srv.ConfigureGetKeyValue(context.Background(), &sidecarv1.ConfigureGetKeyValueRequest{
		FilePath: path,
		Type:     sidecarv1.KeyValueConfType_KEY_VALUE_CONF_TYPE_EQUAL,
		Key:      "name",
	})
	if err != nil {
		t.Fatalf("ConfigureGetKeyValue returned error: %v", err)
	}
	if resp.GetIsValid() || resp.Value != nil || resp.Type != nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func ptr(value string) *string {
	return &value
}
