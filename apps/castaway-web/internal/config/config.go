package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	AutoMigrate   bool
	MigrationsDir string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://castaway:castaway@localhost:5432/castaway?sslmode=disable"),
		MigrationsDir: getEnv("MIGRATIONS_DIR", "./db/migrations"),
	}

	autoMigrate, err := strconv.ParseBool(getEnv("AUTO_MIGRATE", "true"))
	if err != nil {
		return nil, fmt.Errorf("parse AUTO_MIGRATE: %w", err)
	}
	cfg.AutoMigrate = autoMigrate

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
