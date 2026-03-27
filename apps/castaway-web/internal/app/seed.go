package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
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
	gameplayService := gameplay.NewService(q)
	if err := gameplayService.CopyInstanceSchedule(ctx, instance.ID, seasonNumber); err != nil {
		return fmt.Errorf("copy episode schedule for season %d: %w", season.Season, err)
	}

	contestantIDByName := make(map[string]pgtype.UUID, len(season.Contestants))
	participantIDByName := make(map[string]pgtype.UUID, len(season.Participants))
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
		participantName := strings.TrimSpace(participantSeed.Name)
		participant, err := q.CreateParticipant(ctx, db.CreateParticipantParams{
			InstanceID: instance.ID,
			Name:       participantName,
		})
		if err != nil {
			return fmt.Errorf("create participant %q for season %d: %w", participantSeed.Name, season.Season, err)
		}
		participantIDByName[normalizeSeedName(participantName)] = participant.ID
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

	participantGroupIDByName, err := seedParticipantGroups(ctx, q, gameplayService, season, instance.ID, participantIDByName)
	if err != nil {
		return fmt.Errorf("seed participant groups for season %d: %w", season.Season, err)
	}

	if err := seedActivityHistory(ctx, q, gameplayService, season, instance.ID, participantIDByName, participantGroupIDByName); err != nil {
		return fmt.Errorf("seed activities for season %d: %w", season.Season, err)
	}

	if err := seedAdvantages(ctx, q, season, instance.ID, participantIDByName); err != nil {
		return fmt.Errorf("seed advantages for season %d: %w", season.Season, err)
	}

	result.Seasons++
	return nil
}

func seedParticipantGroups(
	ctx context.Context,
	q *db.Queries,
	gameplayService *gameplay.Service,
	season seeddata.SeasonSeed,
	instanceID pgtype.UUID,
	participantIDByName map[string]pgtype.UUID,
) (map[string]pgtype.UUID, error) {
	participantGroupIDByName := make(map[string]pgtype.UUID, len(season.ParticipantGroups))
	for _, groupSeed := range season.ParticipantGroups {
		kind := strings.TrimSpace(groupSeed.Kind)
		if kind == "" {
			kind = "tribe"
		}

		groupName := strings.TrimSpace(groupSeed.Name)
		group, err := q.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{
			InstanceID: instanceID,
			Name:       groupName,
			Kind:       kind,
			Metadata:   jsonBytesOrEmpty(groupSeed.Metadata),
		})
		if err != nil {
			return nil, fmt.Errorf("create participant group %q: %w", groupSeed.Name, err)
		}
		participantGroupIDByName[normalizeSeedName(groupName)] = group.ID

		for _, membershipSeed := range groupSeed.Memberships {
			participantID, ok := participantIDByName[normalizeSeedName(membershipSeed.ParticipantName)]
			if !ok {
				return nil, fmt.Errorf("resolve group membership participant %q for group %q", membershipSeed.ParticipantName, groupSeed.Name)
			}

			role := strings.TrimSpace(membershipSeed.Role)
			if role == "" {
				role = "member"
			}

			if _, err := gameplayService.CreateMembershipPeriod(ctx, gameplay.CreateMembershipPeriodParams{
				ParticipantGroupID: group.ID,
				ParticipantID:      participantID,
				Role:               role,
				StartsAt:           membershipSeed.StartsAt,
				EndsAt:             membershipSeed.EndsAt,
			}); err != nil {
				return nil, fmt.Errorf("create membership for %q in group %q: %w", membershipSeed.ParticipantName, groupSeed.Name, err)
			}
		}
	}

	return participantGroupIDByName, nil
}

