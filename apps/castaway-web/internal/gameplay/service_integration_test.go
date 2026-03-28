package gameplay_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appinternal "github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var emptyJSONB = []byte("{}")

func TestEpisodeScheduleIntegration(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	service := gameplay.NewService(queries)
	instance := createInstanceForTest(t, ctx, queries, "Schedule Integration", 50)

	if err := service.CopyInstanceSchedule(ctx, instance.ID, instance.Season); err != nil {
		t.Fatalf("copy instance schedule: %v", err)
	}

	episodes, err := queries.ListInstanceEpisodes(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list instance episodes: %v", err)
	}
	expectedSchedule := gameplay.DefaultEpisodeScheduleForSeason(50)
	if len(episodes) != len(expectedSchedule) {
		t.Fatalf("expected %d episodes, got %d", len(expectedSchedule), len(episodes))
	}
	if episodes[0].EpisodeNumber != 0 {
		t.Fatalf("expected episode 0 first, got %d", episodes[0].EpisodeNumber)
	}

	currentBeforeEpisodeOne, err := service.CurrentEpisode(ctx, instance.ID, expectedSchedule[0].AirsAt.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("current episode before episode one: %v", err)
	}
	if currentBeforeEpisodeOne.EpisodeNumber != 0 {
		t.Fatalf("expected current episode 0 before episode 1, got %d", currentBeforeEpisodeOne.EpisodeNumber)
	}

	currentAfterEpisodeTwo, err := service.CurrentEpisode(ctx, instance.ID, expectedSchedule[2].AirsAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("current episode after episode two: %v", err)
	}
	if currentAfterEpisodeTwo.EpisodeNumber != 2 {
		t.Fatalf("expected current episode 2, got %d", currentAfterEpisodeTwo.EpisodeNumber)
	}

	windows, err := service.EpisodeBoundaryWindows(ctx, instance.ID)
	if err != nil {
		t.Fatalf("episode boundary windows: %v", err)
	}
	if len(windows) != len(expectedSchedule) {
		t.Fatalf("expected %d windows, got %d", len(expectedSchedule), len(windows))
	}
}

