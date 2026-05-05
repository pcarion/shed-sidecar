package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

const DefaultPath = "/etc/sidecar/config.toml"

type Config struct {
	Port            int      `toml:"port"`
	SocketPath      string   `toml:"socket_path"`
	DatabasePath    string   `toml:"database_path"`
	NetworkPortMin  int      `toml:"network_port_min"`
	NetworkPortMax  int      `toml:"network_port_max"`
	AllowedServices []string `toml:"allowed_services"`
}

func Default() Config {
	return Config{
		Port:           50051,
		SocketPath:     "/run/sidecar/sidecar.sock",
		DatabasePath:   "sidecar.db",
		NetworkPortMin: 20000,
		NetworkPortMax: 29999,
	}
}

func (c Config) TCPAddress() string {
	return "127.0.0.1:" + strconv.Itoa(c.Port)
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return finalize(path, cfg), nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.Port == 0 {
		cfg.Port = Default().Port
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = Default().SocketPath
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = Default().DatabasePath
	}
	if cfg.NetworkPortMin == 0 {
		cfg.NetworkPortMin = Default().NetworkPortMin
	}
	if cfg.NetworkPortMax == 0 {
		cfg.NetworkPortMax = Default().NetworkPortMax
	}
	if cfg.NetworkPortMin < 1 || cfg.NetworkPortMax > 65535 || cfg.NetworkPortMin > cfg.NetworkPortMax {
		return Config{}, fmt.Errorf("invalid network port range %d-%d", cfg.NetworkPortMin, cfg.NetworkPortMax)
	}
	return finalize(path, cfg), nil
}

func finalize(path string, cfg Config) Config {
	if !filepath.IsAbs(cfg.DatabasePath) {
		cfg.DatabasePath = filepath.Join(filepath.Dir(path), cfg.DatabasePath)
	}
	return cfg
}