func seedActivityHistory(
	ctx context.Context,
	q *db.Queries,
	gameplayService *gameplay.Service,
	season seeddata.SeasonSeed,
	instanceID pgtype.UUID,
	participantIDByName map[string]pgtype.UUID,
	participantGroupIDByName map[string]pgtype.UUID,
) error {
	for _, activitySeed := range season.Activities {
		if activitySeed.StartsAt.IsZero() {
			return fmt.Errorf("activity %q must include starts_at", activitySeed.Name)
		}

		status := strings.TrimSpace(activitySeed.Status)
		if status == "" {
			status = "active"
		}

		activity, err := q.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
			InstanceID:   instanceID,
			ActivityType: strings.TrimSpace(activitySeed.ActivityType),
			Name:         strings.TrimSpace(activitySeed.Name),
			Status:       status,
			StartsAt:     timestamptz(activitySeed.StartsAt),
			EndsAt:       optionalTimestamptz(activitySeed.EndsAt),
			Metadata:     jsonBytesOrEmpty(activitySeed.Metadata),
		})
		if err != nil {
			return fmt.Errorf("create activity %q: %w", activitySeed.Name, err)
		}

		for _, assignmentSeed := range activitySeed.GroupAssignments {
			groupID, ok := participantGroupIDByName[normalizeSeedName(assignmentSeed.ParticipantGroupName)]
			if !ok {
				return fmt.Errorf("resolve activity group assignment group %q for activity %q", assignmentSeed.ParticipantGroupName, activitySeed.Name)
			}

			role := strings.TrimSpace(assignmentSeed.Role)
			if role == "" {
				role = "group"
			}

			if _, err := gameplayService.CreateActivityGroupAssignment(ctx, gameplay.CreateActivityGroupAssignmentParams{
				ActivityID:         activity.ID,
				ParticipantGroupID: groupID,
				Role:               role,
				StartsAt:           assignmentSeed.StartsAt,
				EndsAt:             assignmentSeed.EndsAt,
				Configuration:      jsonBytesOrEmpty(assignmentSeed.Configuration),
			}); err != nil {
				return fmt.Errorf("create activity group assignment %q for activity %q: %w", assignmentSeed.ParticipantGroupName, activitySeed.Name, err)
			}
		}

		for _, assignmentSeed := range activitySeed.ParticipantAssignments {
			participantID, ok := participantIDByName[normalizeSeedName(assignmentSeed.ParticipantName)]
			if !ok {
				return fmt.Errorf("resolve activity participant assignment participant %q for activity %q", assignmentSeed.ParticipantName, activitySeed.Name)
			}

			participantGroupID := pgtype.UUID{}
			participantGroupSet := false
			if strings.TrimSpace(assignmentSeed.ParticipantGroupName) != "" {
				resolvedGroupID, ok := participantGroupIDByName[normalizeSeedName(assignmentSeed.ParticipantGroupName)]
				if !ok {
					return fmt.Errorf("resolve activity participant assignment group %q for activity %q", assignmentSeed.ParticipantGroupName, activitySeed.Name)
				}
				participantGroupID = resolvedGroupID
				participantGroupSet = true
			}

			role := strings.TrimSpace(assignmentSeed.Role)
			if role == "" {
				role = "participant"
			}

			if _, err := gameplayService.CreateParticipantAssignment(ctx, gameplay.CreateActivityParticipantAssignmentParams{
				ActivityID:          activity.ID,
				ParticipantID:       participantID,
				ParticipantGroupID:  participantGroupID,
				ParticipantGroupSet: participantGroupSet,
				Role:                role,
				StartsAt:            assignmentSeed.StartsAt,
				EndsAt:              assignmentSeed.EndsAt,
				Configuration:       jsonBytesOrEmpty(assignmentSeed.Configuration),
			}); err != nil {
				return fmt.Errorf("create activity participant assignment %q for activity %q: %w", assignmentSeed.ParticipantName, activitySeed.Name, err)
			}
		}

		for _, occurrenceSeed := range activitySeed.Occurrences {
			if occurrenceSeed.EffectiveAt.IsZero() {
				return fmt.Errorf("activity %q occurrence %q must include effective_at", activitySeed.Name, occurrenceSeed.Name)
			}

			occurrenceStatus := strings.TrimSpace(occurrenceSeed.Status)
			if occurrenceStatus == "" {
				occurrenceStatus = "recorded"
			}

			occurrence, err := q.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
				ActivityID:     activity.ID,
				OccurrenceType: strings.TrimSpace(occurrenceSeed.OccurrenceType),
				Name:           strings.TrimSpace(occurrenceSeed.Name),
				EffectiveAt:    timestamptz(occurrenceSeed.EffectiveAt),
				StartsAt:       optionalTimestamptz(occurrenceSeed.StartsAt),
				EndsAt:         optionalTimestamptz(occurrenceSeed.EndsAt),
				Status:         occurrenceStatus,
				SourceRef:      optionalText(occurrenceSeed.SourceRef),
				Metadata:       jsonBytesOrEmpty(occurrenceSeed.Metadata),
			})
			if err != nil {
				return fmt.Errorf("create occurrence %q for activity %q: %w", occurrenceSeed.Name, activitySeed.Name, err)
			}

			for _, participantSeed := range occurrenceSeed.Participants {
				participantID, ok := participantIDByName[normalizeSeedName(participantSeed.Name)]
				if !ok {
					return fmt.Errorf("resolve participant %q for occurrence %q", participantSeed.Name, occurrenceSeed.Name)
				}

				participantGroupID := pgtype.UUID{}
				if strings.TrimSpace(participantSeed.ParticipantGroupName) != "" {
					resolvedGroupID, ok := participantGroupIDByName[normalizeSeedName(participantSeed.ParticipantGroupName)]
					if !ok {
						return fmt.Errorf("resolve participant group %q for occurrence participant %q", participantSeed.ParticipantGroupName, participantSeed.Name)
					}
					participantGroupID = resolvedGroupID
				}

				role := strings.TrimSpace(participantSeed.Role)
				if role == "" {
					role = "participant"
				}

				if _, err := q.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
					ActivityOccurrenceID: occurrence.ID,
					ParticipantID:        participantID,
					ParticipantGroupID:   participantGroupID,
					Role:                 role,
					Result:               strings.TrimSpace(participantSeed.Result),
					Metadata:             jsonBytesOrEmpty(participantSeed.Metadata),
				}); err != nil {
					return fmt.Errorf("create occurrence participant %q for occurrence %q: %w", participantSeed.Name, occurrenceSeed.Name, err)
				}
			}

			if occurrenceSeed.Resolve {
				if _, err := gameplayService.ResolveActivityOccurrence(ctx, occurrence.ID); err != nil {
					return fmt.Errorf("resolve occurrence %q for activity %q: %w", occurrenceSeed.Name, activitySeed.Name, err)
				}
			}
		}
	}

	return nil
}