func TestGroupActivityAndLedgerIntegration(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	service := gameplay.NewService(queries)
	instance := createInstanceForTest(t, ctx, queries, "Gameplay Integration", 50)
	if err := service.CopyInstanceSchedule(ctx, instance.ID, instance.Season); err != nil {
		t.Fatalf("copy instance schedule: %v", err)
	}

	schedule := gameplay.DefaultEpisodeScheduleForSeason(50)
	boundary0 := schedule[0].AirsAt
	boundary1 := schedule[1].AirsAt
	boundary2 := schedule[2].AirsAt

	participantA := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	participantB := createParticipantForTest(t, ctx, queries, instance.ID, "Bob")
	group := createGroupForTest(t, ctx, queries, instance.ID, "Lotus", "tribe")

	if _, err := service.CreateMembershipPeriod(ctx, gameplay.CreateMembershipPeriodParams{
		ParticipantGroupID: group.ID,
		ParticipantID:      participantA.ID,
		StartsAt:           boundary0,
		EndsAt:             &boundary1,
	}); err != nil {
		t.Fatalf("create membership period A: %v", err)
	}
	if _, err := service.CreateMembershipPeriod(ctx, gameplay.CreateMembershipPeriodParams{
		ParticipantGroupID: group.ID,
		ParticipantID:      participantB.ID,
		StartsAt:           boundary1,
	}); err != nil {
		t.Fatalf("create membership period B: %v", err)
	}

	membersBeforeEpisodeOne, err := service.ActiveGroupMembershipsAt(ctx, group.ID, boundary0.Add(time.Hour))
	if err != nil {
		t.Fatalf("active memberships before episode one: %v", err)
	}
	if len(membersBeforeEpisodeOne) != 1 || membersBeforeEpisodeOne[0].ParticipantID != participantA.ID {
		t.Fatalf("expected only participant A before episode one, got %+v", membersBeforeEpisodeOne)
	}

	membersAfterEpisodeOne, err := service.ActiveGroupMembershipsAt(ctx, group.ID, boundary1.Add(time.Hour))
	if err != nil {
		t.Fatalf("active memberships after episode one: %v", err)
	}
	if len(membersAfterEpisodeOne) != 1 || membersAfterEpisodeOne[0].ParticipantID != participantB.ID {
		t.Fatalf("expected only participant B after episode one, got %+v", membersAfterEpisodeOne)
	}

	activity := createActivityForTest(t, ctx, queries, instance.ID, boundary0, &boundary2)
	if _, err := service.CreateActivityGroupAssignment(ctx, gameplay.CreateActivityGroupAssignmentParams{
		ActivityID:         activity.ID,
		ParticipantGroupID: group.ID,
		Role:               "tribe",
		StartsAt:           boundary0,
		EndsAt:             &boundary1,
		Configuration:      []byte(`{"pony_survivor_tribe":"vatu"}`),
	}); err != nil {
		t.Fatalf("create first group assignment: %v", err)
	}
	if _, err := service.CreateActivityGroupAssignment(ctx, gameplay.CreateActivityGroupAssignmentParams{
		ActivityID:         activity.ID,
		ParticipantGroupID: group.ID,
		Role:               "tribe",
		StartsAt:           boundary1,
		Configuration:      []byte(`{"pony_survivor_tribe":"kalo"}`),
	}); err != nil {
		t.Fatalf("create second group assignment: %v", err)
	}
	if _, err := service.CreateParticipantAssignment(ctx, gameplay.CreateActivityParticipantAssignmentParams{
		ActivityID:          activity.ID,
		ParticipantID:       participantB.ID,
		ParticipantGroupID:  group.ID,
		ParticipantGroupSet: true,
		Role:                "delegate",
		StartsAt:            boundary1,
	}); err != nil {
		t.Fatalf("create participant assignment: %v", err)
	}

	groupAssignmentsBeforeEpisodeOne, err := service.ActiveActivityGroupAssignmentsAt(ctx, activity.ID, boundary0.Add(time.Hour))
	if err != nil {
		t.Fatalf("active group assignments before episode one: %v", err)
	}
	if len(groupAssignmentsBeforeEpisodeOne) != 1 || !strings.Contains(string(groupAssignmentsBeforeEpisodeOne[0].Configuration), `"vatu"`) {
		t.Fatalf("expected first config before episode one, got %+v", groupAssignmentsBeforeEpisodeOne)
	}

	groupAssignmentsAfterEpisodeOne, err := service.ActiveActivityGroupAssignmentsAt(ctx, activity.ID, boundary1.Add(time.Hour))
	if err != nil {
		t.Fatalf("active group assignments after episode one: %v", err)
	}
	if len(groupAssignmentsAfterEpisodeOne) != 1 || !strings.Contains(string(groupAssignmentsAfterEpisodeOne[0].Configuration), `"kalo"`) {
		t.Fatalf("expected second config after episode one, got %+v", groupAssignmentsAfterEpisodeOne)
	}

	participantAssignmentsAfterEpisodeOne, err := service.ActiveActivityParticipantAssignmentsAt(ctx, activity.ID, boundary1.Add(time.Hour))
	if err != nil {
		t.Fatalf("active participant assignments after episode one: %v", err)
	}
	if len(participantAssignmentsAfterEpisodeOne) != 1 || participantAssignmentsAfterEpisodeOne[0].ParticipantID != participantB.ID {
		t.Fatalf("expected participant B assignment after episode one, got %+v", participantAssignmentsAfterEpisodeOne)
	}

	occurrence, err := queries.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		OccurrenceType: "journey_resolution",
		Name:           "Journey 1 Resolution",
		EffectiveAt:    timestamptz(boundary1.Add(2 * time.Hour)),
		StartsAt:       timestamptz(boundary1),
		EndsAt:         timestamptz(boundary1.Add(2 * time.Hour)),
		Status:         "resolved",
		SourceRef:      pgtype.Text{String: "discord://journey/1", Valid: true},
		Metadata:       emptyJSONB,
		ActivityID:     activity.ID,
	})
	if err != nil {
		t.Fatalf("create occurrence: %v", err)
	}
	if _, err := queries.CreateActivityOccurrenceGroup(ctx, db.CreateActivityOccurrenceGroupParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantGroupID:   group.ID,
		Role:                 "recipient_group",
		Result:               "won",
		Metadata:             emptyJSONB,
	}); err != nil {
		t.Fatalf("create occurrence group: %v", err)
	}
	if _, err := queries.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantID:        participantB.ID,
		ParticipantGroupID:   group.ID,
		Role:                 "delegate",
		Result:               "share",
		Metadata:             []byte(`{"choice":"SHARE"}`),
	}); err != nil {
		t.Fatalf("create occurrence participant: %v", err)
	}

	occurrenceGroups, err := queries.ListActivityOccurrenceGroups(ctx, occurrence.ID)
	if err != nil {
		t.Fatalf("list occurrence groups: %v", err)
	}
	if len(occurrenceGroups) != 1 || occurrenceGroups[0].ParticipantGroupID != group.ID {
		t.Fatalf("expected one persisted occurrence group, got %+v", occurrenceGroups)
	}

	occurrenceParticipants, err := queries.ListActivityOccurrenceParticipants(ctx, occurrence.ID)
	if err != nil {
		t.Fatalf("list occurrence participants: %v", err)
	}
	if len(occurrenceParticipants) != 1 || occurrenceParticipants[0].ParticipantID != participantB.ID {
		t.Fatalf("expected one persisted occurrence participant, got %+v", occurrenceParticipants)
	}

	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "award", 2, "public", "journey award", "attendance")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "spend", -1, "public", "spent public point", "spent-public")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "award", 3, "secret", "secret risk award", "secret-award")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "spend", -2, "secret", "secret spend", "spent-secret")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "reveal", 1, "revealed", "revealed point", "reveal-1")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantB.ID, occurrence.ID, group.ID, "correction", -1, "public", "manual correction", "correction-1")

	visibleTotal, err := service.VisibleBonusTotalByParticipant(ctx, instance.ID, participantB.ID)
	if err != nil {
		t.Fatalf("visible bonus total: %v", err)
	}
	if visibleTotal != 1 {
		t.Fatalf("expected visible total 1, got %d", visibleTotal)
	}

	secretTotal, err := service.SecretBonusTotalByParticipant(ctx, instance.ID, participantB.ID)
	if err != nil {
		t.Fatalf("secret bonus total: %v", err)
	}
	if secretTotal != 1 {
		t.Fatalf("expected secret total 1, got %d", secretTotal)
	}

	visibleAsOf, err := service.VisibleBonusTotalByParticipantAsOf(ctx, instance.ID, participantB.ID, time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("visible bonus total as-of: %v", err)
	}
	if visibleAsOf != 1 {
		t.Fatalf("expected visible as-of total 1, got %d", visibleAsOf)
	}

	availableSecret, err := service.AvailableSecretBalanceByParticipant(ctx, instance.ID, participantB.ID)
	if err != nil {
		t.Fatalf("available secret balance: %v", err)
	}
	if availableSecret != 1 {
		t.Fatalf("expected available secret balance 1, got %d", availableSecret)
	}

	visibleEntries, err := queries.ListVisibleBonusPointLedgerEntriesForParticipant(ctx, db.ListVisibleBonusPointLedgerEntriesForParticipantParams{
		InstanceID:    instance.ID,
		ParticipantID: participantB.ID,
	})
	if err != nil {
		t.Fatalf("list visible ledger entries: %v", err)
	}
	if len(visibleEntries) != 4 {
		t.Fatalf("expected 4 visible ledger entries, got %d", len(visibleEntries))
	}
	for _, entry := range visibleEntries {
		if entry.Visibility == "secret" {
			t.Fatalf("visible query should exclude secret entries")
		}
	}

	allEntries, err := queries.ListAllBonusPointLedgerEntriesForParticipant(ctx, db.ListAllBonusPointLedgerEntriesForParticipantParams{
		InstanceID:    instance.ID,
		ParticipantID: participantB.ID,
	})
	if err != nil {
		t.Fatalf("list all ledger entries: %v", err)
	}
	if len(allEntries) != 6 {
		t.Fatalf("expected 6 ledger entries, got %d", len(allEntries))
	}
}

func integrationPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("CASTAWAY_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set CASTAWAY_TEST_DATABASE_URL or run `mise run integration` to execute integration tests")
	}

	ctx := context.Background()
	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("create admin pool: %v", err)
	}

	databaseName := "castaway_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, databaseName)); err != nil {
		adminPool.Close()
		t.Fatalf("create temp database: %v", err)
	}

	testConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse temp database url: %v", err)
	}
	testConfig.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		if _, dropErr := adminPool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, databaseName)); dropErr != nil {
			t.Logf("drop temp database after pool creation failure: %v", dropErr)
		}
		adminPool.Close()
		t.Fatalf("create temp pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := adminPool.Exec(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, databaseName)); err != nil {
			t.Logf("drop temp database cleanup: %v", err)
		}
		adminPool.Close()
	})

	return ctx, pool
}

func resetDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if err := appinternal.RunMigrations(ctx, pool, "../../db/migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}

func createInstanceForTest(t *testing.T, ctx context.Context, queries *db.Queries, name string, season int32) db.CreateInstanceRow {
	t.Helper()
	instance, err := queries.CreateInstance(ctx, db.CreateInstanceParams{Name: name, Season: season})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	return instance
}

func createParticipantForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, name string) db.CreateParticipantRow {
	t.Helper()
	participant, err := queries.CreateParticipant(ctx, db.CreateParticipantParams{InstanceID: instanceID, Name: name})
	if err != nil {
		t.Fatalf("create participant %q: %v", name, err)
	}
	return participant
}

func createGroupForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, name string, kind string) db.CreateParticipantGroupRow {
	t.Helper()
	group, err := queries.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{
		InstanceID: instanceID,
		Name:       name,
		Kind:       kind,
		Metadata:   emptyJSONB,
	})
	if err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	return group
}

func createActivityForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, startsAt time.Time, endsAt *time.Time) db.CreateInstanceActivityRow {
	t.Helper()
	activity, err := queries.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
		InstanceID:   instanceID,
		ActivityType: "journey",
		Name:         "Journey 1",
		Status:       "active",
		StartsAt:     timestamptz(startsAt),
		EndsAt:       optionalTimestamptz(endsAt),
		Metadata:     emptyJSONB,
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	return activity
}

func createLedgerEntryForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID, participantID, occurrenceID, sourceGroupID pgtype.UUID, entryKind string, points int32, visibility string, reason string, awardKey string) {
	t.Helper()
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instanceID,
		ParticipantID:        participantID,
		ActivityOccurrenceID: occurrenceID,
		SourceGroupID:        sourceGroupID,
		EntryKind:            entryKind,
		Points:               points,
		Visibility:           visibility,
		Reason:               reason,
		EffectiveAt:          timestamptz(time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)),
		AwardKey:             pgtype.Text{String: awardKey, Valid: true},
		Metadata:             emptyJSONB,
	}); err != nil {
		t.Fatalf("create ledger entry %q: %v", awardKey, err)
	}
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
