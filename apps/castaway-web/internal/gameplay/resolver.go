package gameplay

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/conv"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	bonusVisibilityPublic   = "public"
	bonusVisibilitySecret   = "secret"
	bonusVisibilityRevealed = "revealed"

	bonusEntryKindAward      = "award"
	bonusEntryKindCorrection = "correction"
	bonusEntryKindSpend      = "spend"
)

type resolvedLedgerEntry struct {
	ParticipantID  pgtype.UUID
	SourceGroupID  pgtype.UUID
	HasSourceGroup bool
	EntryKind      string
	Points         int32
	Visibility     string
	Reason         string
	AwardKey       string
	Metadata       []byte
}

type resolverContext struct {
	activity               db.GetInstanceActivityRow
	occurrence             db.GetActivityOccurrenceRow
	occurrenceGroups       []db.ListActivityOccurrenceGroupsRow
	occurrenceParticipants []db.ListActivityOccurrenceParticipantsRow
}

type tribalPonyOccurrenceMetadata struct {
	WinningSurvivorTribes []string `json:"winning_survivor_tribes"`
}

type tribalPonyAssignmentConfiguration struct {
	PonySurvivorTribe string `json:"pony_survivor_tribe"`
}

type wordleParticipantMetadata struct {
	GuessCount int `json:"guess_count"`
}

type journeyChoiceMetadata struct {
	Choice string `json:"choice"`
}

type manualAdjustmentParticipantMetadata struct {
	Points     int32  `json:"points"`
	Visibility string `json:"visibility"`
	Reason     string `json:"reason"`
	EntryKind  string `json:"entry_kind"`
	AwardKey   string `json:"award_key"`
	Metadata   []byte `json:"metadata"`
}

type stirThePotContributionMetadata struct {
	Contribution int32 `json:"contribution"`
}

type stirThePotRewardTier struct {
	Contributions int32 `json:"contributions"`
	Bonus         int32 `json:"bonus"`
}

type mergeTargetEpisodeMetadata struct {
	EpisodeID     string `json:"episode_id,omitempty"`
	EpisodeNumber int32  `json:"episode_number,omitempty"`
	EpisodeLabel  string `json:"episode_label,omitempty"`
	EpisodeAirsAt string `json:"episode_airs_at,omitempty"`
}

type stirThePotRoundMetadata struct {
	RewardTiers   []stirThePotRewardTier     `json:"reward_tiers,omitempty"`
	TargetEpisode mergeTargetEpisodeMetadata `json:"target_episode,omitempty"`
	ResolvedBy    string                     `json:"resolved_by,omitempty"`
	ResolvedAt    string                     `json:"resolved_at,omitempty"`
}

type individualPonyOccurrenceMetadata struct {
	WinningContestantID string `json:"winning_contestant_id"`
}

type wordleGroupScore struct {
	GroupID   pgtype.UUID
	GroupName string
	Total     int
	Count     int
}

func (s *Service) ResolveActivityOccurrence(ctx context.Context, occurrenceID pgtype.UUID) ([]db.CreateBonusPointLedgerEntryRow, error) {
	occurrence, err := s.queries.GetActivityOccurrence(ctx, occurrenceID)
	if err != nil {
		return nil, fmt.Errorf("get activity occurrence: %w", err)
	}

	activity, err := s.queries.GetInstanceActivity(ctx, occurrence.ActivityID)
	if err != nil {
		return nil, fmt.Errorf("get instance activity: %w", err)
	}

	occurrenceGroups, err := s.queries.ListActivityOccurrenceGroups(ctx, occurrence.ID)
	if err != nil {
		return nil, fmt.Errorf("list occurrence groups: %w", err)
	}
	occurrenceParticipants, err := s.queries.ListActivityOccurrenceParticipants(ctx, occurrence.ID)
	if err != nil {
		return nil, fmt.Errorf("list occurrence participants: %w", err)
	}

	resolverCtx := resolverContext{
		activity:               activity,
		occurrence:             occurrence,
		occurrenceGroups:       occurrenceGroups,
		occurrenceParticipants: occurrenceParticipants,
	}

	var entries []resolvedLedgerEntry
	switch activity.ActivityType {
	case "tribal_pony":
		entries, err = s.resolveTribalPony(ctx, resolverCtx)
	case "tribe_wordle":
		entries, err = s.resolveTribeWordle(ctx, resolverCtx)
	case "journey":
		entries, err = s.resolveJourney(ctx, resolverCtx)
	case "manual_adjustment":
		entries, err = s.resolveManualAdjustment(resolverCtx)
	case "stir_the_pot":
		entries, err = s.resolveStirThePot(ctx, resolverCtx)
	case "individual_pony":
		entries, err = s.resolveIndividualPony(ctx, resolverCtx)
	default:
		return nil, fmt.Errorf("unsupported activity type %q", activity.ActivityType)
	}
	if err != nil {
		return nil, err
	}

	created := make([]db.CreateBonusPointLedgerEntryRow, 0, len(entries))
	for _, entry := range entries {
		if entry.Points == 0 {
			continue
		}

		sourceGroupID := pgtype.UUID{}
		if entry.HasSourceGroup {
			sourceGroupID = entry.SourceGroupID
		}

		createdEntry, err := s.queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
			InstanceID:           activity.InstanceID,
			ParticipantID:        entry.ParticipantID,
			ActivityOccurrenceID: occurrence.ID,
			SourceGroupID:        sourceGroupID,
			EntryKind:            entry.EntryKind,
			Points:               entry.Points,
			Visibility:           entry.Visibility,
			Reason:               entry.Reason,
			EffectiveAt:          occurrence.EffectiveAt,
			AwardKey:             optionalText(entry.AwardKey),
			Metadata:             jsonbOrEmpty(entry.Metadata),
		})
		if err != nil {
			return nil, fmt.Errorf("create bonus ledger entry %q for participant %s: %w", entry.AwardKey, pgUUIDString(entry.ParticipantID), err)
		}
		created = append(created, createdEntry)
	}

	return created, nil
}

