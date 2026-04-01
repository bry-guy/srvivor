package gameplay

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

var emptyJSONB = []byte("{}")

type serviceQuerier interface {
	CreateInstanceEpisode(ctx context.Context, arg db.CreateInstanceEpisodeParams) (db.CreateInstanceEpisodeRow, error)
	ListInstanceEpisodes(ctx context.Context, instanceID pgtype.UUID) ([]db.ListInstanceEpisodesRow, error)
	GetCurrentEpisodeAt(ctx context.Context, arg db.GetCurrentEpisodeAtParams) (db.GetCurrentEpisodeAtRow, error)
	ListEpisodeBoundaryWindows(ctx context.Context, instanceID pgtype.UUID) ([]db.ListEpisodeBoundaryWindowsRow, error)
	GetParticipantGroup(ctx context.Context, id pgtype.UUID) (db.GetParticipantGroupRow, error)
	CreateParticipantGroupMembershipPeriod(ctx context.Context, arg db.CreateParticipantGroupMembershipPeriodParams) (db.CreateParticipantGroupMembershipPeriodRow, error)
	ListActiveParticipantGroupMembershipsAt(ctx context.Context, arg db.ListActiveParticipantGroupMembershipsAtParams) ([]db.ListActiveParticipantGroupMembershipsAtRow, error)
	GetActivityOccurrence(ctx context.Context, id pgtype.UUID) (db.GetActivityOccurrenceRow, error)
	GetInstanceActivity(ctx context.Context, id pgtype.UUID) (db.GetInstanceActivityRow, error)
	CreateActivityGroupAssignment(ctx context.Context, arg db.CreateActivityGroupAssignmentParams) (db.CreateActivityGroupAssignmentRow, error)
	ListActiveActivityGroupAssignmentsAt(ctx context.Context, arg db.ListActiveActivityGroupAssignmentsAtParams) ([]db.ListActiveActivityGroupAssignmentsAtRow, error)
	CreateActivityParticipantAssignment(ctx context.Context, arg db.CreateActivityParticipantAssignmentParams) (db.CreateActivityParticipantAssignmentRow, error)
	ListActiveActivityParticipantAssignmentsAt(ctx context.Context, arg db.ListActiveActivityParticipantAssignmentsAtParams) ([]db.ListActiveActivityParticipantAssignmentsAtRow, error)
	ListActivityOccurrenceGroups(ctx context.Context, activityOccurrenceID pgtype.UUID) ([]db.ListActivityOccurrenceGroupsRow, error)
	ListActivityOccurrenceParticipants(ctx context.Context, activityOccurrenceID pgtype.UUID) ([]db.ListActivityOccurrenceParticipantsRow, error)
	ListActivityOccurrencesByActivityAndStatus(ctx context.Context, arg db.ListActivityOccurrencesByActivityAndStatusParams) ([]db.ListActivityOccurrencesByActivityAndStatusRow, error)
	UpdateActivityOccurrenceStatusAndMetadata(ctx context.Context, arg db.UpdateActivityOccurrenceStatusAndMetadataParams) (db.UpdateActivityOccurrenceStatusAndMetadataRow, error)
	ListInstanceActivitiesByType(ctx context.Context, arg db.ListInstanceActivitiesByTypeParams) ([]db.ListInstanceActivitiesByTypeRow, error)
	CreateBonusPointLedgerEntry(ctx context.Context, arg db.CreateBonusPointLedgerEntryParams) (db.CreateBonusPointLedgerEntryRow, error)
	GetVisibleBonusTotalByParticipant(ctx context.Context, arg db.GetVisibleBonusTotalByParticipantParams) (int32, error)
	GetSecretBonusTotalByParticipant(ctx context.Context, arg db.GetSecretBonusTotalByParticipantParams) (int32, error)
	GetVisibleBonusTotalByParticipantAsOf(ctx context.Context, arg db.GetVisibleBonusTotalByParticipantAsOfParams) (int32, error)
	GetAvailableSecretBalanceByParticipant(ctx context.Context, arg db.GetAvailableSecretBalanceByParticipantParams) (int32, error)
	CreateParticipantAdvantage(ctx context.Context, arg db.CreateParticipantAdvantageParams) (db.CreateParticipantAdvantageRow, error)
	ListActiveAdvantagesByTypeForGroup(ctx context.Context, arg db.ListActiveAdvantagesByTypeForGroupParams) ([]db.ListActiveAdvantagesByTypeForGroupRow, error)
	ListActiveAdvantagesByTypeForParticipant(ctx context.Context, arg db.ListActiveAdvantagesByTypeForParticipantParams) ([]db.ListActiveAdvantagesByTypeForParticipantRow, error)
	CreateParticipantPonyOwnership(ctx context.Context, arg db.CreateParticipantPonyOwnershipParams) (db.CreateParticipantPonyOwnershipRow, error)
	ListActiveParticipantPonyOwnershipsByContestantAt(ctx context.Context, arg db.ListActiveParticipantPonyOwnershipsByContestantAtParams) ([]db.ListActiveParticipantPonyOwnershipsByContestantAtRow, error)
	MarkAdvantageUsed(ctx context.Context, id pgtype.UUID) error
}

