package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/seeddata"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SeedResult struct {
	Seasons      int
	Participants int
	DraftPicks   int
	Outcomes     int
}

func SeedHistorical(ctx context.Context, pool *pgxpool.Pool, seasons []seeddata.SeasonSeed) (SeedResult, error) {
	result := SeedResult{}

	for _, seasonSeed := range seasons {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return result, fmt.Errorf("begin tx for season %d: %w", seasonSeed.Season, err)
		}
		if err := seedSeasonTx(ctx, tx, seasonSeed, &result); err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr != pgx.ErrTxClosed {
				return result, fmt.Errorf("rollback season %d: %w", seasonSeed.Season, rollbackErr)
			}
			return result, err
		}
		if err := tx.Commit(ctx); err != nil {
			return result, fmt.Errorf("commit season %d: %w", seasonSeed.Season, err)
		}
	}

	return result, nil
}

func seedSeasonTx(ctx context.Context, tx pgx.Tx, season seeddata.SeasonSeed, result *SeedResult) error {
	q := db.New(tx)
	seasonNumber, err := conv.ToInt32(season.Season)
	if err != nil {
		return fmt.Errorf("convert season number %d: %w", season.Season, err)
	}

	if err := q.DeleteInstanceByNameSeason(ctx, db.DeleteInstanceByNameSeasonParams{
		Name:   season.InstanceName,
		Season: seasonNumber,
	}); err != nil {
		return fmt.Errorf("delete existing instance for season %d: %w", season.Season, err)
	}

	instance, err := q.CreateInstance(ctx, db.CreateInstanceParams{
		Name:   season.InstanceName,
		Season: seasonNumber,
	})
	if err != nil {
		return fmt.Errorf("create instance for season %d: %w", season.Season, err)
	}

	contestantIDByName := make(map[string]pgtype.UUID, len(season.Contestants))
	for _, name := range season.Contestants {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		contestant, err := q.CreateContestant(ctx, db.CreateContestantParams{
			InstanceID: instance.ID,
			Name:       trimmed,
		})
		if err != nil {
			return fmt.Errorf("create contestant %q for season %d: %w", trimmed, season.Season, err)
		}
		contestantIDByName[strings.ToLower(trimmed)] = contestant.ID
	}

	for _, participantSeed := range season.Participants {
		participant, err := q.CreateParticipant(ctx, db.CreateParticipantParams{
			InstanceID: instance.ID,
			Name:       strings.TrimSpace(participantSeed.Name),
		})
		if err != nil {
			return fmt.Errorf("create participant %q for season %d: %w", participantSeed.Name, season.Season, err)
		}
		result.Participants++

		for index, contestantName := range participantSeed.Picks {
			trimmed := strings.TrimSpace(contestantName)
			contestantID, ok := contestantIDByName[strings.ToLower(trimmed)]
			if !ok && trimmed != "" {
				contestant, createErr := q.CreateContestant(ctx, db.CreateContestantParams{
					InstanceID: instance.ID,
					Name:       trimmed,
				})
				if createErr != nil {
					return fmt.Errorf("create missing contestant %q for season %d: %w", trimmed, season.Season, createErr)
				}
				contestantID = contestant.ID
				contestantIDByName[strings.ToLower(trimmed)] = contestant.ID
			}
			if !ok && trimmed == "" {
				continue
			}

			position, err := conv.ToInt32(index + 1)
			if err != nil {
				return fmt.Errorf("draft pick position for season %d: %w", season.Season, err)
			}

			if _, err := q.CreateDraftPick(ctx, db.CreateDraftPickParams{
				InstanceID:    instance.ID,
				ParticipantID: participant.ID,
				ContestantID:  contestantID,
				Position:      position,
			}); err != nil {
				return fmt.Errorf("create draft pick participant %q season %d: %w", participantSeed.Name, season.Season, err)
			}
			result.DraftPicks++
		}
	}

	for _, outcome := range season.Outcomes {
		position, err := conv.ToInt32(outcome.Position)
		if err != nil {
			return fmt.Errorf("outcome position for season %d: %w", season.Season, err)
		}

		contestantParam := pgtype.UUID{Valid: false}
		trimmed := strings.TrimSpace(outcome.ContestantName)
		if trimmed != "" {
			contestantID, ok := contestantIDByName[strings.ToLower(trimmed)]
			if !ok {
				contestant, createErr := q.CreateContestant(ctx, db.CreateContestantParams{
					InstanceID: instance.ID,
					Name:       trimmed,
				})
				if createErr != nil {
					return fmt.Errorf("create missing outcome contestant %q for season %d: %w", trimmed, season.Season, createErr)
				}
				contestantID = contestant.ID
				contestantIDByName[strings.ToLower(trimmed)] = contestant.ID
			}
			contestantParam = contestantID
		}

		if _, err := q.UpsertOutcomePosition(ctx, db.UpsertOutcomePositionParams{
			InstanceID:   instance.ID,
			Position:     position,
			ContestantID: contestantParam,
		}); err != nil {
			return fmt.Errorf("upsert outcome season %d position %d: %w", season.Season, outcome.Position, err)
		}
		result.Outcomes++
	}

	result.Seasons++
	return nil
}

func SeedSummary(result SeedResult) string {
	return "seeded seasons=" + strconv.Itoa(result.Seasons) +
		" participants=" + strconv.Itoa(result.Participants) +
		" draft_picks=" + strconv.Itoa(result.DraftPicks) +
		" outcomes=" + strconv.Itoa(result.Outcomes)
}
