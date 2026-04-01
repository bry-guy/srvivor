package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/config"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("backfill-restore-secret-reveals: %v", err)
	}
}

type affectedParticipant struct {
	InstanceID           pgtype.UUID
	ParticipantID        pgtype.UUID
	ActivityOccurrenceID pgtype.UUID
	SourceGroupID        pgtype.UUID
	ParticipantName      string
	Reason               string
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
			log.Printf("backfill-restore-secret-reveals: rollback tx: %v", rollbackErr)
		}
	}()

	q := db.New(tx)
	affected, err := findAffectedParticipants(ctx, tx)
	if err != nil {
		return err
	}
	if len(affected) == 0 {
		fmt.Println("no premature secret reveals found")
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit empty tx: %w", err)
		}
		return nil
	}

	now := time.Now().UTC()
	secretMetadata, err := json.Marshal(map[string]any{
		"consumes_secret_balance":   true,
		"restores_premature_reveal": true,
	})
	if err != nil {
		return fmt.Errorf("marshal secret restore metadata: %w", err)
	}
	revealedMetadata, err := json.Marshal(map[string]any{
		"consumes_secret_balance":   false,
		"restores_premature_reveal": true,
	})
	if err != nil {
		return fmt.Errorf("marshal revealed restore metadata: %w", err)
	}

	for _, participant := range affected {
		participantID := participant.ParticipantID.String()
		reason := "Restore 1 secret bonus point after premature reveal"
		if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
			InstanceID:           participant.InstanceID,
			ParticipantID:        participant.ParticipantID,
			ActivityOccurrenceID: participant.ActivityOccurrenceID,
			SourceGroupID:        participant.SourceGroupID,
			EntryKind:            "correction",
			Points:               1,
			Visibility:           "secret",
			Reason:               reason,
			EffectiveAt:          optionalTime(now),
			AwardKey:             optionalText(fmt.Sprintf("secret:restore:credit:%s", participantID)),
			Metadata:             secretMetadata,
		}); err != nil {
			return fmt.Errorf("create secret restore for %s: %w", participant.ParticipantName, err)
		}
		if _, err := q.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
			InstanceID:           participant.InstanceID,
			ParticipantID:        participant.ParticipantID,
			ActivityOccurrenceID: participant.ActivityOccurrenceID,
			SourceGroupID:        participant.SourceGroupID,
			EntryKind:            "correction",
			Points:               -1,
			Visibility:           "revealed",
			Reason:               reason,
			EffectiveAt:          optionalTime(now),
			AwardKey:             optionalText(fmt.Sprintf("secret:restore:debit:%s", participantID)),
			Metadata:             revealedMetadata,
		}); err != nil {
			return fmt.Errorf("create revealed restore for %s: %w", participant.ParticipantName, err)
		}
		fmt.Printf("restored 1 secret point for %s (%s)\n", participant.ParticipantName, participant.Reason)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	fmt.Printf("restored premature reveals for %d participant(s)\n", len(affected))
	return nil
}

func findAffectedParticipants(ctx context.Context, tx pgx.Tx) ([]affectedParticipant, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT ON (p.public_id)
			i.public_id,
			p.public_id,
			ao.public_id,
			COALESCE(pg.public_id, '00000000-0000-0000-0000-000000000000'::uuid),
			p.name,
			bple.reason
		FROM bonus_point_ledger_entries bple
		JOIN participants p ON p.id = bple.participant_id
		JOIN instances i ON i.id = bple.instance_id
		JOIN activity_occurrences ao ON ao.id = bple.activity_occurrence_id
		LEFT JOIN participant_groups pg ON pg.id = bple.source_group_id
		WHERE bple.entry_kind = 'reveal'
		  AND bple.visibility = 'revealed'
		  AND bple.reason LIKE 'Revealed % secret bonus point(s) for %'
		  AND NOT EXISTS (
			SELECT 1
			FROM bonus_point_ledger_entries fix
			WHERE fix.participant_id = bple.participant_id
			  AND fix.award_key = ('secret:restore:credit:' || p.public_id::text)
		  )
		ORDER BY p.public_id, bple.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query affected participants: %w", err)
	}
	defer rows.Close()

	var affected []affectedParticipant
	for rows.Next() {
		var participant affectedParticipant
		if err := rows.Scan(&participant.InstanceID, &participant.ParticipantID, &participant.ActivityOccurrenceID, &participant.SourceGroupID, &participant.ParticipantName, &participant.Reason); err != nil {
			return nil, fmt.Errorf("scan affected participant: %w", err)
		}
		affected = append(affected, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate affected participants: %w", err)
	}
	return affected, nil
}

func optionalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}
