package server

import (
	"context"
	"log/slog"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/systemd"
)

type SystemdClient interface {
	Status(ctx context.Context, service string, includeRaw bool) (systemd.Status, error)
}

type Server struct {
	sidecarv1.UnimplementedSidecarServer

	systemd         SystemdClient
	logger          *slog.Logger
	allowedServices map[string]struct{}
}

func New(systemdClient SystemdClient, logger *slog.Logger, allowedServices []string) *Server {
	allowed := map[string]struct{}{}
	for _, service := range allowedServices {
		if service != "" {
			allowed[service] = struct{}{}
		}
	}
	return &Server{
		systemd:         systemdClient,
		logger:          logger,
		allowedServices: allowed,
	}
}

func (s *Server) ServiceStatus(ctx context.Context, req *sidecarv1.ServiceStatusRequest) (*sidecarv1.ServiceStatusResponse, error) {
	includeRaw := req.GetIncludeRaw()
	resp := &sidecarv1.ServiceStatusResponse{
		Statuses: make([]*sidecarv1.ServiceStatus, 0, len(req.GetServices())),
	}

	for _, service := range req.GetServices() {
		if len(s.allowedServices) > 0 {
			if _, ok := s.allowedServices[service]; !ok {
				resp.Statuses = append(resp.Statuses, errorStatus(service, "service is not allowed by sidecar config"))
				continue
			}
		}

		status, err := s.systemd.Status(ctx, service, includeRaw)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("service status query failed", "service", service, "error", err)
			}
			resp.Statuses = append(resp.Statuses, errorStatus(service, err.Error()))
			continue
		}
		resp.Statuses = append(resp.Statuses, protoStatus(status, includeRaw))
	}

	return resp, nil
}

func protoStatus(status systemd.Status, includeRaw bool) *sidecarv1.ServiceStatus {
	out := &sidecarv1.ServiceStatus{
		Name:        status.Name,
		LoadState:   status.LoadState,
		ActiveState: status.ActiveState,
		SubState:    status.SubState,
		Description: status.Description,
		MainPid:     status.MainPID,
	}
	if includeRaw && status.Raw != "" {
		out.Raw = &status.Raw
	}
	return out
}

func errorStatus(service, message string) *sidecarv1.ServiceStatus {
	return &sidecarv1.ServiceStatus{
		Name:        service,
		LoadState:   "error",
		ActiveState: "unknown",
		SubState:    message,
	}
}
