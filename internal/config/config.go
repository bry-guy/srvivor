package config

import (
	"fmt"
	"log/slog"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevel slog.Level `envconfig:"LOG_LEVEL"`
}

func Validate() (*Config, error) {
	var cfg Config
	err := envconfig.Process("srvvr", &cfg) // TODO: Remove srvvr prefix
	if err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}

	return &cfg, nil
}
