package server

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/passwords"
	"github.com/pcarion/shed-sidecar/internal/systemd"
)

type fakeSystemd struct{}
type fakePasswords struct{}

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
