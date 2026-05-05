package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

const DefaultPath = "/etc/sidecar/config.toml"

type Config struct {
	Port            int      `toml:"port"`
	SocketPath      string   `toml:"socket_path"`
	AllowedServices []string `toml:"allowed_services"`
}

func Default() Config {
	return Config{
		Port:       8443,
		SocketPath: "/run/sidecar/sidecar.sock",
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
			return cfg, nil
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
	return cfg, nil
}