func (s *Service) resolveTribalPony(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	var metadata tribalPonyOccurrenceMetadata
	if err := parseJSON(resolverCtx.occurrence.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("parse tribal_pony occurrence metadata: %w", err)
	}
	winningTribes := make(map[string]struct{}, len(metadata.WinningSurvivorTribes))
	for _, tribe := range metadata.WinningSurvivorTribes {
		if normalized := normalizeKey(tribe); normalized != "" {
			winningTribes[normalized] = struct{}{}
		}
	}
	if len(winningTribes) == 0 {
		return nil, fmt.Errorf("tribal_pony occurrence must include winning_survivor_tribes")
	}

	assignments, err := s.ActiveActivityGroupAssignmentsAt(ctx, resolverCtx.activity.ID, resolverCtx.occurrence.EffectiveAt.Time)
	if err != nil {
		return nil, fmt.Errorf("list active activity group assignments: %w", err)
	}
	stirThePotBonusByGroup, err := s.stirThePotBonusesForInstance(ctx, resolverCtx.activity.InstanceID, resolverCtx.occurrence.EffectiveAt.Time)
	if err != nil {
		return nil, err
	}

	entries := make([]resolvedLedgerEntry, 0)
	membershipCache := make(map[pgtype.UUID][]db.ListActiveParticipantGroupMembershipsAtRow)
	for _, assignment := range assignments {
		var configuration tribalPonyAssignmentConfiguration
		if err := parseJSON(assignment.Configuration, &configuration); err != nil {
			return nil, fmt.Errorf("parse tribal_pony assignment configuration for group %q: %w", assignment.ParticipantGroupName, err)
		}
		if _, ok := winningTribes[normalizeKey(configuration.PonySurvivorTribe)]; !ok {
			continue
		}

		members, err := s.membersForGroupAt(ctx, membershipCache, assignment.ParticipantGroupID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("list active members for group %q: %w", assignment.ParticipantGroupName, err)
		}
		points := int32(1)
		if bonus, ok := stirThePotBonusByGroup[assignment.ParticipantGroupID]; ok && bonus > 0 {
			points += bonus
		}
		for _, member := range members {
			reason := fmt.Sprintf("%s pony tribe won immunity", assignment.ParticipantGroupName)
			awardKey := fmt.Sprintf("tribal_pony:%s", pgUUIDString(assignment.ParticipantGroupID))
			if points > 1 {
				reason = fmt.Sprintf("%s pony tribe won immunity with Stir the Pot bonus", assignment.ParticipantGroupName)
				awardKey = fmt.Sprintf("tribal_pony:%s:stir_the_pot", pgUUIDString(assignment.ParticipantGroupID))
			}
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  assignment.ParticipantGroupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         points,
				Visibility:     bonusVisibilityPublic,
				Reason:         reason,
				AwardKey:       awardKey,
			})
		}
	}

	return entries, nil
}

