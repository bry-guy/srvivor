package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	appinternal "github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testEmptyJSONB = []byte("{}")

type leaderboardResponse struct {
	Leaderboard []struct {
		ParticipantID   string `json:"participant_id"`
		ParticipantName string `json:"participant_name"`
		Score           int    `json:"score"`
		DraftPoints     int    `json:"draft_points"`
		BonusPoints     int    `json:"bonus_points"`
		TotalPoints     int    `json:"total_points"`
		PointsAvailable int    `json:"points_available"`
	} `json:"leaderboard"`
}

type bonusLedgerResponse struct {
	Participant struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"participant"`
	BonusPoints int `json:"bonus_points"`
	Ledger      []struct {
		ID              string  `json:"id"`
		ActivityID      string  `json:"activity_id"`
		ActivityType    string  `json:"activity_type"`
		ActivityName    string  `json:"activity_name"`
		OccurrenceID    string  `json:"activity_occurrence_id"`
		OccurrenceType  string  `json:"occurrence_type"`
		OccurrenceName  string  `json:"occurrence_name"`
		SourceGroupID   *string `json:"source_group_id"`
		SourceGroupName *string `json:"source_group_name"`
		EntryKind       string  `json:"entry_kind"`
		Points          int     `json:"points"`
		Visibility      string  `json:"visibility"`
		Reason          string  `json:"reason"`
		EffectiveAt     string  `json:"effective_at"`
		AwardKey        *string `json:"award_key"`
		CreatedAt       string  `json:"created_at"`
	} `json:"ledger"`
}

func TestServiceAuthProtectsNonHealthRoutes(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Auth Integration", 50)

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      true,
		BearerTokens: []string{"top-secret-token"},
	}))
	router := server.Router()

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	router.ServeHTTP(healthRecorder, healthReq)
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, body = %s", healthRecorder.Code, healthRecorder.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/instances", nil)
	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingRecorder, missingReq)
	if missingRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d, body = %s", missingRecorder.Code, missingRecorder.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/instances", nil)
	invalidReq.Header.Set("Authorization", "Bearer wrong-token")
	invalidRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidRecorder, invalidReq)
	if invalidRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid auth status = %d, body = %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	validReq := httptest.NewRequest(http.MethodGet, "/instances", nil)
	validReq.Header.Set("Authorization", "Bearer top-secret-token")
	validRecorder := httptest.NewRecorder()
	router.ServeHTTP(validRecorder, validReq)
	if validRecorder.Code != http.StatusOK {
		t.Fatalf("valid auth status = %d, body = %s", validRecorder.Code, validRecorder.Body.String())
	}
	if !strings.Contains(validRecorder.Body.String(), uuid.UUID(instance.ID.Bytes).String()) {
		t.Fatalf("expected instances response to include created instance, body = %s", validRecorder.Body.String())
	}
}

func TestLeaderboardAndBonusLedgerHideSecretPoints(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "API Integration", 50)
	alice := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	bob := createParticipantForTest(t, ctx, queries, instance.ID, "Bob")
	contestantA := createContestantForTest(t, ctx, queries, instance.ID, "Contestant A")
	contestantB := createContestantForTest(t, ctx, queries, instance.ID, "Contestant B")

	createDraftPickForTest(t, ctx, queries, instance.ID, alice.ID, contestantA.ID, 1)
	createDraftPickForTest(t, ctx, queries, instance.ID, alice.ID, contestantB.ID, 2)
	createDraftPickForTest(t, ctx, queries, instance.ID, bob.ID, contestantA.ID, 2)
	createDraftPickForTest(t, ctx, queries, instance.ID, bob.ID, contestantB.ID, 1)

	upsertOutcomeForTest(t, ctx, queries, instance.ID, 1, contestantA.ID)
	upsertOutcomeForTest(t, ctx, queries, instance.ID, 2, contestantB.ID)

	effectiveAt := time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)
	activity := createActivityForTest(t, ctx, queries, instance.ID, effectiveAt, nil, "journey", "Journey 1")
	occurrence := createOccurrenceForTest(t, ctx, queries, activity.ID, "journey_resolution", "Journey 1 Resolution", effectiveAt)

	createLedgerEntryForTest(t, ctx, queries, instance.ID, alice.ID, occurrence.ID, pgtype.UUID{}, "award", 2, "public", "public award", "alice-public")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, alice.ID, occurrence.ID, pgtype.UUID{}, "award", 5, "secret", "secret award", "alice-secret")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, bob.ID, occurrence.ID, pgtype.UUID{}, "award", 1, "public", "public award", "bob-public")

	server := httpapi.New(pool)
	router := server.Router()

	leaderboardReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/leaderboard", uuid.UUID(instance.ID.Bytes).String()), nil)
	leaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(leaderboardRecorder, leaderboardReq)
	if leaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("leaderboard status = %d, body = %s", leaderboardRecorder.Code, leaderboardRecorder.Body.String())
	}

	var leaderboard leaderboardResponse
	if err := json.Unmarshal(leaderboardRecorder.Body.Bytes(), &leaderboard); err != nil {
		t.Fatalf("unmarshal leaderboard response: %v", err)
	}
	if len(leaderboard.Leaderboard) != 2 {
		t.Fatalf("expected 2 leaderboard rows, got %d", len(leaderboard.Leaderboard))
	}
	if leaderboard.Leaderboard[0].ParticipantName != "Alice" {
		t.Fatalf("expected Alice first, got %+v", leaderboard.Leaderboard[0])
	}
	if leaderboard.Leaderboard[0].DraftPoints != 3 || leaderboard.Leaderboard[0].BonusPoints != 2 || leaderboard.Leaderboard[0].TotalPoints != 5 || leaderboard.Leaderboard[0].Score != 5 {
		t.Fatalf("unexpected Alice leaderboard totals: %+v", leaderboard.Leaderboard[0])
	}
	if leaderboard.Leaderboard[1].ParticipantName != "Bob" || leaderboard.Leaderboard[1].TotalPoints != 2 {
		t.Fatalf("unexpected Bob leaderboard row: %+v", leaderboard.Leaderboard[1])
	}

	bonusLedgerReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String()), nil)
	bonusLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(bonusLedgerRecorder, bonusLedgerReq)
	if bonusLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("bonus ledger status = %d, body = %s", bonusLedgerRecorder.Code, bonusLedgerRecorder.Body.String())
	}

	var bonusLedger bonusLedgerResponse
	if err := json.Unmarshal(bonusLedgerRecorder.Body.Bytes(), &bonusLedger); err != nil {
		t.Fatalf("unmarshal bonus ledger response: %v", err)
	}
	if bonusLedger.Participant.Name != "Alice" {
		t.Fatalf("expected Alice bonus ledger participant, got %+v", bonusLedger.Participant)
	}
	if bonusLedger.BonusPoints != 2 {
		t.Fatalf("expected visible bonus total 2, got %d", bonusLedger.BonusPoints)
	}
	if len(bonusLedger.Ledger) != 1 {
		t.Fatalf("expected only visible ledger entry, got %d", len(bonusLedger.Ledger))
	}
	if bonusLedger.Ledger[0].Visibility != "public" || bonusLedger.Ledger[0].Points != 2 {
		t.Fatalf("unexpected public ledger entry: %+v", bonusLedger.Ledger[0])
	}
}

func integrationPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("CASTAWAY_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set CASTAWAY_TEST_DATABASE_URL to run integration tests")
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

	databaseName := "castaway_httpapi_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func createContestantForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, name string) db.CreateContestantRow {
	t.Helper()
	contestant, err := queries.CreateContestant(ctx, db.CreateContestantParams{InstanceID: instanceID, Name: name})
	if err != nil {
		t.Fatalf("create contestant %q: %v", name, err)
	}
	return contestant
}

func createDraftPickForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID, participantID, contestantID pgtype.UUID, position int32) {
	t.Helper()
	if _, err := queries.CreateDraftPick(ctx, db.CreateDraftPickParams{
		InstanceID:    instanceID,
		ParticipantID: participantID,
		ContestantID:  contestantID,
		Position:      position,
	}); err != nil {
		t.Fatalf("create draft pick %d: %v", position, err)
	}
}

func upsertOutcomeForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, position int32, contestantID pgtype.UUID) {
	t.Helper()
	if _, err := queries.UpsertOutcomePosition(ctx, db.UpsertOutcomePositionParams{
		InstanceID:   instanceID,
		Position:     position,
		ContestantID: contestantID,
	}); err != nil {
		t.Fatalf("upsert outcome %d: %v", position, err)
	}
}

func createActivityForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, startsAt time.Time, endsAt *time.Time, activityType string, name string) db.CreateInstanceActivityRow {
	t.Helper()
	activity, err := queries.CreateInstanceActivity(ctx, db.CreateInstanceActivityParams{
		InstanceID:   instanceID,
		ActivityType: activityType,
		Name:         name,
		Status:       "active",
		StartsAt:     timestamptz(startsAt),
		EndsAt:       optionalTimestamptz(endsAt),
		Metadata:     testEmptyJSONB,
	})
	if err != nil {
		t.Fatalf("create activity %q: %v", name, err)
	}
	return activity
}

func createOccurrenceForTest(t *testing.T, ctx context.Context, queries *db.Queries, activityID pgtype.UUID, occurrenceType string, name string, effectiveAt time.Time) db.CreateActivityOccurrenceRow {
	t.Helper()
	occurrence, err := queries.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID:     activityID,
		OccurrenceType: occurrenceType,
		Name:           name,
		EffectiveAt:    timestamptz(effectiveAt),
		StartsAt:       timestamptz(effectiveAt),
		EndsAt:         timestamptz(effectiveAt.Add(time.Hour)),
		Status:         "resolved",
		SourceRef:      pgtype.Text{},
		Metadata:       testEmptyJSONB,
	})
	if err != nil {
		t.Fatalf("create occurrence %q: %v", name, err)
	}
	return occurrence
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
		EffectiveAt:          timestamptz(time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)),
		AwardKey:             pgtype.Text{String: awardKey, Valid: true},
		Metadata:             testEmptyJSONB,
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
