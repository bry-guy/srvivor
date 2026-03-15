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
		for _, member := range members {
			entries = append(entries, resolvedLedgerEntry{
				ParticipantID:  member.ParticipantID,
				SourceGroupID:  assignment.ParticipantGroupID,
				HasSourceGroup: true,
				EntryKind:      bonusEntryKindAward,
				Points:         1,
				Visibility:     bonusVisibilityPublic,
				Reason:         fmt.Sprintf("%s pony tribe won immunity", assignment.ParticipantGroupName),
				AwardKey:       fmt.Sprintf("tribal_pony:%s", pgUUIDString(assignment.ParticipantGroupID)),
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