type Service struct {
	queries serviceQuerier
}

func NewService(queries serviceQuerier) *Service {
	return &Service{queries: queries}
}

type EpisodeTemplate struct {
	EpisodeNumber int32
	Label         string
	AirsAt        time.Time
}

func DefaultEpisodeScheduleForSeason(season int32) []EpisodeTemplate {
	if season == 50 {
		location, err := time.LoadLocation("America/New_York")
		if err != nil {
			location = time.UTC
		}

		preseason := time.Date(2026, time.February, 25, 20, 0, 0, 0, location)
		firstEpisode := time.Date(2026, time.March, 4, 20, 0, 0, 0, location)
		schedule := []EpisodeTemplate{{
			EpisodeNumber: 0,
			Label:         "Preseason",
			AirsAt:        preseason,
		}}
		for episodeNumber := int32(1); episodeNumber <= 13; episodeNumber++ {
			schedule = append(schedule, EpisodeTemplate{
				EpisodeNumber: episodeNumber,
				Label:         fmt.Sprintf("Episode %d", episodeNumber),
				AirsAt:        firstEpisode.AddDate(0, 0, int(7*(episodeNumber-1))),
			})
		}
		return schedule
	}

	return []EpisodeTemplate{{
		EpisodeNumber: 0,
		Label:         "Preseason",
		AirsAt:        time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}}
}

func (s *Service) CopyInstanceSchedule(ctx context.Context, instanceID pgtype.UUID, season int32) error {
	for _, episode := range DefaultEpisodeScheduleForSeason(season) {
		if _, err := s.queries.CreateInstanceEpisode(ctx, db.CreateInstanceEpisodeParams{
			InstanceID:    instanceID,
			EpisodeNumber: episode.EpisodeNumber,
			Label:         episode.Label,
			AirsAt:        timestamptz(episode.AirsAt),
			Metadata:      emptyJSONB,
		}); err != nil {
			return fmt.Errorf("create episode %d for season %d: %w", episode.EpisodeNumber, season, err)
		}
	}
	return nil
}

func (s *Service) CurrentEpisode(ctx context.Context, instanceID pgtype.UUID, at time.Time) (db.GetCurrentEpisodeAtRow, error) {
	return s.queries.GetCurrentEpisodeAt(ctx, db.GetCurrentEpisodeAtParams{
		InstanceID: instanceID,
		At:         timestamptz(at),
	})
}

func (s *Service) EpisodeBoundaryWindows(ctx context.Context, instanceID pgtype.UUID) ([]db.ListEpisodeBoundaryWindowsRow, error) {
	return s.queries.ListEpisodeBoundaryWindows(ctx, instanceID)
}

