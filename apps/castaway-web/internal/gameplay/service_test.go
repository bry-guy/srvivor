package gameplay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDefaultEpisodeScheduleForSeason50(t *testing.T) {
	schedule := DefaultEpisodeScheduleForSeason(50)
	if got := len(schedule); got != 14 {
		t.Fatalf("expected 14 schedule entries, got %d", got)
	}
	if schedule[0].EpisodeNumber != 0 || schedule[0].Label != "Preseason" {
		t.Fatalf("expected episode 0 preseason, got %+v", schedule[0])
	}
	if schedule[1].EpisodeNumber != 1 || schedule[1].Label != "Episode 1" {
		t.Fatalf("expected episode 1 label, got %+v", schedule[1])
	}
	if !schedule[1].AirsAt.Before(schedule[2].AirsAt) {
		t.Fatalf("expected episode 1 before episode 2")
	}
}

func TestCopyInstanceSchedule(t *testing.T) {
	fake := &fakeQuerier{}
	service := NewService(fake)
	instanceID := testUUID()

	if err := service.CopyInstanceSchedule(context.Background(), instanceID, 50); err != nil {
		t.Fatalf("copy instance schedule: %v", err)
	}

	expected := DefaultEpisodeScheduleForSeason(50)
	if got := len(fake.createdEpisodes); got != len(expected) {
		t.Fatalf("expected %d created episodes, got %d", len(expected), got)
	}
	if fake.createdEpisodes[0].EpisodeNumber != 0 {
		t.Fatalf("expected first created episode number 0, got %d", fake.createdEpisodes[0].EpisodeNumber)
	}
	if fake.createdEpisodes[0].InstanceID != instanceID {
		t.Fatalf("expected copied instance id to match")
	}
}

func TestCreateMembershipPeriodRequiresEpisodeBoundary(t *testing.T) {
	groupID := testUUID()
	participantID := testUUID()
	instanceID := testUUID()
	boundary0 := time.Date(2026, time.March, 4, 20, 0, 0, 0, time.UTC)
	boundary1 := boundary0.Add(7 * 24 * time.Hour)

	fake := &fakeQuerier{
		participantGroup: db.GetParticipantGroupRow{ID: groupID, InstanceID: instanceID},
		episodes: []db.ListInstanceEpisodesRow{
			{EpisodeNumber: 0, AirsAt: timestamptz(boundary0.Add(-7 * 24 * time.Hour))},
			{EpisodeNumber: 1, AirsAt: timestamptz(boundary0)},
			{EpisodeNumber: 2, AirsAt: timestamptz(boundary1)},
		},
		membershipRow: db.CreateParticipantGroupMembershipPeriodRow{ParticipantGroupID: groupID, ParticipantID: participantID},
	}
	service := NewService(fake)

	if _, err := service.CreateMembershipPeriod(context.Background(), CreateMembershipPeriodParams{
		ParticipantGroupID: groupID,
		ParticipantID:      participantID,
		StartsAt:           boundary0,
		EndsAt:             &boundary1,
	}); err != nil {
		t.Fatalf("expected boundary-aligned period to succeed, got %v", err)
	}
	if len(fake.createdMemberships) != 1 {
		t.Fatalf("expected one created membership, got %d", len(fake.createdMemberships))
	}

	if _, err := service.CreateMembershipPeriod(context.Background(), CreateMembershipPeriodParams{
		ParticipantGroupID: groupID,
		ParticipantID:      participantID,
		StartsAt:           boundary0.Add(time.Hour),
	}); err == nil {
		t.Fatalf("expected non-boundary membership change to fail")
	}
	if got := len(fake.createdMemberships); got != 1 {
		t.Fatalf("expected invalid membership not to be created, got %d creates", got)
	}
}

