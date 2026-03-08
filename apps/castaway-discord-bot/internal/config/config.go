package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevelStr string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogLevel    slog.Level

	DiscordBotToken      string `envconfig:"CASTAWAY_DISCORD_BOT_TOKEN" required:"true"`
	DiscordApplicationID string `envconfig:"CASTAWAY_DISCORD_APPLICATION_ID" required:"true"`
	DiscordDevGuildID    string `envconfig:"DISCORD_BRAINLAND_SERVER_ID"`

	CastawayAPIBaseURL string `envconfig:"CASTAWAY_API_BASE_URL" default:"http://localhost:8080"`
	StatePath          string `envconfig:"BOT_STATE_PATH" default:"./data/state.db"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if strings.TrimSpace(cfg.StatePath) == "" {
		return nil, fmt.Errorf("BOT_STATE_PATH is required")
	}

	if _, err := url.ParseRequestURI(cfg.CastawayAPIBaseURL); err != nil {
		return nil, fmt.Errorf("parse CASTAWAY_API_BASE_URL: %w", err)
	}

	switch strings.ToUpper(cfg.LogLevelStr) {
	case "DEBUG":
		cfg.LogLevel = slog.LevelDebug
	case "INFO":
		cfg.LogLevel = slog.LevelInfo
	case "WARN":
		cfg.LogLevel = slog.LevelWarn
	case "ERROR":
		cfg.LogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid LOG_LEVEL: %s", cfg.LogLevelStr)
	}

	return &cfg, nil
}
