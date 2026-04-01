package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("backfill-season50-missing-tribal-pony: %v", err)
	}
}

type tribalPonyMetadata struct {
	WinningSurvivorTribes []string `json:"winning_survivor_tribes"`
}

type tribalPonyOccurrenceSpec struct {
	Name                  string
	EffectiveAt           time.Time
	WinningSurvivorTribes []string
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
			log.Printf("backfill-season50-missing-tribal-pony: rollback tx: %v", rollbackErr)
		}
	}()

	q := db.New(tx)
	svc := gameplay.NewService(q)

	instance, err := season50Instance(ctx, q)
	if err != nil {
		return err
	}
	activity, err := requireTribalPonyActivity(ctx, q, instance.ID)
	if err != nil {
		return err
	}

	for _, spec := range []tribalPonyOccurrenceSpec{
		{
			Name:                  "Episode 4 Immunity",
			EffectiveAt:           mustTime("2026-03-26T00:00:00Z"),
			WinningSurvivorTribes: []string{"leaf", "tangerine"},
		},
		{
			Name:                  "Episode 5 Immunity",
			EffectiveAt:           mustTime("2026-04-02T00:00:00Z"),
			WinningSurvivorTribes: []string{"leaf"},
		},
	} {
		if err := ensureResolvedTribalPonyOccurrence(ctx, q, svc, activity.ID, spec); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	fmt.Println("season 50 missing tribal pony backfill complete")
	return nil
}

func season50Instance(ctx context.Context, q *db.Queries) (db.ListInstancesRow, error) {
	instances, err := q.ListInstances(ctx)
	if err != nil {
		return db.ListInstancesRow{}, fmt.Errorf("list instances: %w", err)
	}
	matches := make([]db.ListInstancesRow, 0)
	for _, instance := range instances {
		if instance.Season == 50 {
			matches = append(matches, instance)
		}
	}
	if len(matches) != 1 {
		return db.ListInstancesRow{}, fmt.Errorf("expected exactly one season 50 instance, found %d", len(matches))
	}
	return matches[0], nil
}

func requireTribalPonyActivity(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (db.ListInstanceActivitiesByInstanceRow, error) {
	activities, err := q.ListInstanceActivitiesByInstance(ctx, instanceID)
	if err != nil {
		return db.ListInstanceActivitiesByInstanceRow{}, fmt.Errorf("list instance activities: %w", err)
	}
	for _, activity := range activities {
		if activity.ActivityType == "tribal_pony" && activity.Name == "Tribal Pony" {
			return activity, nil
		}
	}
	return db.ListInstanceActivitiesByInstanceRow{}, fmt.Errorf("season 50 tribal pony activity not found")
}

func ensureResolvedTribalPonyOccurrence(ctx context.Context, q *db.Queries, svc *gameplay.Service, activityID pgtype.UUID, spec tribalPonyOccurrenceSpec) error {
	occurrences, err := q.ListActivityOccurrencesByActivity(ctx, activityID)
	if err != nil {
		return fmt.Errorf("list activity occurrences for %q: %w", spec.Name, err)
	}

	expectedMetadata := tribalPonyMetadata{WinningSurvivorTribes: spec.WinningSurvivorTribes}
	for _, occurrence := range occurrences {
		if occurrence.Name != spec.Name {
			continue
		}
		if !occurrence.EffectiveAt.Valid || !occurrence.EffectiveAt.Time.Equal(spec.EffectiveAt) {
			return fmt.Errorf("existing %q has unexpected effective_at %v", spec.Name, occurrence.EffectiveAt.Time)
		}
		if err := validateTribalPonyMetadata(occurrence.Metadata, expectedMetadata); err != nil {
			return fmt.Errorf("existing %q metadata drift: %w", spec.Name, err)
		}
		if occurrence.Status == "resolved" {
			fmt.Printf("verified existing resolved occurrence %s\n", spec.Name)
			return nil
		}
		if _, err := svc.ResolveActivityOccurrence(ctx, occurrence.ID); err != nil {
			return fmt.Errorf("resolve existing occurrence %q: %w", spec.Name, err)
		}
		fmt.Printf("resolved existing occurrence %s\n", spec.Name)
		return nil
	}

	metadata, err := json.Marshal(expectedMetadata)
	if err != nil {
		return fmt.Errorf("marshal metadata for %q: %w", spec.Name, err)
	}
	occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID:     activityID,
		OccurrenceType: "immunity_result",
		Name:           spec.Name,
		EffectiveAt:    timestamptz(spec.EffectiveAt),
		Status:         "recorded",
		Metadata:       metadata,
	})
	if err != nil {
		return fmt.Errorf("create occurrence %q: %w", spec.Name, err)
	}
	if _, err := svc.ResolveActivityOccurrence(ctx, occurrence.ID); err != nil {
		return fmt.Errorf("resolve occurrence %q: %w", spec.Name, err)
	}
	fmt.Printf("created and resolved occurrence %s\n", spec.Name)
	return nil
}

func validateTribalPonyMetadata(raw []byte, expected tribalPonyMetadata) error {
	var actual tribalPonyMetadata
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}
	if len(actual.WinningSurvivorTribes) != len(expected.WinningSurvivorTribes) {
		return fmt.Errorf("expected %d winning tribes, got %d", len(expected.WinningSurvivorTribes), len(actual.WinningSurvivorTribes))
	}
	for index, tribe := range expected.WinningSurvivorTribes {
		if !strings.EqualFold(strings.TrimSpace(actual.WinningSurvivorTribes[index]), tribe) {
			if slices.ContainsFunc(actual.WinningSurvivorTribes, func(actualTribe string) bool {
				return strings.EqualFold(strings.TrimSpace(actualTribe), tribe)
			}) {
				continue
			}
			return fmt.Errorf("expected winning tribes %v, got %v", expected.WinningSurvivorTribes, actual.WinningSurvivorTribes)
		}
	}
	return nil
}

func mustTime(raw string) time.Time {
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return value
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