func TestCreateActivityGroupAssignmentRequiresEpisodeBoundary(t *testing.T) {
	activityID := testUUID()
	groupID := testUUID()
	instanceID := testUUID()
	boundary0 := time.Date(2026, time.March, 4, 20, 0, 0, 0, time.UTC)
	boundary1 := boundary0.Add(7 * 24 * time.Hour)

	fake := &fakeQuerier{
		instanceActivity: db.GetInstanceActivityRow{ID: activityID, InstanceID: instanceID},
		episodes: []db.ListInstanceEpisodesRow{
			{EpisodeNumber: 0, AirsAt: timestamptz(boundary0.Add(-7 * 24 * time.Hour))},
			{EpisodeNumber: 1, AirsAt: timestamptz(boundary0)},
			{EpisodeNumber: 2, AirsAt: timestamptz(boundary1)},
		},
		activityGroupAssignmentRow: db.CreateActivityGroupAssignmentRow{ActivityID: activityID, ParticipantGroupID: groupID},
	}
	service := NewService(fake)

	if _, err := service.CreateActivityGroupAssignment(context.Background(), CreateActivityGroupAssignmentParams{
		ActivityID:         activityID,
		ParticipantGroupID: groupID,
		Role:               "tribe",
		StartsAt:           boundary0,
	}); err != nil {
		t.Fatalf("expected boundary-aligned activity assignment to succeed, got %v", err)
	}
	if len(fake.createdActivityGroupAssignments) != 1 {
		t.Fatalf("expected one created activity group assignment, got %d", len(fake.createdActivityGroupAssignments))
	}

	if _, err := service.CreateActivityGroupAssignment(context.Background(), CreateActivityGroupAssignmentParams{
		ActivityID:         activityID,
		ParticipantGroupID: groupID,
		Role:               "tribe",
		StartsAt:           boundary0.Add(30 * time.Minute),
	}); err == nil {
		t.Fatalf("expected non-boundary assignment to fail")
	}
}

func TestCreateParticipantAssignmentAllowsOptionalGroup(t *testing.T) {
	activityID := testUUID()
	participantID := testUUID()
	instanceID := testUUID()
	boundary0 := time.Date(2026, time.March, 4, 20, 0, 0, 0, time.UTC)

	fake := &fakeQuerier{
		instanceActivity: db.GetInstanceActivityRow{ID: activityID, InstanceID: instanceID},
		episodes: []db.ListInstanceEpisodesRow{
			{EpisodeNumber: 0, AirsAt: timestamptz(boundary0)},
		},
		activityParticipantAssignmentRow: db.CreateActivityParticipantAssignmentRow{ActivityID: activityID, ParticipantID: participantID},
	}
	service := NewService(fake)

	if _, err := service.CreateParticipantAssignment(context.Background(), CreateActivityParticipantAssignmentParams{
		ActivityID:    activityID,
		ParticipantID: participantID,
		Role:          "delegate",
		StartsAt:      boundary0,
	}); err != nil {
		t.Fatalf("create participant assignment: %v", err)
	}
	if len(fake.createdActivityParticipantAssignments) != 1 {
		t.Fatalf("expected one created participant assignment, got %d", len(fake.createdActivityParticipantAssignments))
	}
	if fake.createdActivityParticipantAssignments[0].ParticipantGroupID.Valid {
		t.Fatalf("expected optional participant_group_id to be unset")
	}
}

func TestBonusAggregateHelpers(t *testing.T) {
	instanceID := testUUID()
	participantID := testUUID()
	asOf := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)
	fake := &fakeQuerier{
		visibleTotal:         2,
		secretTotal:          3,
		visibleTotalAsOf:     1,
		availableSecretTotal: 1,
	}
	service := NewService(fake)

	visible, err := service.VisibleBonusTotalByParticipant(context.Background(), instanceID, participantID)
	if err != nil || visible != 2 {
		t.Fatalf("visible total = %d, %v; want 2, nil", visible, err)
	}
	secret, err := service.SecretBonusTotalByParticipant(context.Background(), instanceID, participantID)
	if err != nil || secret != 3 {
		t.Fatalf("secret total = %d, %v; want 3, nil", secret, err)
	}
	visibleAsOf, err := service.VisibleBonusTotalByParticipantAsOf(context.Background(), instanceID, participantID, asOf)
	if err != nil || visibleAsOf != 1 {
		t.Fatalf("visible as-of total = %d, %v; want 1, nil", visibleAsOf, err)
	}
	availableSecret, err := service.AvailableSecretBalanceByParticipant(context.Background(), instanceID, participantID)
	if err != nil || availableSecret != 1 {
		t.Fatalf("available secret total = %d, %v; want 1, nil", availableSecret, err)
	}
}

