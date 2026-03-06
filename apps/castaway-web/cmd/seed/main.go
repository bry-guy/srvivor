package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/seeddata"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	if err := app.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	seedFile := getenv("SEED_FILE", "./seeds/historical-seasons.json")
	legacyDir := getenv("LEGACY_CLI_DIR", "../cli")

	seasons, err := seeddata.LoadFromJSON(seedFile)
	if err != nil {
		seasons, err = seeddata.LoadFromLegacy(legacyDir)
		if err != nil {
			return fmt.Errorf("load seed data from json or legacy: %w", err)
		}
	}

	result, err := app.SeedHistorical(ctx, pool, seasons)
	if err != nil {
		return fmt.Errorf("seed historical data: %w", err)
	}

	fmt.Println(app.SeedSummary(result))
	return nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
