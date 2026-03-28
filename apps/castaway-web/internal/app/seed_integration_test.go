package app

import (
	"context"
	"os"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/seeddata"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSeedHistoricalIntegration(t *testing.T) {
	databaseURL := os.Getenv("CASTAWAY_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set CASTAWAY_TEST_DATABASE_URL or run `mise run integration` to execute integration seed tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := RunMigrations(ctx, pool, "../../db/migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	seasons, err := seeddata.LoadFromJSON("../../seeds/historical-seasons.json")
	if err != nil {
		t.Fatalf("load seeds: %v", err)
	}

	result, err := SeedHistorical(ctx, pool, seasons)
	if err != nil {
		t.Fatalf("seed historical: %v", err)
	}
	if result.Seasons != len(seasons) {
		t.Fatalf("expected %d seeded seasons, got %d", len(seasons), result.Seasons)
	}

	instances, err := db.New(pool).ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(instances) < len(seasons) {
		t.Fatalf("expected at least %d instances, got %d", len(seasons), len(instances))
	}
}