type fakeQuerier struct {
	createdEpisodes                       []db.CreateInstanceEpisodeParams
	episodes                              []db.ListInstanceEpisodesRow
	participantGroup                      db.GetParticipantGroupRow
	membershipRow                         db.CreateParticipantGroupMembershipPeriodRow
	createdMemberships                    []db.CreateParticipantGroupMembershipPeriodParams
	activeMembershipsByGroup              map[[16]byte][]db.ListActiveParticipantGroupMembershipsAtRow
	activityOccurrence                    db.GetActivityOccurrenceRow
	occurrenceGroups                      []db.ListActivityOccurrenceGroupsRow
	occurrenceParticipants                []db.ListActivityOccurrenceParticipantsRow
	instanceActivity                      db.GetInstanceActivityRow
	activityGroupAssignmentRow            db.CreateActivityGroupAssignmentRow
	createdActivityGroupAssignments       []db.CreateActivityGroupAssignmentParams
	activeActivityGroupAssignments        []db.ListActiveActivityGroupAssignmentsAtRow
	activityParticipantAssignmentRow      db.CreateActivityParticipantAssignmentRow
	createdActivityParticipantAssignments []db.CreateActivityParticipantAssignmentParams
	activeActivityParticipantAssignments  []db.ListActiveActivityParticipantAssignmentsAtRow
	createdBonusLedgerEntries             []db.CreateBonusPointLedgerEntryParams
	visibleTotal                          int32
	secretTotal                           int32
	visibleTotalAsOf                      int32
	availableSecretTotal                  int32
	activeAdvantagesByGroup               []db.ListActiveAdvantagesByTypeForGroupRow
	activeAdvantagesByParticipant         []db.ListActiveAdvantagesByTypeForParticipantRow
	createdAdvantages                     []db.CreateParticipantAdvantageParams
}

func (f *fakeQuerier) CreateInstanceEpisode(_ context.Context, arg db.CreateInstanceEpisodeParams) (db.CreateInstanceEpisodeRow, error) {
	f.createdEpisodes = append(f.createdEpisodes, arg)
	return db.CreateInstanceEpisodeRow{InstanceID: arg.InstanceID, EpisodeNumber: arg.EpisodeNumber, Label: arg.Label, AirsAt: arg.AirsAt}, nil
}

func (f *fakeQuerier) ListInstanceEpisodes(context.Context, pgtype.UUID) ([]db.ListInstanceEpisodesRow, error) {
	return f.episodes, nil
}

func (f *fakeQuerier) GetCurrentEpisodeAt(context.Context, db.GetCurrentEpisodeAtParams) (db.GetCurrentEpisodeAtRow, error) {
	return db.GetCurrentEpisodeAtRow{}, errors.New("unexpected call")
}

func (f *fakeQuerier) ListEpisodeBoundaryWindows(context.Context, pgtype.UUID) ([]db.ListEpisodeBoundaryWindowsRow, error) {
	return nil, errors.New("unexpected call")
}

func (f *fakeQuerier) GetParticipantGroup(context.Context, pgtype.UUID) (db.GetParticipantGroupRow, error) {
	return f.participantGroup, nil
}

func (f *fakeQuerier) CreateParticipantGroupMembershipPeriod(_ context.Context, arg db.CreateParticipantGroupMembershipPeriodParams) (db.CreateParticipantGroupMembershipPeriodRow, error) {
	f.createdMemberships = append(f.createdMemberships, arg)
	return f.membershipRow, nil
}

func (f *fakeQuerier) ListActiveParticipantGroupMembershipsAt(_ context.Context, arg db.ListActiveParticipantGroupMembershipsAtParams) ([]db.ListActiveParticipantGroupMembershipsAtRow, error) {
	if f.activeMembershipsByGroup == nil {
		return nil, errors.New("unexpected call")
	}
	return f.activeMembershipsByGroup[arg.ParticipantGroupID.Bytes], nil
}

func (f *fakeQuerier) GetActivityOccurrence(context.Context, pgtype.UUID) (db.GetActivityOccurrenceRow, error) {
	return f.activityOccurrence, nil
}

func (f *fakeQuerier) GetInstanceActivity(context.Context, pgtype.UUID) (db.GetInstanceActivityRow, error) {
	return f.instanceActivity, nil
}

func (f *fakeQuerier) CreateActivityGroupAssignment(_ context.Context, arg db.CreateActivityGroupAssignmentParams) (db.CreateActivityGroupAssignmentRow, error) {
	f.createdActivityGroupAssignments = append(f.createdActivityGroupAssignments, arg)
	return f.activityGroupAssignmentRow, nil
}

func (f *fakeQuerier) ListActiveActivityGroupAssignmentsAt(context.Context, db.ListActiveActivityGroupAssignmentsAtParams) ([]db.ListActiveActivityGroupAssignmentsAtRow, error) {
	if f.activeActivityGroupAssignments == nil {
		return nil, errors.New("unexpected call")
	}
	return f.activeActivityGroupAssignments, nil
}