func (s *Service) resolveTribeWordle(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	participantRows := resolverCtx.occurrenceParticipants
	if len(participantRows) == 0 {
		return nil, fmt.Errorf("tribe_wordle occurrence must include participant results")
	}

	guessCountsByGroup := make(map[pgtype.UUID][]int)
	groupNames := make(map[pgtype.UUID]string)
	for _, participantRow := range participantRows {
		if !participantRow.ParticipantGroupID.Valid {
			return nil, fmt.Errorf("tribe_wordle occurrence participant %q must include participant_group_id", participantRow.ParticipantName)
		}

		guessCount, err := guessCountFromMetadata(participantRow.Metadata)
		if err != nil {
			return nil, fmt.Errorf("parse tribe_wordle guess count for participant %q: %w", participantRow.ParticipantName, err)
		}
		guessCountsByGroup[participantRow.ParticipantGroupID] = append(guessCountsByGroup[participantRow.ParticipantGroupID], guessCount)
		groupNames[participantRow.ParticipantGroupID] = participantRow.ParticipantGroupName.String
	}

	groupScores := make([]wordleGroupScore, 0, len(guessCountsByGroup))
	for groupID, guessCounts := range guessCountsByGroup {
		sort.Ints(guessCounts)
		topCount := minInt(len(guessCounts), 3)
		if topCount == 0 {
			continue
		}
		total := 0
		for _, guessCount := range guessCounts[:topCount] {
			total += guessCount
		}
		groupScores = append(groupScores, wordleGroupScore{
			GroupID:   groupID,
			GroupName: groupNames[groupID],
			Total:     total,
			Count:     topCount,
		})
	}
	if len(groupScores) == 0 {
		return nil, nil
	}

	winningGroups := make([]wordleGroupScore, 0, len(groupScores))
	for _, score := range groupScores {
		if len(winningGroups) == 0 {
			winningGroups = append(winningGroups, score)
			continue
		}
		comparison := compareWordleScores(score, winningGroups[0])
		if comparison < 0 {
			winningGroups = []wordleGroupScore{score}
			continue
		}
		if comparison == 0 {
			winningGroups = append(winningGroups, score)
		}
	}

	membershipCache := make(map[pgtype.UUID][]db.ListActiveParticipantGroupMembershipsAtRow)
	entries := make([]resolvedLedgerEntry, 0)
	for _, winningGroup := range winningGroups {
		members, err := s.membersForGroupAt(ctx, membershipCache, winningGroup.GroupID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("list active members for winning wordle group %q: %w", winningGroup.GroupName, err)
		}
		for _, member := range members {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  winningGroup.GroupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         1,
				Visibility:     bonusVisibilityPublic,
				Reason:         fmt.Sprintf("%s won the tribe Wordle challenge", winningGroup.GroupName),
				AwardKey:       fmt.Sprintf("tribe_wordle:%s", pgUUIDString(winningGroup.GroupID)),
			})
		}
	}

	return entries, nil
}

func (s *Service) resolveJourney(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	switch resolverCtx.occurrence.OccurrenceType {
	case "journey_attendance":
		return s.resolveJourneyAttendance(ctx, resolverCtx)
	case "journey_resolution":
		return s.resolveJourneyDiplomacy(ctx, resolverCtx)
	case "secret_risk_result", "lost_for_words":
		return s.resolveJourneySecretRisk(ctx, resolverCtx)
	default:
		return nil, fmt.Errorf("unsupported journey occurrence type %q", resolverCtx.occurrence.OccurrenceType)
	}
}

func (s *Service) resolveJourneyAttendance(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	delegates, err := s.resolveJourneyDelegatesAt(ctx, resolverCtx)
	if err != nil {
		return nil, err
	}

	entries := make([]resolvedLedgerEntry, 0, len(delegates))
	for _, delegate := range delegates {
		entries = append(entries, resolvedLedgerEntry{
			ParticipantID: delegate.ParticipantID,
			EntryKind:     bonusEntryKindAward,
			Points:        1,
			Visibility:    bonusVisibilityPublic,
			Reason:        fmt.Sprintf("%s attendance bonus", resolverCtx.occurrence.Name),
			AwardKey:      "journey:attendance",
		})
	}
	return entries, nil
}

