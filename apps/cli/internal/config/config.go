package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevelStr string `envconfig:"LOG_LEVEL" default:"ERROR"`
	LogLevel    slog.Level

	// Validation settings
	FuzzyMatchThreshold float64 `envconfig:"SRVVR_FUZZY_THRESHOLD" envDefault:"0.70"`
	RequireExactMatch   bool    `envconfig:"SRVVR_REQUIRE_EXACT" envDefault:"false"`

	// Discord bot settings
	DiscordBotURL string `envconfig:"DISCORD_BOT_URL" default:"http://localhost:8080/publish"`
}

func Validate() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}

	// Parse log level
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