func seedAdvantages(
	ctx context.Context,
	q *db.Queries,
	season seeddata.SeasonSeed,
	instanceID pgtype.UUID,
	participantIDByName map[string]pgtype.UUID,
) error {
	for _, advantageSeed := range season.Advantages {
		participantID, ok := participantIDByName[normalizeSeedName(advantageSeed.ParticipantName)]
		if !ok {
			return fmt.Errorf("resolve advantage participant %q", advantageSeed.ParticipantName)
		}

		status := strings.TrimSpace(advantageSeed.Status)
		if status == "" {
			status = "active"
		}

		groupID := pgtype.UUID{}
		if advantageSeed.GroupName != "" {
			groups, err := q.ListParticipantGroupsByInstance(ctx, instanceID)
			if err != nil {
				return fmt.Errorf("list groups for advantage %q: %w", advantageSeed.Name, err)
			}
			for _, g := range groups {
				if strings.EqualFold(g.Name, strings.TrimSpace(advantageSeed.GroupName)) {
					groupID = g.ID
					break
				}
			}
			if !groupID.Valid {
				return fmt.Errorf("resolve advantage group %q for participant %q", advantageSeed.GroupName, advantageSeed.ParticipantName)
			}
		}

		if _, err := q.CreateParticipantAdvantage(ctx, db.CreateParticipantAdvantageParams{
			InstanceID:                 instanceID,
			ParticipantID:              participantID,
			ParticipantGroupID:         groupID,
			AdvantageType:              strings.TrimSpace(advantageSeed.AdvantageType),
			Name:                       strings.TrimSpace(advantageSeed.Name),
			Status:                     status,
			SourceActivityOccurrenceID: pgtype.UUID{},
			GrantedAt:                  timestamptz(advantageSeed.GrantedAt),
			EffectiveAt:                timestamptz(advantageSeed.EffectiveAt),
			EffectiveUntil:             optionalTimestamptz(advantageSeed.EffectiveUntil),
			Metadata:                   jsonBytesOrEmpty(advantageSeed.Metadata),
		}); err != nil {
			return fmt.Errorf("create advantage %q for participant %q: %w", advantageSeed.Name, advantageSeed.ParticipantName, err)
		}
	}

	return nil
}

func normalizeSeedName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func optionalText(value string) pgtype.Text {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func jsonBytesOrEmpty(value []byte) []byte {
	if len(value) == 0 {
		return []byte("{}")
	}
	return value
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return timestamptz(*value)
}

func SeedSummary(result SeedResult) string {
	return "seeded seasons=" + strconv.Itoa(result.Seasons) +
		" participants=" + strconv.Itoa(result.Participants) +
		" draft_picks=" + strconv.Itoa(result.DraftPicks) +
		" outcomes=" + strconv.Itoa(result.Outcomes)
}
