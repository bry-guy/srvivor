package gameplay

import (
	"context"
	"fmt"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type memberRow = db.ListActiveParticipantGroupMembershipsAtRow

const (
	// TribeChallengeActivityType records a Survivor tribe winning immunity or reward.
	TribeChallengeActivityType = "tribe_challenge"
	// WordleScoringIndividualAndTribeAverage is opted into through tribe_wordle activity metadata {"scoring": ...}.
	WordleScoringIndividualAndTribeAverage = "individual_and_tribe_average"
)

// TribeChallengePoints is the per-member award for each tribe challenge kind.
var TribeChallengePoints = map[string]int32{"immunity": 2, "reward": 1}

type tribeChallengeMetadata struct {
	WinningTribeIDs []string `json:"winning_tribe_ids"`
}

func (s *Service) resolveTribeChallenge(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	kind := resolverCtx.occurrence.OccurrenceType
	points, ok := TribeChallengePoints[kind]
	if !ok {
		return nil, fmt.Errorf("unsupported tribe challenge kind %q", kind)
	}
	var metadata tribeChallengeMetadata
	if err := parseJSON(resolverCtx.occurrence.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("parse tribe_challenge occurrence metadata: %w", err)
	}
	if len(metadata.WinningTribeIDs) == 0 {
		return nil, fmt.Errorf("tribe_challenge occurrence must include winning_tribe_ids")
	}

	cache := make(map[pgtype.UUID][]memberRow)
	entries := make([]resolvedLedgerEntry, 0)
	for _, rawID := range metadata.WinningTribeIDs {
		parsed, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("invalid winning tribe id %q", rawID)
		}
		groupID := pgtype.UUID{Bytes: parsed, Valid: true}
		group, err := s.queries.GetParticipantGroup(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("get tribe %s: %w", rawID, err)
		}
		if group.Kind != "tribe" || group.InstanceID != resolverCtx.activity.InstanceID {
			return nil, fmt.Errorf("group %s is not a tribe in this instance", rawID)
		}
		tribe := group.Name
		members, err := s.membersForGroupAt(ctx, cache, groupID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("list active members for tribe %q: %w", tribe, err)
		}
		if len(members) == 0 {
			return nil, fmt.Errorf("tribe %q has no members at %s", tribe, resolverCtx.occurrence.EffectiveAt.Time.UTC().Format("2006-01-02T15:04:05Z"))
		}
		for _, member := range members {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  groupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         points,
				Visibility:     bonusVisibilityPublic,
				Reason:         fmt.Sprintf("%s won %s", tribe, kind),
				AwardKey:       fmt.Sprintf("tribe_challenge:%s", pgUUIDString(groupID)),
			})
		}
	}
	return entries, nil
}

// resolveWordleIndividualAndTribeAverage awards +2 to the best individual guess count and +1 to every
// member of the tribe with the best average over its submitters. Ties share the award; non-submitters
// don't count toward averages.
func (s *Service) resolveWordleIndividualAndTribeAverage(ctx context.Context, resolverCtx resolverContext, guessCountsByGroup map[pgtype.UUID][]int, groupNames map[pgtype.UUID]string) ([]resolvedLedgerEntry, error) {
	entries := make([]resolvedLedgerEntry, 0)

	best := 0
	guessCounts := make([]int, len(resolverCtx.occurrenceParticipants))
	for i, row := range resolverCtx.occurrenceParticipants {
		guessCount, err := guessCountFromMetadata(row.Metadata)
		if err != nil {
			return nil, err
		}
		guessCounts[i] = guessCount
		if best == 0 || guessCount < best {
			best = guessCount
		}
	}
	for i, row := range resolverCtx.occurrenceParticipants {
		if guessCounts[i] != best {
			continue
		}
		entries = append(entries, resolvedLedgerEntry{
			ParticipantID:  row.ParticipantID,
			SourceGroupID:  row.ParticipantGroupID,
			HasSourceGroup: true,
			EntryKind:      bonusEntryKindAward,
			Points:         2,
			Visibility:     bonusVisibilityPublic,
			Reason:         "Best Wordle score",
			AwardKey:       "tribe_wordle:individual",
		})
	}

	var winners []wordleGroupScore
	for groupID, guessCounts := range guessCountsByGroup {
		score := wordleGroupScore{GroupID: groupID, GroupName: groupNames[groupID], Count: len(guessCounts)}
		for _, guessCount := range guessCounts {
			score.Total += guessCount
		}
		switch {
		case len(winners) == 0 || compareWordleScores(score, winners[0]) < 0:
			winners = []wordleGroupScore{score}
		case compareWordleScores(score, winners[0]) == 0:
			winners = append(winners, score)
		}
	}
	cache := make(map[pgtype.UUID][]memberRow)
	for _, winner := range winners {
		members, err := s.membersForGroupAt(ctx, cache, winner.GroupID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("list active members for winning wordle group %q: %w", winner.GroupName, err)
		}
		for _, member := range members {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  winner.GroupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         1,
				Visibility:     bonusVisibilityPublic,
				Reason:         fmt.Sprintf("%s had the best Wordle average", winner.GroupName),
				AwardKey:       fmt.Sprintf("tribe_wordle:%s", pgUUIDString(winner.GroupID)),
			})
		}
	}
	return entries, nil
}
