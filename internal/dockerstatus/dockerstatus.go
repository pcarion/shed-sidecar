package dockerstatus

import (
	"context"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
)

type ContainerLister interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
}

type Client struct {
	client ContainerLister
}

func New(client ContainerLister) *Client {
	return &Client{client: client}
}

func (c *Client) Status(ctx context.Context) ([]*sidecarv1.ContainerStatus, error) {
	containers, err := c.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	sort.Slice(containers, func(i, j int) bool {
		return containerName(containers[i]) < containerName(containers[j])
	})

	out := make([]*sidecarv1.ContainerStatus, 0, len(containers))
	for _, ctr := range containers {
		out = append(out, &sidecarv1.ContainerStatus{
			Name:    containerName(ctr),
			State:   string(ctr.State),
			Status:  ctr.Status,
			Image:   ctr.Image,
			Created: ctr.Created,
			Id:      shortID(ctr.ID),
		})
	}
	return out, nil
}

func containerName(ctr container.Summary) string {
	if len(ctr.Names) == 0 {
		return shortID(ctr.ID)
	}
	return strings.TrimPrefix(ctr.Names[0], "/")
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