func (s *Service) resolveJourneyDiplomacy(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	participantAssignments, err := s.ActiveActivityParticipantAssignmentsAt(ctx, resolverCtx.activity.ID, resolverCtx.occurrence.EffectiveAt.Time)
	if err != nil {
		return nil, fmt.Errorf("list active journey participant assignments: %w", err)
	}
	assignmentGroups := make(map[pgtype.UUID]pgtype.UUID, len(participantAssignments))
	for _, assignment := range participantAssignments {
		if assignment.ParticipantGroupID.Valid {
			assignmentGroups[assignment.ParticipantID] = assignment.ParticipantGroupID
		}
	}

	type delegateChoice struct {
		participantID pgtype.UUID
		groupID       pgtype.UUID
		choice        string
	}

	choices := make([]delegateChoice, 0, len(resolverCtx.occurrenceParticipants))
	for _, participantRow := range resolverCtx.occurrenceParticipants {
		choice, err := journeyChoiceForParticipant(participantRow)
		if err != nil {
			return nil, fmt.Errorf("parse journey choice for participant %q: %w", participantRow.ParticipantName, err)
		}
		if choice == "" {
			continue
		}

		groupID := participantRow.ParticipantGroupID
		if !groupID.Valid {
			if assignedGroupID, ok := assignmentGroups[participantRow.ParticipantID]; ok {
				groupID = assignedGroupID
			}
		}
		if !groupID.Valid {
			return nil, fmt.Errorf("journey participant %q must resolve to a tribe", participantRow.ParticipantName)
		}

		choices = append(choices, delegateChoice{
			participantID: participantRow.ParticipantID,
			groupID:       groupID,
			choice:        choice,
		})
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("journey_resolution occurrence must include SHARE/STEAL participant choices")
	}

	stealers := make([]delegateChoice, 0)
	sharers := make([]delegateChoice, 0)
	for _, choice := range choices {
		switch choice.choice {
		case "SHARE":
			sharers = append(sharers, choice)
		case "STEAL":
			stealers = append(stealers, choice)
		default:
			return nil, fmt.Errorf("unsupported journey choice %q", choice.choice)
		}
	}

	tribeAwards := make(map[pgtype.UUID]int32)
	switch {
	case len(stealers) == 0:
		for _, sharer := range sharers {
			tribeAwards[sharer.groupID] = 1
		}
	case len(stealers) == 1:
		tribeAwards[stealers[0].groupID] = 3
	default:
		for _, sharer := range sharers {
			tribeAwards[sharer.groupID] = 1
		}
	}

	membershipCache := make(map[pgtype.UUID][]db.ListActiveParticipantGroupMembershipsAtRow)
	entries := make([]resolvedLedgerEntry, 0)
	for groupID, points := range tribeAwards {
		members, err := s.membersForGroupAt(ctx, membershipCache, groupID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("list active members for journey tribe %s: %w", pgUUIDString(groupID), err)
		}
		for _, member := range members {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  groupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         points,
				Visibility:     bonusVisibilityPublic,
				Reason:         fmt.Sprintf("%s Tribal Diplomacy result", resolverCtx.occurrence.Name),
				AwardKey:       fmt.Sprintf("journey:diplomacy:%s", pgUUIDString(groupID)),
			})
		}
	}

	return entries, nil
}

func (s *Service) resolveJourneySecretRisk(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	if len(resolverCtx.occurrenceParticipants) == 0 {
		return nil, fmt.Errorf("secret risk occurrence must include participant results")
	}

	entries := make([]resolvedLedgerEntry, 0, len(resolverCtx.occurrenceParticipants)*3)
	for _, participantRow := range resolverCtx.occurrenceParticipants {
		guessCount, err := guessCountFromMetadata(participantRow.Metadata)
		if err != nil {
			return nil, fmt.Errorf("parse secret risk guess count for participant %q: %w", participantRow.ParticipantName, err)
		}
		guessCountInt32, err := conv.ToInt32(guessCount)
		if err != nil {
			return nil, fmt.Errorf("convert secret risk guess count for participant %q: %w", participantRow.ParticipantName, err)
		}
		secretBalance, err := s.AvailableSecretBalanceByParticipant(ctx, resolverCtx.activity.InstanceID, participantRow.ParticipantID)
		if err != nil {
			return nil, fmt.Errorf("get available secret balance for participant %q: %w", participantRow.ParticipantName, err)
		}
		visibleBalance, err := s.VisibleBonusTotalByParticipantAsOf(ctx, resolverCtx.activity.InstanceID, participantRow.ParticipantID, resolverCtx.occurrence.EffectiveAt.Time)
		if err != nil {
			return nil, fmt.Errorf("get visible bonus balance for participant %q: %w", participantRow.ParticipantName, err)
		}
		if visibleBalance < 0 {
			visibleBalance = 0
		}

		const secretAward int32 = 3
		secretSpend := minInt32(guessCountInt32, secretBalance+secretAward)
		remainingSpend := guessCountInt32 - secretSpend
		publicSpend := minInt32(remainingSpend, visibleBalance)

		entries = append(entries, resolvedLedgerEntry{
			ParticipantID: participantRow.ParticipantID,
			EntryKind:     bonusEntryKindAward,
			Points:        secretAward,
			Visibility:    bonusVisibilitySecret,
			Reason:        fmt.Sprintf("%s secret bonus award", resolverCtx.occurrence.Name),
			AwardKey:      "journey:secret_risk:award",
		})
		if secretSpend > 0 {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID: participantRow.ParticipantID,
				EntryKind:     bonusEntryKindSpend,
				Points:        -secretSpend,
				Visibility:    bonusVisibilitySecret,
				Reason:        fmt.Sprintf("%s secret guess penalty", resolverCtx.occurrence.Name),
				AwardKey:      "journey:secret_risk:spend_secret",
			})
		}
		if publicSpend > 0 {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID: participantRow.ParticipantID,
				EntryKind:     bonusEntryKindSpend,
				Points:        -publicSpend,
				Visibility:    bonusVisibilityPublic,
				Reason:        fmt.Sprintf("%s public guess penalty", resolverCtx.occurrence.Name),
				AwardKey:      "journey:secret_risk:spend_public",
			})
		}
	}

	return entries, nil
}

