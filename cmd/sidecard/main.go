package main

import (
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	sidecarv1 "github.com/pcarion/shed-proto/gen/go/sidecar/v1"
	"github.com/pcarion/shed-sidecar/internal/config"
	"github.com/pcarion/shed-sidecar/internal/server"
	systemdstatus "github.com/pcarion/shed-sidecar/internal/systemd"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", config.DefaultPath, "path to TOML config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	conn, err := systemdstatus.ConnectSystemBus()
	if err != nil {
		logger.Error("connect system bus", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	grpcServer := grpc.NewServer()
	sidecarv1.RegisterSidecarServer(grpcServer, server.New(systemdstatus.NewClient(conn), logger, cfg.AllowedServices))

	tcpAddress := cfg.TCPAddress()
	tcpListener, err := net.Listen("tcp", tcpAddress)
	if err != nil {
		logger.Error("listen tcp", "address", tcpAddress, "error", err)
		os.Exit(1)
	}
	defer tcpListener.Close()

	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0o755); err != nil {
		logger.Error("create socket directory", "path", filepath.Dir(cfg.SocketPath), "error", err)
		os.Exit(1)
	}
	if err := os.Remove(cfg.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("remove stale unix socket", "path", cfg.SocketPath, "error", err)
		os.Exit(1)
	}
	unixListener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		logger.Error("listen unix socket", "path", cfg.SocketPath, "error", err)
		os.Exit(1)
	}
	defer unixListener.Close()
	if err := os.Chmod(cfg.SocketPath, 0o660); err != nil {
		logger.Error("chmod unix socket", "path", cfg.SocketPath, "error", err)
		os.Exit(1)
	}

	errCh := make(chan error, 2)
	go serve(errCh, grpcServer, tcpListener)
	go serve(errCh, grpcServer, unixListener)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("sidecard started", "tcp_address", tcpAddress, "socket_path", cfg.SocketPath)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		logger.Error("grpc server stopped", "error", err)
	}

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		logger.Warn("graceful shutdown timed out")
		grpcServer.Stop()
	}
}

func serve(errCh chan<- error, grpcServer *grpc.Server, listener net.Listener) {
	if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		errCh <- err
	}
}