func (f *fakeQuerier) CreateActivityParticipantAssignment(_ context.Context, arg db.CreateActivityParticipantAssignmentParams) (db.CreateActivityParticipantAssignmentRow, error) {
	f.createdActivityParticipantAssignments = append(f.createdActivityParticipantAssignments, arg)
	return f.activityParticipantAssignmentRow, nil
}

func (f *fakeQuerier) ListActiveActivityParticipantAssignmentsAt(context.Context, db.ListActiveActivityParticipantAssignmentsAtParams) ([]db.ListActiveActivityParticipantAssignmentsAtRow, error) {
	if f.activeActivityParticipantAssignments == nil {
		return nil, errors.New("unexpected call")
	}
	return f.activeActivityParticipantAssignments, nil
}

func (f *fakeQuerier) ListActivityOccurrenceGroups(context.Context, pgtype.UUID) ([]db.ListActivityOccurrenceGroupsRow, error) {
	return f.occurrenceGroups, nil
}

func (f *fakeQuerier) ListActivityOccurrenceParticipants(context.Context, pgtype.UUID) ([]db.ListActivityOccurrenceParticipantsRow, error) {
	return f.occurrenceParticipants, nil
}

func (f *fakeQuerier) CreateBonusPointLedgerEntry(_ context.Context, arg db.CreateBonusPointLedgerEntryParams) (db.CreateBonusPointLedgerEntryRow, error) {
	f.createdBonusLedgerEntries = append(f.createdBonusLedgerEntries, arg)
	return db.CreateBonusPointLedgerEntryRow{
		InstanceID:           arg.InstanceID,
		ParticipantID:        arg.ParticipantID,
		ActivityOccurrenceID: arg.ActivityOccurrenceID,
		SourceGroupID:        arg.SourceGroupID,
		EntryKind:            arg.EntryKind,
		Points:               arg.Points,
		Visibility:           arg.Visibility,
		Reason:               arg.Reason,
		EffectiveAt:          arg.EffectiveAt,
		AwardKey:             arg.AwardKey,
		Metadata:             arg.Metadata,
	}, nil
}

func (f *fakeQuerier) GetVisibleBonusTotalByParticipant(context.Context, db.GetVisibleBonusTotalByParticipantParams) (int32, error) {
	return f.visibleTotal, nil
}

func (f *fakeQuerier) GetSecretBonusTotalByParticipant(context.Context, db.GetSecretBonusTotalByParticipantParams) (int32, error) {
	return f.secretTotal, nil
}

func (f *fakeQuerier) GetVisibleBonusTotalByParticipantAsOf(context.Context, db.GetVisibleBonusTotalByParticipantAsOfParams) (int32, error) {
	return f.visibleTotalAsOf, nil
}

func (f *fakeQuerier) GetAvailableSecretBalanceByParticipant(context.Context, db.GetAvailableSecretBalanceByParticipantParams) (int32, error) {
	return f.availableSecretTotal, nil
}

func (f *fakeQuerier) CreateParticipantAdvantage(_ context.Context, arg db.CreateParticipantAdvantageParams) (db.CreateParticipantAdvantageRow, error) {
	f.createdAdvantages = append(f.createdAdvantages, arg)
	return db.CreateParticipantAdvantageRow{
		InstanceID:    arg.InstanceID,
		ParticipantID: arg.ParticipantID,
		AdvantageType: arg.AdvantageType,
		Name:          arg.Name,
		Status:        arg.Status,
	}, nil
}

func (f *fakeQuerier) ListActiveAdvantagesByTypeForGroup(context.Context, db.ListActiveAdvantagesByTypeForGroupParams) ([]db.ListActiveAdvantagesByTypeForGroupRow, error) {
	return f.activeAdvantagesByGroup, nil
}

func (f *fakeQuerier) ListActiveAdvantagesByTypeForParticipant(context.Context, db.ListActiveAdvantagesByTypeForParticipantParams) ([]db.ListActiveAdvantagesByTypeForParticipantRow, error) {
	return f.activeAdvantagesByParticipant, nil
}

func (f *fakeQuerier) MarkAdvantageUsed(context.Context, pgtype.UUID) error {
	return nil
}

func testUUID() pgtype.UUID {
	id := uuid.New()
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}
