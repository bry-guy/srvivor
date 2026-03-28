package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	AutoMigrate             bool
	MigrationsDir           string
	ServiceAuthEnabled      bool
	ServiceAuthBearerTokens []string
	ServiceAuthPrincipal    string
	DiscordAdminUserIDs     []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://castaway:castaway@localhost:5432/castaway?sslmode=disable"),
		MigrationsDir:        getEnv("MIGRATIONS_DIR", "./db/migrations"),
		ServiceAuthPrincipal: strings.TrimSpace(getEnv("SERVICE_AUTH_PRINCIPAL", "castaway-discord-bot")),
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
	cfg.DiscordAdminUserIDs = parseCSV(getEnv("DISCORD_ADMIN_USER_IDS", ""))

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
