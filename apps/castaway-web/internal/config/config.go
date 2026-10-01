package config

import (
	"github.com/google/uuid"

	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                        string
	DatabaseURL                 string
	AutoMigrate                 bool
	MigrationsDir               string
	ServiceAuthEnabled          bool
	ServiceAuthBearerTokens     []string
	ServiceAuthPrincipal        string
	BootstrapAdminDiscordUserID string
	// Public listener (website + /api behind Discord login); disabled when PublicPort is empty.
	PublicPort          string
	PublicBaseURL       string
	DiscordClientID     string
	DiscordClientSecret string
	PublicInstanceID    string
	PublicLeagueName    string
	PublicLeagueIDs     []string
}

func Load() (*Config, error) {
	cfg := &Config{
		BootstrapAdminDiscordUserID: strings.TrimSpace(getEnv("BOOTSTRAP_ADMIN_DISCORD_USER_ID", "")),
		Port:                        getEnv("PORT", "8080"),
		DatabaseURL:                 getEnv("DATABASE_URL", "postgres://castaway:castaway@localhost:5432/castaway?sslmode=disable"),
		MigrationsDir:               getEnv("MIGRATIONS_DIR", "./db/migrations"),
		ServiceAuthPrincipal:        strings.TrimSpace(getEnv("SERVICE_AUTH_PRINCIPAL", "castaway-discord-bot")),
		PublicPort:                  strings.TrimSpace(getEnv("PUBLIC_PORT", "")),
		PublicBaseURL:               strings.TrimSpace(getEnv("PUBLIC_BASE_URL", "")),
		DiscordClientID:             strings.TrimSpace(getEnv("DISCORD_OAUTH_CLIENT_ID", "")),
		DiscordClientSecret:         strings.TrimSpace(getEnv("DISCORD_OAUTH_CLIENT_SECRET", "")),
		PublicInstanceID:            strings.TrimSpace(getEnv("PUBLIC_INSTANCE_ID", "")),
	}
	if cfg.PublicInstanceID != "" {
		if _, err := uuid.Parse(cfg.PublicInstanceID); err != nil {
			return nil, fmt.Errorf("PUBLIC_INSTANCE_ID must be the instance's public UUID: %w", err)
		}
	}
	cfg.PublicLeagueName = strings.TrimSpace(getEnv("PUBLIC_LEAGUE_NAME", ""))
	for _, id := range strings.Split(getEnv("PUBLIC_LEAGUE_INSTANCE_IDS", ""), ",") {
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("PUBLIC_LEAGUE_INSTANCE_IDS must be public instance UUIDs: %w", err)
		}
		cfg.PublicLeagueIDs = append(cfg.PublicLeagueIDs, id)
	}
	if cfg.PublicPort != "" && (cfg.PublicBaseURL == "" || cfg.DiscordClientID == "" || cfg.DiscordClientSecret == "") {
		return nil, fmt.Errorf("PUBLIC_PORT needs PUBLIC_BASE_URL, DISCORD_OAUTH_CLIENT_ID and DISCORD_OAUTH_CLIENT_SECRET")
	}

	autoMigrate, err := strconv.ParseBool(getEnv("AUTO_MIGRATE", "true"))
	if err != nil {
		return nil, fmt.Errorf("parse AUTO_MIGRATE: %w", err)
	}
	cfg.AutoMigrate = autoMigrate

	serviceAuthEnabled, err := strconv.ParseBool(getEnv("SERVICE_AUTH_ENABLED", "false"))
	if err != nil {
		return nil, fmt.Errorf("parse SERVICE_AUTH_ENABLED: %w", err)
	}
	cfg.ServiceAuthEnabled = serviceAuthEnabled
	cfg.ServiceAuthBearerTokens = parseCSV(getEnv("SERVICE_AUTH_BEARER_TOKENS", ""))

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.ServiceAuthEnabled && len(cfg.ServiceAuthBearerTokens) == 0 {
		return nil, fmt.Errorf("SERVICE_AUTH_BEARER_TOKENS is required when SERVICE_AUTH_ENABLED=true")
	}
	if cfg.ServiceAuthPrincipal == "" {
		return nil, fmt.Errorf("SERVICE_AUTH_PRINCIPAL is required when service auth is configured")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	parsed := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		parsed = append(parsed, trimmed)
	}
	return parsed
}