type CreateMembershipPeriodParams struct {
	ParticipantGroupID pgtype.UUID
	ParticipantID      pgtype.UUID
	Role               string
	StartsAt           time.Time
	EndsAt             *time.Time
	Metadata           []byte
}

func (s *Service) CreateMembershipPeriod(ctx context.Context, params CreateMembershipPeriodParams) (db.CreateParticipantGroupMembershipPeriodRow, error) {
	group, err := s.queries.GetParticipantGroup(ctx, params.ParticipantGroupID)
	if err != nil {
		return db.CreateParticipantGroupMembershipPeriodRow{}, fmt.Errorf("get participant group: %w", err)
	}
	if err := s.requireEpisodeBoundaries(ctx, group.InstanceID, params.StartsAt, params.EndsAt); err != nil {
		return db.CreateParticipantGroupMembershipPeriodRow{}, err
	}
	role := params.Role
	if role == "" {
		role = "member"
	}

	return s.queries.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{
		ParticipantGroupID: params.ParticipantGroupID,
		ParticipantID:      params.ParticipantID,
		Role:               role,
		StartsAt:           timestamptz(params.StartsAt),
		EndsAt:             optionalTimestamptz(params.EndsAt),
		Metadata:           jsonbOrEmpty(params.Metadata),
	})
}

func (s *Service) ActiveGroupMembershipsAt(ctx context.Context, participantGroupID pgtype.UUID, at time.Time) ([]db.ListActiveParticipantGroupMembershipsAtRow, error) {
	return s.queries.ListActiveParticipantGroupMembershipsAt(ctx, db.ListActiveParticipantGroupMembershipsAtParams{
		ParticipantGroupID: participantGroupID,
		At:                 timestamptz(at),
	})
}

type CreateActivityGroupAssignmentParams struct {
	ActivityID         pgtype.UUID
	ParticipantGroupID pgtype.UUID
	Role               string
	StartsAt           time.Time
	EndsAt             *time.Time
	Configuration      []byte
}

func (s *Service) CreateActivityGroupAssignment(ctx context.Context, params CreateActivityGroupAssignmentParams) (db.CreateActivityGroupAssignmentRow, error) {
	activity, err := s.queries.GetInstanceActivity(ctx, params.ActivityID)
	if err != nil {
		return db.CreateActivityGroupAssignmentRow{}, fmt.Errorf("get instance activity: %w", err)
	}
	if err := s.requireEpisodeBoundaries(ctx, activity.InstanceID, params.StartsAt, params.EndsAt); err != nil {
		return db.CreateActivityGroupAssignmentRow{}, err
	}

	return s.queries.CreateActivityGroupAssignment(ctx, db.CreateActivityGroupAssignmentParams{
		ActivityID:         params.ActivityID,
		ParticipantGroupID: params.ParticipantGroupID,
		Role:               params.Role,
		StartsAt:           timestamptz(params.StartsAt),
		EndsAt:             optionalTimestamptz(params.EndsAt),
		Configuration:      jsonbOrEmpty(params.Configuration),
	})
}

func (s *Service) ActiveActivityGroupAssignmentsAt(ctx context.Context, activityID pgtype.UUID, at time.Time) ([]db.ListActiveActivityGroupAssignmentsAtRow, error) {
	return s.queries.ListActiveActivityGroupAssignmentsAt(ctx, db.ListActiveActivityGroupAssignmentsAtParams{
		ActivityID: activityID,
		At:         timestamptz(at),
	})
}

type CreateActivityParticipantAssignmentParams struct {
	ActivityID          pgtype.UUID
	ParticipantID       pgtype.UUID
	ParticipantGroupID  pgtype.UUID
	Role                string
	StartsAt            time.Time
	EndsAt              *time.Time
	Configuration       []byte
	ParticipantGroupSet bool
}

