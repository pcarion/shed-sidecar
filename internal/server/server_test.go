package server

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/systemd"
)

type fakeSystemd struct{}

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

func TestServiceStatusReturnsPerServiceErrors(t *testing.T) {
	srv := New(fakeSystemd{}, slog.Default(), nil)

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
	srv := New(fakeSystemd{}, slog.Default(), []string{"good.service"})

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
