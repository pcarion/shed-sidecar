package dockerstatus

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
)

type fakeDockerClient struct {
	containers []container.Summary
}

func (f fakeDockerClient) ContainerList(_ context.Context, options container.ListOptions) ([]container.Summary, error) {
	if !options.All {
		return nil, nil
	}
	return f.containers, nil
}

func TestStatusReturnsSortedContainerStatuses(t *testing.T) {
	client := New(fakeDockerClient{containers: []container.Summary{
		{
			ID:      "abcdef0123456789",
			Names:   []string{"/z-app"},
			Image:   "postgres:16",
			State:   "running",
			Status:  "Up 2 hours",
			Created: 42,
		},
		{
			ID:      "123456789abc",
			Names:   []string{"/a-app"},
			Image:   "redis:7",
			State:   "exited",
			Status:  "Exited (0) 1 hour ago",
			Created: 41,
		},
	}})

	statuses, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2", len(statuses))
	}
	if statuses[0].GetName() != "a-app" || statuses[1].GetName() != "z-app" {
		t.Fatalf("statuses were not sorted by name: %+v", statuses)
	}
	if statuses[1].GetId() != "abcdef012345" {
		t.Fatalf("short id = %q, want abcdef012345", statuses[1].GetId())
	}
	if statuses[1].GetState() != "running" || statuses[1].GetStatus() != "Up 2 hours" || statuses[1].GetImage() != "postgres:16" || statuses[1].GetCreated() != 42 {
		t.Fatalf("unexpected status: %+v", statuses[1])
	}
}

func TestStatusUsesShortIDWhenNameMissing(t *testing.T) {
	client := New(fakeDockerClient{containers: []container.Summary{
		{ID: "abcdef0123456789", State: "created"},
	}})

	statuses, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if statuses[0].GetName() != "abcdef012345" {
		t.Fatalf("name = %q, want short id", statuses[0].GetName())
	}
}
