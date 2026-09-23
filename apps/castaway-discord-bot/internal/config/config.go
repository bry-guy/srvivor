package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevelStr string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogLevel    slog.Level

	DiscordBotToken           string `envconfig:"CASTAWAY_DISCORD_BOT_TOKEN" required:"true"`
	DiscordApplicationID      string `envconfig:"CASTAWAY_DISCORD_APPLICATION_ID" required:"true"`
	DiscordTargetServerID     string `envconfig:"DISCORD_TARGET_SEVER_ID"`
	DiscordTargetServerIDs    string `envconfig:"DISCORD_TARGET_SERVER_IDS"`
	targetServerIDs           []string
	AnnouncementChannelID     string `envconfig:"CASTAWAY_ANNOUNCEMENT_CHANNEL_ID"`
	AdminContactDiscordUserID string `envconfig:"CASTAWAY_ADMIN_CONTACT_DISCORD_USER_ID" default:"235246238382030849"`

	CastawayAPIBaseURL   string `envconfig:"CASTAWAY_API_BASE_URL" default:"http://localhost:8080"`
	CastawayAPIAuthToken string `envconfig:"CASTAWAY_API_AUTH_TOKEN"`
	HTTPAddr             string `envconfig:"BOT_HTTP_ADDR" default:":8080"`

	StateBackend     string `envconfig:"BOT_STATE_BACKEND" default:"bolt"`
	StatePath        string `envconfig:"BOT_STATE_PATH" default:"./data/state.db"`
	StateDatabaseURL string `envconfig:"BOT_STATE_DATABASE_URL"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	targetServerIDs, err := parseTargetServerIDs(cfg.DiscordTargetServerIDs, cfg.DiscordTargetServerID)
	if err != nil {
		return nil, err
	}
	cfg.targetServerIDs = targetServerIDs

	cfg.StateBackend = strings.ToLower(strings.TrimSpace(cfg.StateBackend))
	cfg.CastawayAPIAuthToken = strings.TrimSpace(cfg.CastawayAPIAuthToken)
	cfg.StatePath = strings.TrimSpace(cfg.StatePath)
	cfg.StateDatabaseURL = strings.TrimSpace(cfg.StateDatabaseURL)
	cfg.AnnouncementChannelID = strings.TrimSpace(cfg.AnnouncementChannelID)
	contact, err := strconv.ParseUint(cfg.AdminContactDiscordUserID, 10, 64)
	if err != nil || contact == 0 {
		return nil, fmt.Errorf("CASTAWAY_ADMIN_CONTACT_DISCORD_USER_ID must be a Discord user ID")
	}

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

func (c *Config) TargetServerIDs() []string {
	return append([]string(nil), c.targetServerIDs...)
}

func parseTargetServerIDs(allowlist, legacy string) ([]string, error) {
	configured := strings.TrimSpace(allowlist)
	if configured == "" {
		configured = strings.TrimSpace(legacy)
	}
	if configured == "" {
		return nil, nil
	}

	ids := strings.Split(configured, ",")
	seen := make(map[string]struct{}, len(ids))
	for i, id := range ids {
		parsed, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
		if err != nil || parsed == 0 {
			return nil, fmt.Errorf("DISCORD_TARGET_SERVER_IDS must contain Discord guild IDs")
		}
		ids[i] = strconv.FormatUint(parsed, 10)
		if _, ok := seen[ids[i]]; ok {
			return nil, fmt.Errorf("DISCORD_TARGET_SERVER_IDS contains duplicate guild IDs")
		}
		seen[ids[i]] = struct{}{}
	}
	return ids, nil
}
