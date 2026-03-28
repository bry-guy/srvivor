package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevelStr string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogLevel    slog.Level

	DiscordBotToken       string `envconfig:"CASTAWAY_DISCORD_BOT_TOKEN" required:"true"`
	DiscordApplicationID  string `envconfig:"CASTAWAY_DISCORD_APPLICATION_ID" required:"true"`
	DiscordTargetServerID string `envconfig:"DISCORD_TARGET_SEVER_ID"`

	CastawayAPIBaseURL   string   `envconfig:"CASTAWAY_API_BASE_URL" default:"http://localhost:8080"`
	CastawayAPIAuthToken string   `envconfig:"CASTAWAY_API_AUTH_TOKEN"`
	DiscordAdminUserIDs  []string `envconfig:"DISCORD_ADMIN_USER_IDS"`

	StateBackend     string `envconfig:"BOT_STATE_BACKEND" default:"bolt"`
	StatePath        string `envconfig:"BOT_STATE_PATH" default:"./data/state.db"`
	StateDatabaseURL string `envconfig:"BOT_STATE_DATABASE_URL"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	cfg.StateBackend = strings.ToLower(strings.TrimSpace(cfg.StateBackend))
	cfg.CastawayAPIAuthToken = strings.TrimSpace(cfg.CastawayAPIAuthToken)
	cfg.StatePath = strings.TrimSpace(cfg.StatePath)
	cfg.StateDatabaseURL = strings.TrimSpace(cfg.StateDatabaseURL)
	cfg.DiscordAdminUserIDs = normalizeCSVList(cfg.DiscordAdminUserIDs)

	if _, err := url.ParseRequestURI(cfg.CastawayAPIBaseURL); err != nil {
		return nil, fmt.Errorf("parse CASTAWAY_API_BASE_URL: %w", err)
	}

	switch state.Backend(cfg.StateBackend) {
	case state.BackendBolt:
		if cfg.StatePath == "" {
			return nil, fmt.Errorf("BOT_STATE_PATH is required when BOT_STATE_BACKEND=bolt")
		}
	case state.BackendPostgres:
		if cfg.StateDatabaseURL == "" {
			return nil, fmt.Errorf("BOT_STATE_DATABASE_URL is required when BOT_STATE_BACKEND=postgres")
		}
		if _, err := url.ParseRequestURI(cfg.StateDatabaseURL); err != nil {
			return nil, fmt.Errorf("parse BOT_STATE_DATABASE_URL: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid BOT_STATE_BACKEND: %s", cfg.StateBackend)
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

func normalizeCSVList(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