func (s *Service) resolveManualAdjustment(resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	if len(resolverCtx.occurrenceParticipants) == 0 {
		return nil, fmt.Errorf("manual adjustment occurrence must include participant adjustments")
	}

	entries := make([]resolvedLedgerEntry, 0, len(resolverCtx.occurrenceParticipants))
	for _, participantRow := range resolverCtx.occurrenceParticipants {
		var adjustment manualAdjustmentParticipantMetadata
		if err := parseJSON(participantRow.Metadata, &adjustment); err != nil {
			return nil, fmt.Errorf("parse manual adjustment metadata for participant %q: %w", participantRow.ParticipantName, err)
		}
		if adjustment.Points == 0 {
			return nil, fmt.Errorf("manual adjustment for participant %q must include non-zero points", participantRow.ParticipantName)
		}

		visibility := adjustment.Visibility
		if visibility == "" {
			visibility = bonusVisibilityPublic
		}
		if err := validateBonusVisibility(visibility); err != nil {
			return nil, fmt.Errorf("participant %q: %w", participantRow.ParticipantName, err)
		}

		entryKind := adjustment.EntryKind
		if entryKind == "" {
			entryKind = bonusEntryKindCorrection
		}

		reason := adjustment.Reason
		if reason == "" {
			reason = resolverCtx.occurrence.Name
		}

		entry := resolvedLedgerEntry{
			ParticipantID: participantRow.ParticipantID,
			EntryKind:     entryKind,
			Points:        adjustment.Points,
			Visibility:    visibility,
			Reason:        reason,
			AwardKey:      adjustment.AwardKey,
			Metadata:      participantRow.Metadata,
		}
		if entry.AwardKey == "" {
			entry.AwardKey = fmt.Sprintf("manual_adjustment:%d", participantRow.ID)
		}
		if participantRow.ParticipantGroupID.Valid {
			entry.SourceGroupID = participantRow.ParticipantGroupID
			entry.HasSourceGroup = true
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (s *Service) resolveStirThePot(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	if len(resolverCtx.occurrenceParticipants) == 0 {
		return nil, fmt.Errorf("stir_the_pot occurrence must include participant contributions")
	}

	// Group contributions by participant group (tribe).
	type groupContribution struct {
		groupID   pgtype.UUID
		groupName string
		total     int32
		members   []pgtype.UUID // participant IDs that contributed
	}
	contributionsByGroup := make(map[pgtype.UUID]*groupContribution)

	for _, participantRow := range resolverCtx.occurrenceParticipants {
		if !participantRow.ParticipantGroupID.Valid {
			return nil, fmt.Errorf("stir_the_pot participant %q must belong to a tribe", participantRow.ParticipantName)
		}

		var contribution stirThePotContributionMetadata
		if err := parseJSON(participantRow.Metadata, &contribution); err != nil {
			return nil, fmt.Errorf("parse stir_the_pot contribution for participant %q: %w", participantRow.ParticipantName, err)
		}
		if contribution.Contribution < 0 {
			return nil, fmt.Errorf("stir_the_pot contribution for participant %q must be non-negative", participantRow.ParticipantName)
		}

		gc, ok := contributionsByGroup[participantRow.ParticipantGroupID]
		if !ok {
			gc = &groupContribution{
				groupID:   participantRow.ParticipantGroupID,
				groupName: participantRow.ParticipantGroupName.String,
			}
			contributionsByGroup[participantRow.ParticipantGroupID] = gc
		}
		gc.total += contribution.Contribution
		gc.members = append(gc.members, participantRow.ParticipantID)
	}

	membershipCache := make(map[pgtype.UUID][]db.ListActiveParticipantGroupMembershipsAtRow)
	entries := make([]resolvedLedgerEntry, 0)

	for _, gc := range contributionsByGroup {
		// Check if this tribe has the Stir the Pot Advantage.
		costPerPoint := int32(4)
		advantages, err := s.queries.ListActiveAdvantagesByTypeForGroup(ctx, db.ListActiveAdvantagesByTypeForGroupParams{
			InstanceID:         resolverCtx.activity.InstanceID,
			ParticipantGroupID: gc.groupID,
			AdvantageType:      "stir_the_pot_advantage",
			At:                 resolverCtx.occurrence.EffectiveAt,
		})
		if err != nil {
			return nil, fmt.Errorf("check stir_the_pot advantages for group %q: %w", gc.groupName, err)
		}
		if len(advantages) > 0 {
			costPerPoint = 3
		}

		// Calculate reward: integer division of total contributions by cost per point.
		rewardPerMember := gc.total / costPerPoint

		// Spend entries for each contributor.
		for _, participantRow := range resolverCtx.occurrenceParticipants {
			if participantRow.ParticipantGroupID != gc.groupID {
				continue
			}

			var contribution stirThePotContributionMetadata
			if err := parseJSON(participantRow.Metadata, &contribution); err != nil {
				return nil, fmt.Errorf("re-parse stir_the_pot contribution for participant spend: %w", err)
			}

			if contribution.Contribution > 0 {
				entries = append(entries, resolvedLedgerEntry{
					ParticipantID:  participantRow.ParticipantID,
					SourceGroupID:  gc.groupID,
					HasSourceGroup: true,
					EntryKind:      bonusEntryKindSpend,
					Points:         -contribution.Contribution,
					Visibility:     bonusVisibilityPublic,
					Reason:         fmt.Sprintf("%s Stir the Pot contribution", gc.groupName),
					AwardKey:       fmt.Sprintf("stir_the_pot:spend:%s", pgUUIDString(participantRow.ParticipantID)),
				})
			}
		}

		// Award entries for all tribe members (if any reward).
		if rewardPerMember > 0 {
			members, err := s.membersForGroupAt(ctx, membershipCache, gc.groupID, resolverCtx.occurrence.EffectiveAt.Time)
			if err != nil {
				return nil, fmt.Errorf("list active members for stir_the_pot group %q: %w", gc.groupName, err)
			}
			for _, member := range members {
				entries = append(entries, resolvedLedgerEntry{
					ParticipantID:  member.ParticipantID,
					SourceGroupID:  gc.groupID,
					HasSourceGroup: true,
					EntryKind:      bonusEntryKindAward,
					Points:         rewardPerMember,
					Visibility:     bonusVisibilityPublic,
					Reason:         fmt.Sprintf("%s Stir the Pot reward", gc.groupName),
					AwardKey:       fmt.Sprintf("stir_the_pot:reward:%s", pgUUIDString(gc.groupID)),
				})
			}
		}
	}

	return entries, nil
}

func (s *Service) resolveIndividualPony(ctx context.Context, resolverCtx resolverContext) ([]resolvedLedgerEntry, error) {
	var metadata individualPonyOccurrenceMetadata
	if err := parseJSON(resolverCtx.occurrence.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("parse individual_pony occurrence metadata: %w", err)
	}
	winningContestantID, err := uuid.Parse(strings.TrimSpace(metadata.WinningContestantID))
	if err != nil {
		return nil, fmt.Errorf("individual_pony occurrence must include winning_contestant_id")
	}
	owners, err := s.queries.ListActiveParticipantPonyOwnershipsByContestantAt(ctx, db.ListActiveParticipantPonyOwnershipsByContestantAtParams{
		InstanceID:   resolverCtx.activity.InstanceID,
		ContestantID: pgtype.UUID{Bytes: [16]byte(winningContestantID), Valid: true},
		At:           resolverCtx.occurrence.EffectiveAt,
	})
	if err != nil {
		return nil, fmt.Errorf("list active pony ownerships: %w", err)
	}
	entries := make([]resolvedLedgerEntry, 0, len(owners))
	for _, owner := range owners {
		entries = append(entries, resolvedLedgerEntry{
			ParticipantID: owner.OwnerParticipantID,
			EntryKind:     bonusEntryKindAward,
			Points:        3,
			Visibility:    bonusVisibilityPublic,
			Reason:        fmt.Sprintf("%s won individual immunity", owner.ContestantName),
			AwardKey:      fmt.Sprintf("individual_pony:%s", pgUUIDString(owner.ContestantID)),
		})
	}
	return entries, nil
}

func (s *Service) stirThePotBonusesForInstance(ctx context.Context, instanceID pgtype.UUID, at time.Time) (map[pgtype.UUID]int32, error) {
	activities, err := s.queries.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instanceID, ActivityType: "stir_the_pot"})
	if err != nil {
		return nil, fmt.Errorf("list stir_the_pot activities: %w", err)
	}
	bonusByGroup := make(map[pgtype.UUID]int32)
	if len(activities) == 0 {
		return bonusByGroup, nil
	}
	currentEpisode, err := s.CurrentEpisode(ctx, instanceID, at)
	if err != nil {
		return nil, fmt.Errorf("get current episode for stir_the_pot resolution: %w", err)
	}
	for _, activity := range activities {
		occurrences, err := s.queries.ListActivityOccurrencesByActivityAndStatus(ctx, db.ListActivityOccurrencesByActivityAndStatusParams{ActivityID: activity.ID, Status: "recorded"})
		if err != nil {
			return nil, fmt.Errorf("list open stir_the_pot occurrences: %w", err)
		}
		for _, occurrence := range occurrences {
			if occurrence.OccurrenceType != "stir_the_pot_round" {
				continue
			}
			if occurrence.EffectiveAt.Time.After(at) {
				continue
			}
			metadata := parseStirThePotRoundMetadata(occurrence.Metadata)
			if !stirThePotTargetsEpisode(metadata.TargetEpisode, currentEpisode) {
				continue
			}
			participants, err := s.queries.ListActivityOccurrenceParticipants(ctx, occurrence.ID)
			if err != nil {
				return nil, fmt.Errorf("list stir_the_pot participants: %w", err)
			}
			contributionByGroup := make(map[pgtype.UUID]int32)
			for _, participant := range participants {
				if participant.Role != "contributor" || !participant.ParticipantGroupID.Valid {
					continue
				}
				var contribution stirThePotContributionMetadata
				if err := parseJSON(participant.Metadata, &contribution); err != nil {
					return nil, fmt.Errorf("parse stir_the_pot contribution for participant %q: %w", participant.ParticipantName, err)
				}
				contributionByGroup[participant.ParticipantGroupID] += contribution.Contribution
			}
			for groupID, total := range contributionByGroup {
				bonus := stirThePotBonusForContribution(total, metadata.RewardTiers)
				if bonus > bonusByGroup[groupID] {
					bonusByGroup[groupID] = bonus
				}
			}
			metadata.ResolvedBy = "tribal_pony"
			metadata.ResolvedAt = at.Format(time.RFC3339)
			resolvedMetadata, err := json.Marshal(metadata)
			if err != nil {
				return nil, fmt.Errorf("marshal stir_the_pot resolved metadata: %w", err)
			}
			if _, err := s.queries.UpdateActivityOccurrenceStatusAndMetadata(ctx, db.UpdateActivityOccurrenceStatusAndMetadataParams{ID: occurrence.ID, Status: "resolved", EndsAt: timestamptz(at), Metadata: resolvedMetadata}); err != nil {
				return nil, fmt.Errorf("resolve stir_the_pot round: %w", err)
			}
		}
	}
	return bonusByGroup, nil
}

func parseStirThePotRoundMetadata(raw []byte) stirThePotRoundMetadata {
	var metadata stirThePotRoundMetadata
	if err := parseJSON(raw, &metadata); err != nil {
		return stirThePotRoundMetadata{RewardTiers: defaultStirThePotRewardTiers()}
	}
	if len(metadata.RewardTiers) == 0 {
		metadata.RewardTiers = defaultStirThePotRewardTiers()
	}
	return metadata
}

func stirThePotTargetsEpisode(target mergeTargetEpisodeMetadata, episode db.GetCurrentEpisodeAtRow) bool {
	if strings.TrimSpace(target.EpisodeID) != "" && strings.EqualFold(strings.TrimSpace(target.EpisodeID), pgUUIDString(episode.ID)) {
		return true
	}
	if target.EpisodeNumber > 0 {
		return target.EpisodeNumber == episode.EpisodeNumber
	}
	return true
}

func stirThePotBonusForContribution(total int32, tiers []stirThePotRewardTier) int32 {
	var best int32
	for _, tier := range tiers {
		if total >= tier.Contributions && tier.Bonus > best {
			best = tier.Bonus
		}
	}
	return best
}

func defaultStirThePotRewardTiers() []stirThePotRewardTier {
	return []stirThePotRewardTier{{Contributions: 2, Bonus: 1}, {Contributions: 5, Bonus: 2}, {Contributions: 8, Bonus: 3}, {Contributions: 11, Bonus: 4}}
}

func (s *Service) resolveJourneyDelegatesAt(ctx context.Context, resolverCtx resolverContext) ([]db.ListActiveActivityParticipantAssignmentsAtRow, error) {
	participantAssignments, err := s.ActiveActivityParticipantAssignmentsAt(ctx, resolverCtx.activity.ID, resolverCtx.occurrence.EffectiveAt.Time)
	if err != nil {
		return nil, fmt.Errorf("list active journey participant assignments: %w", err)
	}

	delegates := make([]db.ListActiveActivityParticipantAssignmentsAtRow, 0)
	for _, assignment := range participantAssignments {
		if strings.EqualFold(assignment.Role, "delegate") {
			delegates = append(delegates, assignment)
		}
	}
	if len(delegates) > 0 {
		return delegates, nil
	}

	for _, participantRow := range resolverCtx.occurrenceParticipants {
		if !strings.EqualFold(participantRow.Role, "delegate") {
			continue
		}
		delegates = append(delegates, db.ListActiveActivityParticipantAssignmentsAtRow{
			ParticipantID:        participantRow.ParticipantID,
			ParticipantName:      participantRow.ParticipantName,
			ParticipantGroupID:   participantRow.ParticipantGroupID,
			ParticipantGroupName: participantRow.ParticipantGroupName,
			Role:                 participantRow.Role,
		})
	}
	if len(delegates) == 0 {
		return nil, fmt.Errorf("journey attendance requires delegate assignments or occurrence participants")
	}
	return delegates, nil
}

func (s *Service) membersForGroupAt(ctx context.Context, cache map[pgtype.UUID][]db.ListActiveParticipantGroupMembershipsAtRow, groupID pgtype.UUID, at time.Time) ([]db.ListActiveParticipantGroupMembershipsAtRow, error) {
	if members, ok := cache[groupID]; ok {
		return members, nil
	}
	members, err := s.ActiveGroupMembershipsAt(ctx, groupID, at)
	if err != nil {
		return nil, err
	}
	cache[groupID] = members
	return members, nil
}

func compareWordleScores(left, right wordleGroupScore) int {
	leftValue := left.Total * right.Count
	rightValue := right.Total * left.Count
	switch {
	case leftValue < rightValue:
		return -1
	case leftValue > rightValue:
		return 1
	default:
		return 0
	}
}

func journeyChoiceForParticipant(row db.ListActivityOccurrenceParticipantsRow) (string, error) {
	if normalized := normalizeChoice(row.Result); normalized != "" {
		return normalized, nil
	}
	var metadata journeyChoiceMetadata
	if err := parseJSON(row.Metadata, &metadata); err != nil {
		return "", err
	}
	return normalizeChoice(metadata.Choice), nil
}

func guessCountFromMetadata(raw []byte) (int, error) {
	var metadata wordleParticipantMetadata
	if err := parseJSON(raw, &metadata); err != nil {
		return 0, err
	}
	if metadata.GuessCount <= 0 {
		return 0, fmt.Errorf("guess_count must be positive")
	}
	return metadata.GuessCount, nil
}

func parseJSON(raw []byte, destination any) error {
	payload := raw
	if len(payload) == 0 {
		payload = emptyJSONB
	}
	return json.Unmarshal(payload, destination)
}

func validateBonusVisibility(visibility string) error {
	switch visibility {
	case bonusVisibilityPublic, bonusVisibilitySecret, bonusVisibilityRevealed:
		return nil
	default:
		return fmt.Errorf("unsupported bonus visibility %q", visibility)
	}
}

func optionalText(value string) pgtype.Text {
	if strings.TrimSpace(value) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func normalizeChoice(value string) string {
	switch normalizeKey(value) {
	case "share":
		return "SHARE"
	case "steal":
		return "STEAL"
	default:
		return ""
	}
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func pgUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
