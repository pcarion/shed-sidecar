package server

import (
	"context"
	"log/slog"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/passwords"
	"github.com/pcarion/shed-sidecar/internal/systemd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SystemdClient interface {
	Status(ctx context.Context, service string, includeRaw bool) (systemd.Status, error)
}

type PasswordStore interface {
	Get(ctx context.Context, service, name string, length int32, passwordType sidecarv1.PasswordType) (passwords.Record, error)
	Read(ctx context.Context, service, name string) (string, bool, error)
	List(ctx context.Context) ([]passwords.Entry, error)
	NetworkPortGet(ctx context.Context, service, name string) (passwords.NetworkEntry, bool, error)
	NetworkList(ctx context.Context) ([]passwords.NetworkEntry, error)
}

type Server struct {
	sidecarv1.UnimplementedSidecarServer

	systemd         SystemdClient
	passwords       PasswordStore
	logger          *slog.Logger
	allowedServices map[string]struct{}
}

func New(systemdClient SystemdClient, passwordStore PasswordStore, logger *slog.Logger, allowedServices []string) *Server {
	allowed := map[string]struct{}{}
	for _, service := range allowedServices {
		if service != "" {
			allowed[service] = struct{}{}
			allowed[systemd.NormalizeUnitName(service)] = struct{}{}
		}
	}
	return &Server{
		systemd:         systemdClient,
		passwords:       passwordStore,
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
				if _, ok := s.allowedServices[systemd.NormalizeUnitName(service)]; !ok {
					resp.Statuses = append(resp.Statuses, errorStatus(service, "service is not allowed by sidecar config"))
					continue
				}
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

func (s *Server) PasswordGet(ctx context.Context, req *sidecarv1.PasswordGetRequest) (*sidecarv1.PasswordGetResponse, error) {
	if s.passwords == nil {
		return nil, status.Error(codes.FailedPrecondition, "password store is not configured")
	}
	record, err := s.passwords.Get(ctx, req.GetServiceName(), req.GetName(), req.GetLength(), req.GetType())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &sidecarv1.PasswordGetResponse{
		Password: record.Value,
		IsNew:    record.IsNew,
	}, nil
}

func (s *Server) PasswordRead(ctx context.Context, req *sidecarv1.PasswordReadRequest) (*sidecarv1.PasswordReadResponse, error) {
	if s.passwords == nil {
		return nil, status.Error(codes.FailedPrecondition, "password store is not configured")
	}
	value, ok, err := s.passwords.Read(ctx, req.GetServiceName(), req.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &sidecarv1.PasswordReadResponse{
		Password: value,
		IsOk:     ok,
	}, nil
}

func (s *Server) PasswordList(ctx context.Context, req *sidecarv1.PasswordListRequest) (*sidecarv1.PasswordListResponse, error) {
	if s.passwords == nil {
		return nil, status.Error(codes.FailedPrecondition, "password store is not configured")
	}
	entries, err := s.passwords.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &sidecarv1.PasswordListResponse{}
	var current *sidecarv1.PasswordService
	for _, entry := range entries {
		if current == nil || current.ServiceName != entry.Service {
			current = &sidecarv1.PasswordService{ServiceName: entry.Service}
			resp.Services = append(resp.Services, current)
		}
		current.Passwords = append(current.Passwords, &sidecarv1.PasswordEntry{
			Name:     entry.Name,
			Password: entry.Value,
		})
	}
	return resp, nil
}

func (s *Server) NetworkPortGet(ctx context.Context, req *sidecarv1.NetworkPortGetRequest) (*sidecarv1.NetworkPortGetResponse, error) {
	if s.passwords == nil {
		return nil, status.Error(codes.FailedPrecondition, "password store is not configured")
	}
	entry, isNew, err := s.passwords.NetworkPortGet(ctx, req.GetServiceName(), req.GetName())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &sidecarv1.NetworkPortGetResponse{
		Port:  entry.Port,
		IsNew: isNew,
	}, nil
}

func (s *Server) NetworkList(ctx context.Context, req *sidecarv1.NetworkListRequest) (*sidecarv1.NetworkListResponse, error) {
	if s.passwords == nil {
		return nil, status.Error(codes.FailedPrecondition, "password store is not configured")
	}
	entries, err := s.passwords.NetworkList(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &sidecarv1.NetworkListResponse{
		Networks: make([]*sidecarv1.NetworkEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Networks = append(resp.Networks, &sidecarv1.NetworkEntry{
			ServiceName: entry.Service,
			Name:        entry.Name,
			Port:        entry.Port,
		})
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