func (s *Service) CreateParticipantAssignment(ctx context.Context, params CreateActivityParticipantAssignmentParams) (db.CreateActivityParticipantAssignmentRow, error) {
	activity, err := s.queries.GetInstanceActivity(ctx, params.ActivityID)
	if err != nil {
		return db.CreateActivityParticipantAssignmentRow{}, fmt.Errorf("get instance activity: %w", err)
	}
	if err := s.requireEpisodeBoundaries(ctx, activity.InstanceID, params.StartsAt, params.EndsAt); err != nil {
		return db.CreateActivityParticipantAssignmentRow{}, err
	}

	participantGroupID := pgtype.UUID{Valid: false}
	if params.ParticipantGroupSet {
		participantGroupID = params.ParticipantGroupID
	}

	return s.queries.CreateActivityParticipantAssignment(ctx, db.CreateActivityParticipantAssignmentParams{
		ActivityID:         params.ActivityID,
		ParticipantID:      params.ParticipantID,
		ParticipantGroupID: participantGroupID,
		Role:               params.Role,
		StartsAt:           timestamptz(params.StartsAt),
		EndsAt:             optionalTimestamptz(params.EndsAt),
		Configuration:      jsonbOrEmpty(params.Configuration),
	})
}

func (s *Service) ActiveActivityParticipantAssignmentsAt(ctx context.Context, activityID pgtype.UUID, at time.Time) ([]db.ListActiveActivityParticipantAssignmentsAtRow, error) {
	return s.queries.ListActiveActivityParticipantAssignmentsAt(ctx, db.ListActiveActivityParticipantAssignmentsAtParams{
		ActivityID: activityID,
		At:         timestamptz(at),
	})
}

func (s *Service) VisibleBonusTotalByParticipant(ctx context.Context, instanceID, participantID pgtype.UUID) (int32, error) {
	return s.queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
	})
}

func (s *Service) SecretBonusTotalByParticipant(ctx context.Context, instanceID, participantID pgtype.UUID) (int32, error) {
	return s.queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
	})
}

func (s *Service) VisibleBonusTotalByParticipantAsOf(ctx context.Context, instanceID, participantID pgtype.UUID, asOf time.Time) (int32, error) {
	return s.queries.GetVisibleBonusTotalByParticipantAsOf(ctx, db.GetVisibleBonusTotalByParticipantAsOfParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
		AsOf:          timestamptz(asOf),
	})
}

func (s *Service) AvailableSecretBalanceByParticipant(ctx context.Context, instanceID, participantID pgtype.UUID) (int32, error) {
	return s.queries.GetAvailableSecretBalanceByParticipant(ctx, db.GetAvailableSecretBalanceByParticipantParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
	})
}

func (s *Service) requireEpisodeBoundaries(ctx context.Context, instanceID pgtype.UUID, startsAt time.Time, endsAt *time.Time) error {
	episodes, err := s.queries.ListInstanceEpisodes(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("list instance episodes: %w", err)
	}

	boundaries := make([]time.Time, 0, len(episodes))
	for _, episode := range episodes {
		boundaries = append(boundaries, episode.AirsAt.Time)
	}
	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].Before(boundaries[j])
	})

	if !matchesBoundary(boundaries, startsAt) {
		return fmt.Errorf("starts_at must match an explicit episode boundary")
	}
	if endsAt != nil && !matchesBoundary(boundaries, *endsAt) {
		return fmt.Errorf("ends_at must match an explicit episode boundary")
	}
	return nil
}

func matchesBoundary(boundaries []time.Time, candidate time.Time) bool {
	for _, boundary := range boundaries {
		if boundary.Equal(candidate) {
			return true
		}
	}
	return false
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

func jsonbOrEmpty(value []byte) []byte {
	if len(value) == 0 {
		return emptyJSONB
	}
	return value
}
