package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	if err := app.RunMigrations(context.Background(), pool, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Printf("castaway-web migrate %s", buildinfo.String())
	log.Printf("migrations applied")
	return nil
}
