package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
		log.Fatalf("backfill-season50-wordle-tie: %v", err)
	}
}

type wordleParticipantSpec struct {
	Name       string
	GroupName  string
	GuessCount int
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
			log.Printf("backfill-season50-wordle-tie: rollback tx: %v", rollbackErr)
		}
	}()

	q := db.New(tx)
	svc := gameplay.NewService(q)

	instance, err := season50Instance(ctx, q)
	if err != nil {
		return err
	}
	participantByName, err := participantMap(ctx, q, instance.ID)
	if err != nil {
		return err
	}
	groupByName, err := groupMap(ctx, q, instance.ID)
	if err != nil {
		return err
	}
	occurrence, err := requireWeek2WordleOccurrence(ctx, q, instance.ID)
	if err != nil {
		return err
	}

	for _, spec := range []wordleParticipantSpec{
		{Name: "Adam", GroupName: "Tangerine", GuessCount: 2},
		{Name: "Grant", GroupName: "Tangerine", GuessCount: 2},
		{Name: "Kyle", GroupName: "Tangerine", GuessCount: 2},
	} {
		participantID, ok := participantByName[normalize(spec.Name)]
		if !ok {
			return fmt.Errorf("resolve participant %q", spec.Name)
		}
		group, ok := groupByName[normalize(spec.GroupName)]
		if !ok {
			return fmt.Errorf("resolve participant group %q", spec.GroupName)
		}
		metadata, err := json.Marshal(map[string]int{"guess_count": spec.GuessCount})
		if err != nil {
			return fmt.Errorf("marshal wordle metadata for %q: %w", spec.Name, err)
		}
		if _, err := q.UpsertActivityOccurrenceParticipant(ctx, db.UpsertActivityOccurrenceParticipantParams{
			ActivityOccurrenceID: occurrence.ID,
			ParticipantID:        participantID,
			ParticipantGroupID:   group.ID,
			Role:                 "player",
			Result:               "",
			Metadata:             metadata,
		}); err != nil {
			return fmt.Errorf("upsert wordle participant %q: %w", spec.Name, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM bonus_point_ledger_entries b
		USING activity_occurrences ao
		WHERE ao.public_id = $1
		  AND b.activity_occurrence_id = ao.id
	`, occurrence.ID); err != nil {
		return fmt.Errorf("delete stale wordle ledger entries: %w", err)
	}
	if _, err := q.UpdateActivityOccurrenceStatusAndMetadata(ctx, db.UpdateActivityOccurrenceStatusAndMetadataParams{
		ID:       occurrence.ID,
		Status:   "recorded",
		EndsAt:   pgtype.Timestamptz{},
		Metadata: occurrence.Metadata,
	}); err != nil {
		return fmt.Errorf("reset wordle occurrence status: %w", err)
	}
	if _, err := svc.ResolveActivityOccurrence(ctx, occurrence.ID); err != nil {
		return fmt.Errorf("re-resolve wordle occurrence: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	fmt.Println("season 50 wordle tie backfill complete")
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

func participantMap(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (map[string]pgtype.UUID, error) {
	participants, err := q.ListParticipantsByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	result := make(map[string]pgtype.UUID, len(participants))
	for _, participant := range participants {
		result[normalize(participant.Name)] = participant.ID
	}
	return result, nil
}

func groupMap(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (map[string]db.ListParticipantGroupsByInstanceRow, error) {
	groups, err := q.ListParticipantGroupsByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("list participant groups: %w", err)
	}
	result := make(map[string]db.ListParticipantGroupsByInstanceRow, len(groups))
	for _, group := range groups {
		result[normalize(group.Name)] = group
	}
	return result, nil
}

func requireWeek2WordleOccurrence(ctx context.Context, q *db.Queries, instanceID pgtype.UUID) (db.ListActivityOccurrencesByActivityRow, error) {
	activities, err := q.ListInstanceActivitiesByInstance(ctx, instanceID)
	if err != nil {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("list instance activities: %w", err)
	}
	var activityID pgtype.UUID
	found := false
	for _, activity := range activities {
		if activity.ActivityType == "tribe_wordle" && activity.Name == "Tribe Wordle" {
			activityID = activity.ID
			found = true
			break
		}
	}
	if !found {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("season 50 tribe wordle activity not found")
	}
	occurrences, err := q.ListActivityOccurrencesByActivity(ctx, activityID)
	if err != nil {
		return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("list wordle occurrences: %w", err)
	}
	for _, occurrence := range occurrences {
		if occurrence.Name == "Week 2 Tribe Wordle" {
			return occurrence, nil
		}
	}
	return db.ListActivityOccurrencesByActivityRow{}, fmt.Errorf("season 50 Week 2 Tribe Wordle occurrence not found")
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
