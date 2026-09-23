package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	appinternal "github.com/bry-guy/srvivor/apps/castaway-web/internal/app"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/seeddata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testEmptyJSONB = []byte("{}")

func verificationGameplayNow() time.Time {
	return time.Date(2026, time.March, 5, 12, 0, 0, 0, time.UTC)
}

type leaderboardResponse struct {
	Leaderboard []struct {
		ParticipantID            string `json:"participant_id"`
		ParticipantName          string `json:"participant_name"`
		ParticipantDiscordUserID string `json:"participant_discord_user_id"`
		CurrentTribeName         string `json:"current_tribe_name"`
		Score                    int    `json:"score"`
		DraftPoints              int    `json:"draft_points"`
		BonusPoints              int    `json:"bonus_points"`
		TotalPoints              int    `json:"total_points"`
		PointsAvailable          int    `json:"points_available"`
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

type activitiesResponse struct {
	Activities []struct {
		ID           string          `json:"id"`
		InstanceID   string          `json:"instance_id"`
		ActivityType string          `json:"activity_type"`
		Name         string          `json:"name"`
		Status       string          `json:"status"`
		StartsAt     string          `json:"starts_at"`
		EndsAt       *string         `json:"ends_at"`
		Metadata     json.RawMessage `json:"metadata"`
	} `json:"activities"`
}

type occurrencesResponse struct {
	Occurrences []struct {
		ID             string          `json:"id"`
		ActivityID     string          `json:"activity_id"`
		OccurrenceType string          `json:"occurrence_type"`
		Name           string          `json:"name"`
		EffectiveAt    string          `json:"effective_at"`
		StartsAt       *string         `json:"starts_at"`
		EndsAt         *string         `json:"ends_at"`
		Status         string          `json:"status"`
		SourceRef      *string         `json:"source_ref"`
		Metadata       json.RawMessage `json:"metadata"`
	} `json:"occurrences"`
}

type activityDetailResponse struct {
	Activity struct {
		ID string `json:"id"`
	} `json:"activity"`
	GroupAssignments []struct {
		ParticipantGroupID   string          `json:"participant_group_id"`
		ParticipantGroupName string          `json:"participant_group_name"`
		Role                 string          `json:"role"`
		Configuration        json.RawMessage `json:"configuration"`
	} `json:"group_assignments"`
	ParticipantAssignments []struct {
		ParticipantID        string  `json:"participant_id"`
		ParticipantName      string  `json:"participant_name"`
		ParticipantGroupID   *string `json:"participant_group_id"`
		ParticipantGroupName *string `json:"participant_group_name"`
		Role                 string  `json:"role"`
	} `json:"participant_assignments"`
}

type occurrenceDetailResponse struct {
	Activity struct {
		ID string `json:"id"`
	} `json:"activity"`
	Occurrence struct {
		ID string `json:"id"`
	} `json:"occurrence"`
	Participants []struct {
		ParticipantID        string  `json:"participant_id"`
		ParticipantName      string  `json:"participant_name"`
		ParticipantGroupID   *string `json:"participant_group_id"`
		ParticipantGroupName *string `json:"participant_group_name"`
		Role                 string  `json:"role"`
		Result               string  `json:"result"`
	} `json:"participants"`
	Groups []struct {
		ParticipantGroupID   string `json:"participant_group_id"`
		ParticipantGroupName string `json:"participant_group_name"`
		Role                 string `json:"role"`
		Result               string `json:"result"`
	} `json:"groups"`
	Ledger []struct {
		ParticipantID   string `json:"participant_id"`
		ParticipantName string `json:"participant_name"`
		EntryKind       string `json:"entry_kind"`
		Points          int    `json:"points"`
		Visibility      string `json:"visibility"`
	} `json:"ledger"`
}

type participantActivityHistoryResponse struct {
	Participant struct {
		ID string `json:"id"`
	} `json:"participant"`
	Instance struct {
		ID string `json:"id"`
	} `json:"instance"`
	Activities []struct {
		Activity struct {
			ID string `json:"id"`
		} `json:"activity"`
		Occurrences []struct {
			Occurrence struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"occurrence"`
			Involvement *struct {
				ParticipantID string `json:"participant_id"`
				Role          string `json:"role"`
				Result        string `json:"result"`
			} `json:"involvement"`
			Ledger []struct {
				ParticipantID string `json:"participant_id"`
				Points        int    `json:"points"`
				Visibility    string `json:"visibility"`
			} `json:"ledger"`
		} `json:"occurrences"`
	} `json:"activities"`
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

func TestManagedWritesFailClosedAndImportPreservesInstance(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      true,
		BearerTokens: []string{"managed-token"},
	}))
	router := server.Router()
	createBody := `{"name":"Managed protection","season":100,"managed_progression":true,"contestants":["Protected C1","Protected C2"],"episodes":[{"episode_number":0,"label":"Preseason","airs_at":"2026-02-01T20:00:00-05:00"},{"episode_number":1,"label":"Episode 1","airs_at":"2026-02-08T20:00:00-05:00"}]}`
	createReq := authorizedJSONRequest(http.MethodPost, "/instances", createBody, "managed-token", "managed-admin")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("managed instance creation status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode managed instance: %v", err)
	}
	instanceID, err := uuid.Parse(created.Instance.ID)
	if err != nil {
		t.Fatalf("parse managed instance id: %v", err)
	}
	publicID := pgtype.UUID{Bytes: instanceID, Valid: true}
	queries := db.New(pool)

	noAuthRouter := httpapi.New(pool).Router()
	forgedProgression := authorizedJSONRequest(http.MethodPost, "/instances/"+created.Instance.ID+"/progression/draft/open", `{"idempotency_key":"forged-open","effective_at":"2026-02-02T20:00:00Z"}`, "", "managed-admin")
	forgedProgressionRecorder := httptest.NewRecorder()
	noAuthRouter.ServeHTTP(forgedProgressionRecorder, forgedProgression)
	if forgedProgressionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("forged managed progression status = %d, body = %s", forgedProgressionRecorder.Code, forgedProgressionRecorder.Body.String())
	}
	draftProgress, err := queries.GetInstanceDraftProgress(ctx, publicID)
	if err != nil {
		t.Fatalf("read managed draft state: %v", err)
	}
	if draftProgress.Status != "pending" {
		t.Fatalf("forged request changed draft state to %q", draftProgress.Status)
	}

	activityReq := authorizedJSONRequest(http.MethodPost, "/instances/"+created.Instance.ID+"/activities", `{"activity_type":"tribe_wordle","name":"forged","status":"active","starts_at":"2026-02-02T20:00:00Z"}`, "", "managed-admin")
	activityRecorder := httptest.NewRecorder()
	noAuthRouter.ServeHTTP(activityRecorder, activityReq)
	if activityRecorder.Code != http.StatusConflict {
		t.Fatalf("managed activity bypass status = %d, body = %s", activityRecorder.Code, activityRecorder.Body.String())
	}
	activities, err := queries.ListInstanceActivitiesByInstance(ctx, publicID)
	if err != nil {
		t.Fatalf("list managed activities: %v", err)
	}
	if len(activities) != 0 {
		t.Fatalf("forged activity request created %d activities", len(activities))
	}

	participantReq := authorizedJSONRequest(http.MethodPost, "/instances/"+created.Instance.ID+"/participants", `{"name":"Protected Alice"}`, "managed-token", "managed-admin")
	participantRecorder := httptest.NewRecorder()
	router.ServeHTTP(participantRecorder, participantReq)
	if participantRecorder.Code != http.StatusCreated {
		t.Fatalf("managed participant setup status = %d, body = %s", participantRecorder.Code, participantRecorder.Body.String())
	}
	participants, err := queries.ListParticipantsByInstance(ctx, publicID)
	if err != nil {
		t.Fatalf("list managed participants: %v", err)
	}
	if len(participants) != 1 {
		t.Fatalf("expected one managed participant, got %d", len(participants))
	}
	participantID := uuid.UUID(participants[0].ID.Bytes).String()
	linkReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/participants/%s/discord-link", created.Instance.ID, participantID), `{"discord_user_id":"forged-user"}`, "", "managed-admin")
	linkRecorder := httptest.NewRecorder()
	noAuthRouter.ServeHTTP(linkRecorder, linkReq)
	if linkRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("forged managed link status = %d, body = %s", linkRecorder.Code, linkRecorder.Body.String())
	}
	linkedParticipant, err := queries.GetParticipant(ctx, participants[0].ID)
	if err != nil {
		t.Fatalf("read managed participant after forged link: %v", err)
	}
	if linkedParticipant.DiscordUserID.Valid {
		t.Fatalf("forged managed link changed discord identity to %q", linkedParticipant.DiscordUserID.String)
	}
	unlinkReq := authorizedJSONRequest(http.MethodDelete, fmt.Sprintf("/instances/%s/participants/%s/discord-link", created.Instance.ID, participantID), "", "", "managed-admin")
	unlinkRecorder := httptest.NewRecorder()
	noAuthRouter.ServeHTTP(unlinkRecorder, unlinkReq)
	if unlinkRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("forged managed unlink status = %d, body = %s", unlinkRecorder.Code, unlinkRecorder.Body.String())
	}

	startReq := authorizedJSONRequest(http.MethodPost, "/instances/"+created.Instance.ID+"/progression/episodes/1/start", `{"idempotency_key":"start-1","effective_at":"2026-02-03T20:00:00Z"}`, "managed-token", "managed-admin")
	startRecorder := httptest.NewRecorder()
	router.ServeHTTP(startRecorder, startReq)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("start managed episode status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	lateParticipantReq := authorizedJSONRequest(http.MethodPost, "/instances/"+created.Instance.ID+"/participants", `{"name":"Too Late"}`, "managed-token", "managed-admin")
	lateParticipantRecorder := httptest.NewRecorder()
	router.ServeHTTP(lateParticipantRecorder, lateParticipantReq)
	if lateParticipantRecorder.Code != http.StatusConflict {
		t.Fatalf("late participant setup status = %d, body = %s", lateParticipantRecorder.Code, lateParticipantRecorder.Body.String())
	}
	participants, err = queries.ListParticipantsByInstance(ctx, publicID)
	if err != nil {
		t.Fatalf("relist managed participants: %v", err)
	}
	if len(participants) != 1 {
		t.Fatalf("late setup changed participant count to %d", len(participants))
	}

	importReq := authorizedJSONRequest(http.MethodPost, "/instances/import", `{"name":"Managed protection","season":100,"submissions":[{"participant_name":"Imported","rankings":["Protected C1","Protected C2"]}]}`, "managed-token", "managed-admin")
	importRecorder := httptest.NewRecorder()
	router.ServeHTTP(importRecorder, importReq)
	if importRecorder.Code != http.StatusConflict {
		t.Fatalf("managed import protection status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
	instanceAfterImport, err := queries.GetInstance(ctx, publicID)
	if err != nil {
		t.Fatalf("managed instance missing after rejected import: %v", err)
	}
	if instanceAfterImport.Name != "Managed protection" {
		t.Fatalf("managed instance changed after rejected import: %+v", instanceAfterImport)
	}
}

func TestManagedProgressionPublishesAndCorrectsWithoutLeakingPendingState(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      true,
		BearerTokens: []string{"managed-token"},
	}))
	router := server.Router()
	createBody := `{"name":"Managed progression","season":99,"managed_progression":true,"contestants":["Managed C1","Managed C2","Managed C3"],"episodes":[{"episode_number":0,"label":"Preseason","airs_at":"2026-01-01T20:00:00-05:00"},{"episode_number":1,"label":"Episode 1","airs_at":"2026-01-08T20:00:00-05:00"},{"episode_number":2,"label":"Episode 2","airs_at":"2026-01-15T20:00:00-05:00"}]}`
	createReq := authorizedJSONRequest(http.MethodPost, "/instances", createBody, "managed-token", "managed-admin")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createReq)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("managed instance creation status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode managed instance: %v", err)
	}
	instanceUUID, err := uuid.Parse(created.Instance.ID)
	if err != nil {
		t.Fatalf("parse managed instance id: %v", err)
	}
	instanceID := pgtype.UUID{Bytes: instanceUUID, Valid: true}
	queries := db.New(pool)
	participants := []db.CreateParticipantRow{
		createParticipantForTest(t, ctx, queries, instanceID, "Managed Alice"),
		createParticipantForTest(t, ctx, queries, instanceID, "Managed Bob"),
	}
	contestants, err := queries.ListContestantsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("list managed contestants: %v", err)
	}
	if len(contestants) != 3 {
		t.Fatalf("expected 3 managed contestants, got %d", len(contestants))
	}
	contestantIDs := make([]string, 0, len(contestants))
	for _, contestant := range contestants {
		contestantIDs = append(contestantIDs, uuid.UUID(contestant.ID.Bytes).String())
	}
	postScoreActivity := createActivityForTest(t, ctx, queries, instanceID, time.Date(2026, time.January, 9, 19, 0, 0, 0, time.UTC), nil, "manual_adjustment", "Post-score state")
	postScoreOccurrence := createOccurrenceForTest(t, ctx, queries, postScoreActivity.ID, "post_score", "Post-score state", time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC))
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID: instanceID, ParticipantID: participants[0].ID, ActivityOccurrenceID: postScoreOccurrence.ID, EntryKind: "award", Points: 4, Visibility: "public", Reason: "post-score public", EffectiveAt: timestamptz(time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC)), AwardKey: pgtype.Text{String: "post-score-public", Valid: true}, Metadata: testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create post-score public ledger entry: %v", err)
	}
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID: instanceID, ParticipantID: participants[1].ID, ActivityOccurrenceID: postScoreOccurrence.ID, EntryKind: "award", Points: 3, Visibility: "secret", Reason: "post-score secret", EffectiveAt: timestamptz(time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC)), AwardKey: pgtype.Text{String: "post-score-secret", Valid: true}, Metadata: testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create post-score secret ledger entry: %v", err)
	}
	if _, err := queries.CreateParticipantPonyOwnership(ctx, db.CreateParticipantPonyOwnershipParams{
		InstanceID: instanceID, OwnerParticipantID: participants[0].ID, ContestantID: contestants[0].ID, SourceActivityOccurrenceID: postScoreOccurrence.ID, AcquiredAt: timestamptz(time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC)), Status: "active", Metadata: testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create post-score ownership: %v", err)
	}
	effective := func(key, at string) string {
		return fmt.Sprintf(`{"idempotency_key":%q,"effective_at":%q}`, key, at)
	}
	progress := func(path, body string) *httptest.ResponseRecorder {
		req := authorizedJSONRequest(http.MethodPost, path, body, "managed-token", "managed-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	instancePath := created.Instance.ID

	openBody := effective("draft-open", "2026-01-02T20:00:00Z")
	openRecorder := progress("/instances/"+instancePath+"/progression/draft/open", openBody)
	if openRecorder.Code != http.StatusOK {
		t.Fatalf("open draft status = %d, body = %s", openRecorder.Code, openRecorder.Body.String())
	}
	retryRecorder := progress("/instances/"+instancePath+"/progression/draft/open", openBody)
	var originalResponse, retryResponse map[string]any
	if err := json.Unmarshal(openRecorder.Body.Bytes(), &originalResponse); err != nil {
		t.Fatalf("decode original idempotent response: %v", err)
	}
	if err := json.Unmarshal(retryRecorder.Body.Bytes(), &retryResponse); err != nil {
		t.Fatalf("decode retry idempotent response: %v", err)
	}
	if retryRecorder.Code != http.StatusOK || !reflect.DeepEqual(retryResponse, originalResponse) {
		t.Fatalf("idempotent open retry = %d %s, original = %s", retryRecorder.Code, retryRecorder.Body.String(), openRecorder.Body.String())
	}
	conflictRecorder := progress("/instances/"+instancePath+"/progression/draft/open", effective("draft-open", "2026-01-03T20:00:00Z"))
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("conflicting idempotency retry status = %d, body = %s", conflictRecorder.Code, conflictRecorder.Body.String())
	}

	startOne := progress("/instances/"+instancePath+"/progression/episodes/1/start", effective("episode-1-start", "2026-01-03T20:00:00Z"))
	if startOne.Code != http.StatusOK {
		t.Fatalf("start episode 1 status = %d, body = %s", startOne.Code, startOne.Body.String())
	}
	startTwoEarly := progress("/instances/"+instancePath+"/progression/episodes/2/start", effective("episode-2-start-early", "2026-01-03T20:01:00Z"))
	if startTwoEarly.Code != http.StatusConflict {
		t.Fatalf("early episode 2 start status = %d, body = %s", startTwoEarly.Code, startTwoEarly.Body.String())
	}

	draftBody := func(key, at string, order []string) string {
		return fmt.Sprintf(`{"contestant_ids":[%q,%q,%q],"idempotency_key":%q,"effective_at":%q}`, order[0], order[1], order[2], key, at)
	}
	for index, participant := range participants {
		order := contestantIDs
		if index == 1 {
			order = []string{contestantIDs[1], contestantIDs[0], contestantIDs[2]}
		}
		req := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/drafts/%s", instancePath, uuid.UUID(participant.ID.Bytes).String()), draftBody(fmt.Sprintf("draft-%d", index), "2026-01-03T20:02:00Z", order), "managed-token", "managed-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("draft %d status = %d, body = %s", index, recorder.Code, recorder.Body.String())
		}
	}

	outcomeBody := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"outcome-1","effective_at":"2026-01-04T20:00:00Z"}`, contestantIDs[0])
	outcomeReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), outcomeBody, "managed-token", "managed-admin")
	outcomeRecorder := httptest.NewRecorder()
	router.ServeHTTP(outcomeRecorder, outcomeReq)
	if outcomeRecorder.Code != http.StatusOK {
		t.Fatalf("outcome status = %d, body = %s", outcomeRecorder.Code, outcomeRecorder.Body.String())
	}
	completeOne := progress("/instances/"+instancePath+"/progression/episodes/1/complete", effective("episode-1-complete", "2026-01-05T20:00:00Z"))
	if completeOne.Code != http.StatusOK {
		t.Fatalf("complete episode 1 status = %d, body = %s", completeOne.Code, completeOne.Body.String())
	}
	scoreOne := progress("/instances/"+instancePath+"/progression/episodes/1/score", effective("episode-1-score", "2026-01-06T20:00:00Z"))
	if scoreOne.Code != http.StatusOK {
		t.Fatalf("score episode 1 status = %d, body = %s", scoreOne.Code, scoreOne.Body.String())
	}
	outcomesBeforeUnflagged, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes before unflagged post-score write: %v", err)
	}
	var revisionsBeforeUnflagged int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&revisionsBeforeUnflagged); err != nil {
		t.Fatalf("count revisions before unflagged post-score write: %v", err)
	}
	unflaggedPostScoreBody := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"outcome-after-score-without-correction","effective_at":"2026-01-06T20:15:00Z"}`, contestantIDs[1])
	unflaggedPostScoreReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), unflaggedPostScoreBody, "managed-token", "managed-admin")
	unflaggedPostScoreRecorder := httptest.NewRecorder()
	router.ServeHTTP(unflaggedPostScoreRecorder, unflaggedPostScoreReq)
	var unflaggedPostScoreResponse struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(unflaggedPostScoreRecorder.Body.Bytes(), &unflaggedPostScoreResponse); err != nil {
		t.Fatalf("decode unflagged post-score response: %v", err)
	}
	if unflaggedPostScoreRecorder.Code != http.StatusConflict || unflaggedPostScoreResponse.Error != "use correction: true to fix published outcomes or start the next episode to record new outcomes" {
		t.Fatalf("unflagged post-score write = %d %s", unflaggedPostScoreRecorder.Code, unflaggedPostScoreRecorder.Body.String())
	}
	outcomesAfterUnflagged, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes after unflagged post-score write: %v", err)
	}
	if !reflect.DeepEqual(outcomesAfterUnflagged, outcomesBeforeUnflagged) {
		t.Fatalf("unflagged post-score write changed outcomes: before=%+v after=%+v", outcomesBeforeUnflagged, outcomesAfterUnflagged)
	}
	var revisionsAfterUnflagged int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&revisionsAfterUnflagged); err != nil {
		t.Fatalf("count revisions after unflagged post-score write: %v", err)
	}
	if revisionsAfterUnflagged != revisionsBeforeUnflagged {
		t.Fatalf("unflagged post-score write changed revision count from %d to %d", revisionsBeforeUnflagged, revisionsAfterUnflagged)
	}

	leaderboardReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/leaderboard", "", "managed-token", "managed-admin")
	leaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(leaderboardRecorder, leaderboardReq)
	if leaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("managed leaderboard status = %d, body = %s", leaderboardRecorder.Code, leaderboardRecorder.Body.String())
	}
	var leaderboard leaderboardResponse
	if err := json.Unmarshal(leaderboardRecorder.Body.Bytes(), &leaderboard); err != nil {
		t.Fatalf("decode managed leaderboard: %v", err)
	}
	if len(leaderboard.Leaderboard) != len(participants) {
		t.Fatalf("expected %d published leaderboard rows, got %d", len(participants), len(leaderboard.Leaderboard))
	}
	publishedScores := make(map[string][3]int, len(leaderboard.Leaderboard))
	for _, row := range leaderboard.Leaderboard {
		publishedScores[row.ParticipantName] = [3]int{row.DraftPoints, row.BonusPoints, row.TotalPoints}
	}
	if publishedScores["Managed Alice"] != [3]int{3, 0, 3} || publishedScores["Managed Bob"] != [3]int{2, 0, 2} {
		t.Fatalf("unexpected initial published scores: %+v", publishedScores)
	}
	var initialSnapshotJSON []byte
	if err := pool.QueryRow(ctx, `
		SELECT isr.input_snapshot
		FROM instance_score_revisions isr
		JOIN instances i ON i.id = isr.instance_id
		WHERE i.public_id = $1 AND isr.revision_number = 1`, instanceID).Scan(&initialSnapshotJSON); err != nil {
		t.Fatalf("read initial score snapshot: %v", err)
	}

	correctionBody := draftBody("draft-correction", "2026-01-06T20:30:00Z", []string{contestantIDs[2], contestantIDs[1], contestantIDs[0]})
	correctionReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/drafts/%s", instancePath, uuid.UUID(participants[1].ID.Bytes).String()), correctionBody, "managed-token", "managed-admin")
	correctionRecorder := httptest.NewRecorder()
	router.ServeHTTP(correctionRecorder, correctionReq)
	if correctionRecorder.Code != http.StatusOK || !strings.Contains(correctionRecorder.Body.String(), "revision_number") {
		t.Fatalf("published draft correction = %d %s", correctionRecorder.Code, correctionRecorder.Body.String())
	}

	startTwo := progress("/instances/"+instancePath+"/progression/episodes/2/start", effective("episode-2-start", "2026-01-08T20:00:00Z"))
	if startTwo.Code != http.StatusOK {
		t.Fatalf("start episode 2 status = %d, body = %s", startTwo.Code, startTwo.Body.String())
	}
	activity := createActivityForTest(t, ctx, queries, instanceID, time.Date(2026, time.January, 9, 19, 0, 0, 0, time.UTC), nil, "manual_adjustment", "Pending episode 2 bonus")
	occurrence := createOccurrenceForTest(t, ctx, queries, activity.ID, "pending_bonus", "Pending episode 2 bonus", time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC))
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instanceID,
		ParticipantID:        participants[0].ID,
		ActivityOccurrenceID: occurrence.ID,
		SourceGroupID:        pgtype.UUID{},
		EntryKind:            "award",
		Points:               4,
		Visibility:           "public",
		Reason:               "pending episode 2 bonus",
		EffectiveAt:          timestamptz(time.Date(2026, time.January, 9, 20, 0, 0, 0, time.UTC)),
		AwardKey:             pgtype.Text{String: "pending-episode-2-bonus", Valid: true},
		Metadata:             testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create pending bonus: %v", err)
	}
	pendingOutcomeBody := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"outcome-2","effective_at":"2026-01-09T20:00:00Z"}`, contestantIDs[1])
	pendingOutcomeReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/2", instancePath), pendingOutcomeBody, "managed-token", "managed-admin")
	pendingOutcomeRecorder := httptest.NewRecorder()
	router.ServeHTTP(pendingOutcomeRecorder, pendingOutcomeReq)
	if pendingOutcomeRecorder.Code != http.StatusOK {
		t.Fatalf("pending outcome status = %d, body = %s", pendingOutcomeRecorder.Code, pendingOutcomeRecorder.Body.String())
	}
	episode2Before, err := queries.GetInstanceEpisodeProgress(ctx, db.GetInstanceEpisodeProgressParams{InstanceID: instanceID, EpisodeNumber: 2})
	if err != nil {
		t.Fatalf("read episode 2 before outcome correction: %v", err)
	}
	var ledgerBeforeCorrection int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&ledgerBeforeCorrection); err != nil {
		t.Fatalf("count ledger before outcome correction: %v", err)
	}
	bonusBeforeCorrection, err := queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[0].ID})
	if err != nil {
		t.Fatalf("read bonus before outcome correction: %v", err)
	}
	secretBeforeCorrection, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[1].ID})
	if err != nil {
		t.Fatalf("read secret bonus before outcome correction: %v", err)
	}
	ownershipBeforeCorrection, err := queries.ListActiveParticipantPonyOwnershipsByOwnerAt(ctx, db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{InstanceID: instanceID, OwnerParticipantID: participants[0].ID, At: timestamptz(verificationGameplayNow())})
	if err != nil {
		t.Fatalf("read ownership before outcome correction: %v", err)
	}
	var ledgerRowsBefore, ownershipRowsBefore []byte
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, participant_id, activity_occurrence_id, entry_kind, points, visibility, reason, effective_at, award_key, metadata, created_at FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ledgerRowsBefore); err != nil {
		t.Fatalf("snapshot ledger before outcome correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, owner_participant_id, contestant_id, source_activity_occurrence_id, acquired_at, released_at, status, metadata, created_at, updated_at FROM participant_pony_ownerships WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ownershipRowsBefore); err != nil {
		t.Fatalf("snapshot ownership before outcome correction: %v", err)
	}

	liveBeforeCorrectionConflicts, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes before correction conflicts: %v", err)
	}
	assertCorrectionConflict := func(position, contestantID, key string) {
		t.Helper()
		body := fmt.Sprintf(`{"contestant_id":%q,"correction":true,"idempotency_key":%q,"effective_at":"2026-01-09T20:30:00Z"}`, contestantID, key)
		req := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/%s", instancePath, position), body, "managed-token", "managed-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		var response struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode %s correction conflict: %v", key, err)
		}
		if recorder.Code != http.StatusConflict || response.Error != "outcome correction conflicts with pending outcome changes" {
			t.Fatalf("%s correction conflict = %d %s", key, recorder.Code, recorder.Body.String())
		}
	}
	assertCorrectionConflict("1", contestantIDs[1], "outcome-correction-contestant-conflict")
	assertCorrectionConflict("2", contestantIDs[2], "outcome-correction-position-conflict")
	liveAfterCorrectionConflicts, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes after correction conflicts: %v", err)
	}
	if !reflect.DeepEqual(liveAfterCorrectionConflicts, liveBeforeCorrectionConflicts) {
		t.Fatalf("correction conflicts changed live outcomes: before=%+v after=%+v", liveBeforeCorrectionConflicts, liveAfterCorrectionConflicts)
	}
	correctionOutcomeBody := fmt.Sprintf(`{"contestant_id":%q,"correction":true,"idempotency_key":"outcome-correction","effective_at":"2026-01-09T21:00:00Z"}`, contestantIDs[2])
	correctionOutcomeReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), correctionOutcomeBody, "managed-token", "managed-admin")
	correctionOutcomeRecorder := httptest.NewRecorder()
	router.ServeHTTP(correctionOutcomeRecorder, correctionOutcomeReq)
	if correctionOutcomeRecorder.Code != http.StatusOK || !strings.Contains(correctionOutcomeRecorder.Body.String(), "revision_number") {
		t.Fatalf("immediate outcome correction = %d %s", correctionOutcomeRecorder.Code, correctionOutcomeRecorder.Body.String())
	}
	var correctionResponse map[string]any
	if err := json.Unmarshal(correctionOutcomeRecorder.Body.Bytes(), &correctionResponse); err != nil {
		t.Fatalf("decode immediate outcome correction: %v", err)
	}
	correctionRetryRecorder := httptest.NewRecorder()
	router.ServeHTTP(correctionRetryRecorder, authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), correctionOutcomeBody, "managed-token", "managed-admin"))
	var correctionRetryResponse map[string]any
	if err := json.Unmarshal(correctionRetryRecorder.Body.Bytes(), &correctionRetryResponse); err != nil {
		t.Fatalf("decode immediate correction retry: %v", err)
	}
	if correctionRetryRecorder.Code != http.StatusOK || !reflect.DeepEqual(correctionRetryResponse, correctionResponse) {
		t.Fatalf("immediate correction retry = %d %s, original = %s", correctionRetryRecorder.Code, correctionRetryRecorder.Body.String(), correctionOutcomeRecorder.Body.String())
	}
	conflictingRetryBody := fmt.Sprintf(`{"contestant_id":%q,"correction":true,"idempotency_key":"outcome-correction","effective_at":"2026-01-09T21:00:00Z"}`, contestantIDs[0])
	conflictingRetryReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), conflictingRetryBody, "managed-token", "managed-admin")
	conflictingRetryRecorder := httptest.NewRecorder()
	router.ServeHTTP(conflictingRetryRecorder, conflictingRetryReq)
	if conflictingRetryRecorder.Code != http.StatusConflict || !strings.Contains(conflictingRetryRecorder.Body.String(), "idempotency key was already used with a different payload") {
		t.Fatalf("conflicting correction retry = %d %s", conflictingRetryRecorder.Code, conflictingRetryRecorder.Body.String())
	}
	correctedOutcomesReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/outcomes", "", "managed-token", "managed-admin")
	correctedOutcomesRecorder := httptest.NewRecorder()
	router.ServeHTTP(correctedOutcomesRecorder, correctedOutcomesReq)
	if correctedOutcomesRecorder.Code != http.StatusOK {
		t.Fatalf("corrected outcomes status = %d, body = %s", correctedOutcomesRecorder.Code, correctedOutcomesRecorder.Body.String())
	}
	var correctedOutcomes struct {
		Outcomes []struct {
			Position     int    `json:"position"`
			ContestantID string `json:"contestant_id"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(correctedOutcomesRecorder.Body.Bytes(), &correctedOutcomes); err != nil {
		t.Fatalf("decode corrected outcomes: %v", err)
	}
	if len(correctedOutcomes.Outcomes) != 1 || correctedOutcomes.Outcomes[0].Position != 1 || correctedOutcomes.Outcomes[0].ContestantID != contestantIDs[2] {
		t.Fatalf("immediate correction did not replace published outcome: %+v", correctedOutcomes.Outcomes)
	}
	correctedLeaderboardReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/leaderboard", "", "managed-token", "managed-admin")
	correctedLeaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(correctedLeaderboardRecorder, correctedLeaderboardReq)
	if correctedLeaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("corrected leaderboard status = %d, body = %s", correctedLeaderboardRecorder.Code, correctedLeaderboardRecorder.Body.String())
	}
	var correctedLeaderboard leaderboardResponse
	if err := json.Unmarshal(correctedLeaderboardRecorder.Body.Bytes(), &correctedLeaderboard); err != nil {
		t.Fatalf("decode corrected leaderboard: %v", err)
	}
	correctedScores := make(map[string][3]int, len(correctedLeaderboard.Leaderboard))
	for _, row := range correctedLeaderboard.Leaderboard {
		correctedScores[row.ParticipantName] = [3]int{row.DraftPoints, row.BonusPoints, row.TotalPoints}
	}
	if correctedScores["Managed Alice"] != [3]int{1, 0, 1} || correctedScores["Managed Bob"] != [3]int{3, 0, 3} {
		t.Fatalf("unexpected immediately corrected scores: %+v", correctedScores)
	}
	var ledgerAfterOutcomeCorrection int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&ledgerAfterOutcomeCorrection); err != nil {
		t.Fatalf("count ledger after outcome correction: %v", err)
	}
	if ledgerAfterOutcomeCorrection != ledgerBeforeCorrection {
		t.Fatalf("outcome correction changed ledger row count from %d to %d", ledgerBeforeCorrection, ledgerAfterOutcomeCorrection)
	}
	bonusAfterOutcomeCorrection, err := queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[0].ID})
	if err != nil {
		t.Fatalf("read bonus after outcome correction: %v", err)
	}
	if bonusAfterOutcomeCorrection != bonusBeforeCorrection {
		t.Fatalf("outcome correction changed visible bonus from %d to %d", bonusBeforeCorrection, bonusAfterOutcomeCorrection)
	}
	secretAfterOutcomeCorrection, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[1].ID})
	if err != nil {
		t.Fatalf("read secret bonus after outcome correction: %v", err)
	}
	if secretAfterOutcomeCorrection != secretBeforeCorrection {
		t.Fatalf("outcome correction changed secret bonus from %d to %d", secretBeforeCorrection, secretAfterOutcomeCorrection)
	}
	ownershipAfterOutcomeCorrection, err := queries.ListActiveParticipantPonyOwnershipsByOwnerAt(ctx, db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{InstanceID: instanceID, OwnerParticipantID: participants[0].ID, At: timestamptz(verificationGameplayNow())})
	if err != nil {
		t.Fatalf("read ownership after outcome correction: %v", err)
	}
	if !reflect.DeepEqual(ownershipAfterOutcomeCorrection, ownershipBeforeCorrection) {
		t.Fatalf("outcome correction changed ownership state: before=%+v after=%+v", ownershipBeforeCorrection, ownershipAfterOutcomeCorrection)
	}
	var ledgerRowsAfterOutcomeCorrection, ownershipRowsAfterOutcomeCorrection []byte
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, participant_id, activity_occurrence_id, entry_kind, points, visibility, reason, effective_at, award_key, metadata, created_at FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ledgerRowsAfterOutcomeCorrection); err != nil {
		t.Fatalf("snapshot ledger after outcome correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, owner_participant_id, contestant_id, source_activity_occurrence_id, acquired_at, released_at, status, metadata, created_at, updated_at FROM participant_pony_ownerships WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ownershipRowsAfterOutcomeCorrection); err != nil {
		t.Fatalf("snapshot ownership after outcome correction: %v", err)
	}
	if !bytes.Equal(ledgerRowsBefore, ledgerRowsAfterOutcomeCorrection) || !bytes.Equal(ownershipRowsBefore, ownershipRowsAfterOutcomeCorrection) {
		t.Fatalf("outcome correction changed persisted ledger or ownership rows")
	}
	episode2AfterOutcomeCorrection, err := queries.GetInstanceEpisodeProgress(ctx, db.GetInstanceEpisodeProgressParams{InstanceID: instanceID, EpisodeNumber: 2})
	if err != nil {
		t.Fatalf("read episode 2 after outcome correction: %v", err)
	}
	if !reflect.DeepEqual(episode2Before, episode2AfterOutcomeCorrection) {
		t.Fatalf("outcome correction changed episode 2 state: before=%+v after=%+v", episode2Before, episode2AfterOutcomeCorrection)
	}
	closeRecorder := progress("/instances/"+instancePath+"/progression/draft/close", effective("draft-close", "2026-01-10T20:00:00Z"))
	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("close draft status = %d, body = %s", closeRecorder.Code, closeRecorder.Body.String())
	}
	lateBody := draftBody("draft-late", "2026-01-10T20:30:00Z", []string{contestantIDs[1], contestantIDs[0], contestantIDs[2]})
	lateReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/drafts/%s/late", instancePath, uuid.UUID(participants[0].ID.Bytes).String()), lateBody, "managed-token", "managed-admin")
	lateRecorder := httptest.NewRecorder()
	router.ServeHTTP(lateRecorder, lateReq)
	if lateRecorder.Code != http.StatusOK || !strings.Contains(lateRecorder.Body.String(), "revision_number") {
		t.Fatalf("late draft correction = %d %s", lateRecorder.Code, lateRecorder.Body.String())
	}
	outcomesReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/outcomes", "", "managed-token", "managed-admin")
	outcomesRecorder := httptest.NewRecorder()
	router.ServeHTTP(outcomesRecorder, outcomesReq)
	if outcomesRecorder.Code != http.StatusOK {
		t.Fatalf("published outcomes status = %d, body = %s", outcomesRecorder.Code, outcomesRecorder.Body.String())
	}
	var publishedOutcomes struct {
		Outcomes []struct {
			Position     int    `json:"position"`
			ContestantID string `json:"contestant_id"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(outcomesRecorder.Body.Bytes(), &publishedOutcomes); err != nil {
		t.Fatalf("decode published outcomes: %v", err)
	}
	if len(publishedOutcomes.Outcomes) != 1 || publishedOutcomes.Outcomes[0].Position != 1 || publishedOutcomes.Outcomes[0].ContestantID != contestantIDs[2] {
		t.Fatalf("pending outcome leaked or corrected outcome was lost: %s", outcomesRecorder.Body.String())
	}

	finalLeaderboardReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/leaderboard", "", "managed-token", "managed-admin")
	finalLeaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(finalLeaderboardRecorder, finalLeaderboardReq)
	if finalLeaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("final managed leaderboard status = %d, body = %s", finalLeaderboardRecorder.Code, finalLeaderboardRecorder.Body.String())
	}
	var finalLeaderboard leaderboardResponse
	if err := json.Unmarshal(finalLeaderboardRecorder.Body.Bytes(), &finalLeaderboard); err != nil {
		t.Fatalf("decode final managed leaderboard: %v", err)
	}
	finalScores := make(map[string][3]int, len(finalLeaderboard.Leaderboard))
	for _, row := range finalLeaderboard.Leaderboard {
		finalScores[row.ParticipantName] = [3]int{row.DraftPoints, row.BonusPoints, row.TotalPoints}
	}
	if finalScores["Managed Alice"] != [3]int{1, 0, 1} || finalScores["Managed Bob"] != [3]int{3, 0, 3} {
		t.Fatalf("unexpected corrected published scores: %+v", finalScores)
	}
	ledgerAfterCorrection := 0
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&ledgerAfterCorrection); err != nil {
		t.Fatalf("count ledger after late correction: %v", err)
	}
	if ledgerAfterCorrection != ledgerBeforeCorrection {
		t.Fatalf("late correction changed ledger row count from %d to %d", ledgerBeforeCorrection, ledgerAfterCorrection)
	}
	bonusAfterCorrection, err := queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[0].ID})
	if err != nil {
		t.Fatalf("read bonus after late correction: %v", err)
	}
	if bonusAfterCorrection != bonusBeforeCorrection {
		t.Fatalf("late correction changed visible bonus from %d to %d", bonusBeforeCorrection, bonusAfterCorrection)
	}
	secretAfterCorrection, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instanceID, ParticipantID: participants[1].ID})
	if err != nil {
		t.Fatalf("read secret bonus after late correction: %v", err)
	}
	if secretAfterCorrection != secretBeforeCorrection {
		t.Fatalf("late correction changed secret bonus from %d to %d", secretBeforeCorrection, secretAfterCorrection)
	}
	ownershipAfterCorrection, err := queries.ListActiveParticipantPonyOwnershipsByOwnerAt(ctx, db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{InstanceID: instanceID, OwnerParticipantID: participants[0].ID, At: timestamptz(verificationGameplayNow())})
	if err != nil {
		t.Fatalf("read ownership after late correction: %v", err)
	}
	if len(ownershipAfterCorrection) != len(ownershipBeforeCorrection) || (len(ownershipBeforeCorrection) > 0 && ownershipAfterCorrection[0].ID != ownershipBeforeCorrection[0].ID) {
		t.Fatalf("late correction changed ownership state: before=%+v after=%+v", ownershipBeforeCorrection, ownershipAfterCorrection)
	}
	var ledgerRowsAfter, ownershipRowsAfter []byte
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, participant_id, activity_occurrence_id, entry_kind, points, visibility, reason, effective_at, award_key, metadata, created_at FROM bonus_point_ledger_entries WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ledgerRowsAfter); err != nil {
		t.Fatalf("snapshot ledger after late correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.id), '[]'::json) FROM (SELECT id, public_id, owner_participant_id, contestant_id, source_activity_occurrence_id, acquired_at, released_at, status, metadata, created_at, updated_at FROM participant_pony_ownerships WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)) rows`, instanceID).Scan(&ownershipRowsAfter); err != nil {
		t.Fatalf("snapshot ownership after late correction: %v", err)
	}
	if !bytes.Equal(ledgerRowsBefore, ledgerRowsAfter) || !bytes.Equal(ownershipRowsBefore, ownershipRowsAfter) {
		t.Fatalf("late correction changed persisted ledger or ownership rows")
	}
	episode2After, err := queries.GetInstanceEpisodeProgress(ctx, db.GetInstanceEpisodeProgressParams{InstanceID: instanceID, EpisodeNumber: 2})
	if err != nil {
		t.Fatalf("read episode 2 after late correction: %v", err)
	}
	if !reflect.DeepEqual(episode2Before, episode2After) {
		t.Fatalf("late correction changed episode 2 state: before=%+v after=%+v", episode2Before, episode2After)
	}

	type snapshot struct {
		Outcomes map[string]int `json:"outcomes"`
		Drafts   map[string][]struct {
			Position     int    `json:"position"`
			ContestantID string `json:"contestant_id"`
		} `json:"drafts"`
		VisibleBonus map[string]int `json:"visible_bonus"`
	}
	var revisionRows []struct {
		number  int
		rawJSON []byte
	}
	rows, err := pool.Query(ctx, `
		SELECT isr.revision_number, isr.input_snapshot
		FROM instance_score_revisions isr
		JOIN instances i ON i.id = isr.instance_id
		WHERE i.public_id = $1
		ORDER BY isr.revision_number`, instanceID)
	if err != nil {
		t.Fatalf("list score revisions: %v", err)
	}
	for rows.Next() {
		var row struct {
			number  int
			rawJSON []byte
		}
		if err := rows.Scan(&row.number, &row.rawJSON); err != nil {
			rows.Close()
			t.Fatalf("scan score revision: %v", err)
		}
		revisionRows = append(revisionRows, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatalf("read score revisions: %v", err)
	}
	rows.Close()
	if len(revisionRows) != 4 || revisionRows[0].number != 1 || revisionRows[1].number != 2 || revisionRows[2].number != 3 || revisionRows[3].number != 4 {
		t.Fatalf("expected four ordered immutable score revisions, got %+v", revisionRows)
	}
	if !bytes.Equal(initialSnapshotJSON, revisionRows[0].rawJSON) {
		t.Fatalf("first score snapshot changed after corrections")
	}
	var firstSnapshot, latestSnapshot snapshot
	if err := json.Unmarshal(revisionRows[0].rawJSON, &firstSnapshot); err != nil {
		t.Fatalf("decode first score snapshot: %v", err)
	}
	if err := json.Unmarshal(revisionRows[3].rawJSON, &latestSnapshot); err != nil {
		t.Fatalf("decode latest score snapshot: %v", err)
	}
	if firstSnapshot.Outcomes[contestantIDs[0]] != 1 || len(firstSnapshot.Outcomes) != 1 {
		t.Fatalf("first snapshot changed or was incomplete: %+v", firstSnapshot.Outcomes)
	}
	if latestSnapshot.Outcomes[contestantIDs[2]] != 1 || len(latestSnapshot.Outcomes) != 1 {
		t.Fatalf("latest snapshot leaked pending outcomes or lost correction: %+v", latestSnapshot.Outcomes)
	}
	if latestSnapshot.VisibleBonus[uuid.UUID(participants[0].ID.Bytes).String()] != 0 {
		t.Fatalf("latest snapshot included pending bonus: %+v", latestSnapshot.VisibleBonus)
	}
	if len(latestSnapshot.Drafts[uuid.UUID(participants[0].ID.Bytes).String()]) != 3 || latestSnapshot.Drafts[uuid.UUID(participants[0].ID.Bytes).String()][0].ContestantID != contestantIDs[1] {
		t.Fatalf("latest snapshot did not preserve late draft correction: %+v", latestSnapshot.Drafts)
	}
	episode2Complete := progress("/instances/"+instancePath+"/progression/episodes/2/complete", effective("episode-2-complete", "2026-01-11T20:00:00Z"))
	if episode2Complete.Code != http.StatusOK {
		t.Fatalf("complete episode 2 status = %d, body = %s", episode2Complete.Code, episode2Complete.Body.String())
	}
	episode2Score := progress("/instances/"+instancePath+"/progression/episodes/2/score", effective("episode-2-score", "2026-01-12T20:00:00Z"))
	if episode2Score.Code != http.StatusOK {
		t.Fatalf("score episode 2 status = %d, body = %s", episode2Score.Code, episode2Score.Body.String())
	}
	afterEpisode2OutcomesReq := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/outcomes", "", "managed-token", "managed-admin")
	afterEpisode2OutcomesRecorder := httptest.NewRecorder()
	router.ServeHTTP(afterEpisode2OutcomesRecorder, afterEpisode2OutcomesReq)
	if afterEpisode2OutcomesRecorder.Code != http.StatusOK {
		t.Fatalf("post-score outcomes status = %d, body = %s", afterEpisode2OutcomesRecorder.Code, afterEpisode2OutcomesRecorder.Body.String())
	}
	var afterEpisode2Outcomes struct {
		Outcomes []struct {
			Position     int    `json:"position"`
			ContestantID string `json:"contestant_id"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(afterEpisode2OutcomesRecorder.Body.Bytes(), &afterEpisode2Outcomes); err != nil {
		t.Fatalf("decode post-score outcomes: %v", err)
	}
	if len(afterEpisode2Outcomes.Outcomes) != 2 || afterEpisode2Outcomes.Outcomes[0].Position != 1 || afterEpisode2Outcomes.Outcomes[0].ContestantID != contestantIDs[2] || afterEpisode2Outcomes.Outcomes[1].Position != 2 || afterEpisode2Outcomes.Outcomes[1].ContestantID != contestantIDs[1] {
		t.Fatalf("correction did not survive next episode publication: %+v", afterEpisode2Outcomes.Outcomes)
	}
	earlyServer := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"managed-token"}}), httpapi.WithClock(func() time.Time { return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC) }))
	lateServer := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"managed-token"}}), httpapi.WithClock(func() time.Time { return time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC) }))
	var earlyLeaderboard, lateLeaderboard map[string]any
	for _, server := range []*httpapi.Server{earlyServer, lateServer} {
		req := authorizedJSONRequest(http.MethodGet, "/instances/"+instancePath+"/leaderboard", "", "managed-token", "managed-admin")
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("clock-independent leaderboard status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		if server == earlyServer {
			if err := json.Unmarshal(recorder.Body.Bytes(), &earlyLeaderboard); err != nil {
				t.Fatalf("decode early leaderboard: %v", err)
			}
		} else if err := json.Unmarshal(recorder.Body.Bytes(), &lateLeaderboard); err != nil {
			t.Fatalf("decode late leaderboard: %v", err)
		}
	}
	if !reflect.DeepEqual(earlyLeaderboard, lateLeaderboard) {
		t.Fatalf("managed leaderboard changed with wall clock: early=%v late=%v", earlyLeaderboard, lateLeaderboard)
	}
	liveBeforeRollback, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes before rollback correction: %v", err)
	}
	var latestSnapshotBeforeRollback []byte
	if err := pool.QueryRow(ctx, `SELECT input_snapshot FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1) AND revision_number = 5`, instanceID).Scan(&latestSnapshotBeforeRollback); err != nil {
		t.Fatalf("read latest snapshot before rollback correction: %v", err)
	}
	invalidSnapshot := fmt.Sprintf(`{"total_positions":3,"outcomes":{"%s":1},"outcome_names":{"%s":"Managed C3"},"drafts":{},"visible_bonus":{},"participant_names":{"00000000-0000-0000-0000-000000000000":"Unknown"},"tribe_names":{}}`, contestantIDs[2], contestantIDs[2])
	if _, err := pool.Exec(ctx, `UPDATE instance_score_revisions SET input_snapshot = $1 WHERE instance_id = (SELECT id FROM instances WHERE public_id = $2) AND revision_number = 5`, []byte(invalidSnapshot), instanceID); err != nil {
		t.Fatalf("corrupt snapshot for rollback correction: %v", err)
	}
	rollbackCorrectionReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/outcomes/1", instancePath), `{"correction":true,"idempotency_key":"outcome-correction-rollback","effective_at":"2026-01-13T20:00:00Z"}`, "managed-token", "managed-admin")
	rollbackCorrectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(rollbackCorrectionRecorder, rollbackCorrectionReq)
	if _, err := pool.Exec(ctx, `UPDATE instance_score_revisions SET input_snapshot = $1 WHERE instance_id = (SELECT id FROM instances WHERE public_id = $2) AND revision_number = 5`, latestSnapshotBeforeRollback, instanceID); err != nil {
		t.Fatalf("restore snapshot after rollback correction: %v", err)
	}
	if rollbackCorrectionRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("rollback correction status = %d, body = %s", rollbackCorrectionRecorder.Code, rollbackCorrectionRecorder.Body.String())
	}
	liveAfterRollback, err := queries.ListOutcomePositionsByInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("read outcomes after rollback correction: %v", err)
	}
	if !reflect.DeepEqual(liveAfterRollback, liveBeforeRollback) {
		t.Fatalf("failed outcome correction changed live outcomes: before=%+v after=%+v", liveBeforeRollback, liveAfterRollback)
	}
	var revisionCount, commandCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&revisionCount); err != nil {
		t.Fatalf("count revisions after rollback correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&commandCount); err != nil {
		t.Fatalf("count commands after rollback correction: %v", err)
	}
	if revisionCount != 5 || commandCount != 15 {
		t.Fatalf("failed outcome correction persisted state: revisions=%d commands=%d", revisionCount, commandCount)
	}
}

func TestActivitiesOccurrencesHandlersAndResolve(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Activities API", 50)
	participant := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	group := createParticipantGroupForTest(t, ctx, queries, instance.ID, "Team Orange", "tribe")

	server := httpapi.New(pool)
	router := server.Router()

	createActivityReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/instances/%s/activities", uuid.UUID(instance.ID.Bytes).String()), strings.NewReader(`{"activity_type":"manual_adjustment","name":"Manual Adjustments","status":"active","starts_at":"2026-03-21T12:00:00Z","metadata":{"scope":"test"}}`))
	createActivityReq.Header.Set("Content-Type", "application/json")
	createActivityRecorder := httptest.NewRecorder()
	router.ServeHTTP(createActivityRecorder, createActivityReq)
	if createActivityRecorder.Code != http.StatusCreated {
		t.Fatalf("create activity status = %d, body = %s", createActivityRecorder.Code, createActivityRecorder.Body.String())
	}
	var createdActivity struct {
		Activity struct {
			ID string `json:"id"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(createActivityRecorder.Body.Bytes(), &createdActivity); err != nil {
		t.Fatalf("unmarshal create activity: %v", err)
	}

	listActivitiesReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/activities", uuid.UUID(instance.ID.Bytes).String()), nil)
	listActivitiesRecorder := httptest.NewRecorder()
	router.ServeHTTP(listActivitiesRecorder, listActivitiesReq)
	if listActivitiesRecorder.Code != http.StatusOK {
		t.Fatalf("list activities status = %d, body = %s", listActivitiesRecorder.Code, listActivitiesRecorder.Body.String())
	}
	var activities activitiesResponse
	if err := json.Unmarshal(listActivitiesRecorder.Body.Bytes(), &activities); err != nil {
		t.Fatalf("unmarshal activities response: %v", err)
	}
	if len(activities.Activities) != 1 || activities.Activities[0].Name != "Manual Adjustments" {
		t.Fatalf("unexpected activities response: %+v", activities)
	}

	createOccurrenceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/activities/%s/occurrences", createdActivity.Activity.ID), strings.NewReader(`{"occurrence_type":"manual_correction","name":"Episode 1 Correction","effective_at":"2026-03-22T09:00:00Z","status":"recorded","metadata":{"note":"adjustment"}}`))
	createOccurrenceReq.Header.Set("Content-Type", "application/json")
	createOccurrenceRecorder := httptest.NewRecorder()
	router.ServeHTTP(createOccurrenceRecorder, createOccurrenceReq)
	if createOccurrenceRecorder.Code != http.StatusCreated {
		t.Fatalf("create occurrence status = %d, body = %s", createOccurrenceRecorder.Code, createOccurrenceRecorder.Body.String())
	}
	var createdOccurrence struct {
		Occurrence struct {
			ID string `json:"id"`
		} `json:"occurrence"`
	}
	if err := json.Unmarshal(createOccurrenceRecorder.Body.Bytes(), &createdOccurrence); err != nil {
		t.Fatalf("unmarshal create occurrence: %v", err)
	}

	listOccurrencesReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/activities/%s/occurrences", createdActivity.Activity.ID), nil)
	listOccurrencesRecorder := httptest.NewRecorder()
	router.ServeHTTP(listOccurrencesRecorder, listOccurrencesReq)
	if listOccurrencesRecorder.Code != http.StatusOK {
		t.Fatalf("list occurrences status = %d, body = %s", listOccurrencesRecorder.Code, listOccurrencesRecorder.Body.String())
	}
	var occurrences occurrencesResponse
	if err := json.Unmarshal(listOccurrencesRecorder.Body.Bytes(), &occurrences); err != nil {
		t.Fatalf("unmarshal occurrences response: %v", err)
	}
	if len(occurrences.Occurrences) != 1 || occurrences.Occurrences[0].Name != "Episode 1 Correction" {
		t.Fatalf("unexpected occurrences response: %+v", occurrences)
	}

	participantBody := fmt.Sprintf(`{"participant_id":%q,"role":"target","metadata":{"points":3,"visibility":"public","reason":"manual correction","entry_kind":"correction","award_key":"manual-adjustment"}}`, uuid.UUID(participant.ID.Bytes).String())
	participantReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/occurrences/%s/participants", createdOccurrence.Occurrence.ID), strings.NewReader(participantBody))
	participantReq.Header.Set("Content-Type", "application/json")
	participantRecorder := httptest.NewRecorder()
	router.ServeHTTP(participantRecorder, participantReq)
	if participantRecorder.Code != http.StatusCreated {
		t.Fatalf("create occurrence participant status = %d, body = %s", participantRecorder.Code, participantRecorder.Body.String())
	}

	groupBody := fmt.Sprintf(`{"participant_group_id":%q,"role":"tribe","result":"winner","metadata":{"label":"winner"}}`, uuid.UUID(group.ID.Bytes).String())
	groupReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/occurrences/%s/groups", createdOccurrence.Occurrence.ID), strings.NewReader(groupBody))
	groupReq.Header.Set("Content-Type", "application/json")
	groupRecorder := httptest.NewRecorder()
	router.ServeHTTP(groupRecorder, groupReq)
	if groupRecorder.Code != http.StatusCreated {
		t.Fatalf("create occurrence group status = %d, body = %s", groupRecorder.Code, groupRecorder.Body.String())
	}

	resolveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/occurrences/%s/resolve", createdOccurrence.Occurrence.ID), nil)
	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, resolveReq)
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve occurrence status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}
	var resolveResponse struct {
		CreatedCount   int `json:"created_count"`
		CreatedEntries []struct {
			ParticipantID string `json:"participant_id"`
			EntryKind     string `json:"entry_kind"`
			Points        int    `json:"points"`
		} `json:"created_entries"`
	}
	if err := json.Unmarshal(resolveRecorder.Body.Bytes(), &resolveResponse); err != nil {
		t.Fatalf("unmarshal resolve response: %v", err)
	}
	if resolveResponse.CreatedCount != 1 || len(resolveResponse.CreatedEntries) != 1 {
		t.Fatalf("unexpected resolve response: %+v", resolveResponse)
	}
	if resolveResponse.CreatedEntries[0].ParticipantID != uuid.UUID(participant.ID.Bytes).String() || resolveResponse.CreatedEntries[0].EntryKind != "correction" || resolveResponse.CreatedEntries[0].Points != 3 {
		t.Fatalf("unexpected resolved entry: %+v", resolveResponse.CreatedEntries[0])
	}

	ledgerReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(participant.ID.Bytes).String()), nil)
	ledgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(ledgerRecorder, ledgerReq)
	if ledgerRecorder.Code != http.StatusOK {
		t.Fatalf("ledger status = %d, body = %s", ledgerRecorder.Code, ledgerRecorder.Body.String())
	}
	var ledger bonusLedgerResponse
	if err := json.Unmarshal(ledgerRecorder.Body.Bytes(), &ledger); err != nil {
		t.Fatalf("unmarshal bonus ledger response: %v", err)
	}
	if ledger.BonusPoints != 3 || len(ledger.Ledger) != 1 {
		t.Fatalf("unexpected ledger response: %+v", ledger)
	}
}

func TestActivityOccurrenceDetailAndParticipantHistoryReadApis(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Detail APIs", 50)
	participant := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	otherParticipant := createParticipantForTest(t, ctx, queries, instance.ID, "Bob")
	group := createParticipantGroupForTest(t, ctx, queries, instance.ID, "Team Orange", "tribe")
	activity := createActivityForTest(t, ctx, queries, instance.ID, time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC), nil, "journey", "Journey 2")
	occurrence := createOccurrenceForTest(t, ctx, queries, activity.ID, "journey_resolution", "Journey 2 Resolution", time.Date(2026, time.March, 21, 13, 0, 0, 0, time.UTC))
	groupAwardOccurrence := createOccurrenceForTest(t, ctx, queries, activity.ID, "journey_attendance", "Journey 2 Attendance", time.Date(2026, time.March, 21, 12, 30, 0, 0, time.UTC))

	if _, err := queries.CreateActivityGroupAssignment(ctx, db.CreateActivityGroupAssignmentParams{
		ActivityID:         activity.ID,
		ParticipantGroupID: group.ID,
		Role:               "tribe",
		StartsAt:           timestamptz(time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)),
		EndsAt:             pgtype.Timestamptz{},
		Configuration:      []byte(`{"pony_survivor_tribe":"orange"}`),
	}); err != nil {
		t.Fatalf("create activity group assignment: %v", err)
	}
	if _, err := queries.CreateActivityParticipantAssignment(ctx, db.CreateActivityParticipantAssignmentParams{
		ActivityID:         activity.ID,
		ParticipantID:      participant.ID,
		ParticipantGroupID: group.ID,
		Role:               "delegate",
		StartsAt:           timestamptz(time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC)),
		EndsAt:             pgtype.Timestamptz{},
		Configuration:      testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create activity participant assignment: %v", err)
	}
	if _, err := queries.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantID:        participant.ID,
		ParticipantGroupID:   group.ID,
		Role:                 "delegate",
		Result:               "SHARE",
		Metadata:             []byte(`{"choice":"share"}`),
	}); err != nil {
		t.Fatalf("create occurrence participant: %v", err)
	}
	if _, err := queries.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantID:        otherParticipant.ID,
		ParticipantGroupID:   group.ID,
		Role:                 "delegate",
		Result:               "STEAL",
		Metadata:             []byte(`{"choice":"steal"}`),
	}); err != nil {
		t.Fatalf("create second occurrence participant: %v", err)
	}
	if _, err := queries.CreateActivityOccurrenceGroup(ctx, db.CreateActivityOccurrenceGroupParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantGroupID:   group.ID,
		Role:                 "tribe",
		Result:               "winner",
		Metadata:             []byte(`{"label":"winner"}`),
	}); err != nil {
		t.Fatalf("create occurrence group: %v", err)
	}
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participant.ID, occurrence.ID, group.ID, "award", 2, "public", "shared reward", "alice-share")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participant.ID, occurrence.ID, group.ID, "award", 4, "secret", "hidden reward", "alice-secret")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participant.ID, groupAwardOccurrence.ID, group.ID, "award", 1, "public", "attendance reward", "alice-attendance")

	server := httpapi.New(pool)
	router := server.Router()

	activityReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/activities/%s", uuid.UUID(activity.ID.Bytes).String()), nil)
	activityRecorder := httptest.NewRecorder()
	router.ServeHTTP(activityRecorder, activityReq)
	if activityRecorder.Code != http.StatusOK {
		t.Fatalf("activity detail status = %d, body = %s", activityRecorder.Code, activityRecorder.Body.String())
	}
	var activityDetail activityDetailResponse
	if err := json.Unmarshal(activityRecorder.Body.Bytes(), &activityDetail); err != nil {
		t.Fatalf("unmarshal activity detail: %v", err)
	}
	if activityDetail.Activity.ID != uuid.UUID(activity.ID.Bytes).String() || len(activityDetail.GroupAssignments) != 1 || len(activityDetail.ParticipantAssignments) != 1 {
		t.Fatalf("unexpected activity detail response: %+v", activityDetail)
	}

	occurrenceReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/occurrences/%s", uuid.UUID(occurrence.ID.Bytes).String()), nil)
	occurrenceRecorder := httptest.NewRecorder()
	router.ServeHTTP(occurrenceRecorder, occurrenceReq)
	if occurrenceRecorder.Code != http.StatusOK {
		t.Fatalf("occurrence detail status = %d, body = %s", occurrenceRecorder.Code, occurrenceRecorder.Body.String())
	}
	var occurrenceDetail occurrenceDetailResponse
	if err := json.Unmarshal(occurrenceRecorder.Body.Bytes(), &occurrenceDetail); err != nil {
		t.Fatalf("unmarshal occurrence detail: %v", err)
	}
	if occurrenceDetail.Occurrence.ID != uuid.UUID(occurrence.ID.Bytes).String() || len(occurrenceDetail.Participants) != 2 || len(occurrenceDetail.Groups) != 1 {
		t.Fatalf("unexpected occurrence detail response: %+v", occurrenceDetail)
	}
	if len(occurrenceDetail.Ledger) != 1 || occurrenceDetail.Ledger[0].Visibility != "public" || occurrenceDetail.Ledger[0].Points != 2 {
		t.Fatalf("unexpected occurrence ledger visibility/points: %+v", occurrenceDetail.Ledger)
	}

	historyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/activity-history", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(participant.ID.Bytes).String()), nil)
	historyRecorder := httptest.NewRecorder()
	router.ServeHTTP(historyRecorder, historyReq)
	if historyRecorder.Code != http.StatusOK {
		t.Fatalf("participant activity history status = %d, body = %s", historyRecorder.Code, historyRecorder.Body.String())
	}
	var history participantActivityHistoryResponse
	if err := json.Unmarshal(historyRecorder.Body.Bytes(), &history); err != nil {
		t.Fatalf("unmarshal participant activity history: %v", err)
	}
	if history.Participant.ID != uuid.UUID(participant.ID.Bytes).String() || history.Instance.ID != uuid.UUID(instance.ID.Bytes).String() || len(history.Activities) != 1 {
		t.Fatalf("unexpected participant activity history header: %+v", history)
	}
	if len(history.Activities[0].Occurrences) != 2 {
		t.Fatalf("expected 2 participant history occurrences, got %+v", history.Activities[0].Occurrences)
	}
	foundDirect := false
	foundLedgerOnly := false
	for _, item := range history.Activities[0].Occurrences {
		switch item.Occurrence.ID {
		case uuid.UUID(occurrence.ID.Bytes).String():
			foundDirect = item.Involvement != nil && item.Involvement.ParticipantID == uuid.UUID(participant.ID.Bytes).String() && len(item.Ledger) == 1 && item.Ledger[0].Points == 2
		case uuid.UUID(groupAwardOccurrence.ID.Bytes).String():
			foundLedgerOnly = item.Involvement == nil && len(item.Ledger) == 1 && item.Ledger[0].Points == 1
		}
	}
	if !foundDirect || !foundLedgerOnly {
		t.Fatalf("unexpected participant history occurrence breakdown: %+v", history.Activities[0].Occurrences)
	}
}

func TestParticipantDiscordLinkAndPrivateViews(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Discord Auth", 50)
	alice := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	activity := createActivityForTest(t, ctx, queries, instance.ID, time.Date(2026, time.March, 22, 12, 0, 0, 0, time.UTC), nil, "journey", "Journey 3")
	occurrence := createOccurrenceForTest(t, ctx, queries, activity.ID, "journey_resolution", "Journey 3 Resolution", time.Date(2026, time.March, 22, 13, 0, 0, 0, time.UTC))

	if _, err := queries.CreateActivityOccurrenceParticipant(ctx, db.CreateActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: occurrence.ID,
		ParticipantID:        alice.ID,
		Role:                 "delegate",
		Result:               "SHARE",
		Metadata:             testEmptyJSONB,
	}); err != nil {
		t.Fatalf("create occurrence participant: %v", err)
	}
	createLedgerEntryForTest(t, ctx, queries, instance.ID, alice.ID, occurrence.ID, pgtype.UUID{}, "award", 2, "public", "public award", "alice-public")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, alice.ID, occurrence.ID, pgtype.UUID{}, "award", 5, "secret", "secret award", "alice-secret")
	for _, discordUserID := range []string{"user-1", "admin-1"} {
		if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{
			InstanceID:    instance.ID,
			DiscordUserID: discordUserID,
		}); err != nil {
			t.Fatalf("create instance admin %s: %v", discordUserID, err)
		}
	}

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"link-test"}}))
	engine := server.Router()
	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer link-test")
		engine.ServeHTTP(w, r)
	})

	linkReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/instances/%s/participants/%s/discord-link", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String()), nil)
	linkReq.Header.Set("X-Discord-User-ID", "user-1")
	linkRecorder := httptest.NewRecorder()
	router.ServeHTTP(linkRecorder, linkReq)
	if linkRecorder.Code != http.StatusOK {
		t.Fatalf("link status = %d, body = %s", linkRecorder.Code, linkRecorder.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/me", uuid.UUID(instance.ID.Bytes).String()), nil)
	meReq.Header.Set("X-Discord-User-ID", "user-1")
	meRecorder := httptest.NewRecorder()
	router.ServeHTTP(meRecorder, meReq)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("linked participant status = %d, body = %s", meRecorder.Code, meRecorder.Body.String())
	}

	ledgerPath := fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String())
	publicLedgerReq := httptest.NewRequest(http.MethodGet, ledgerPath, nil)
	publicLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(publicLedgerRecorder, publicLedgerReq)
	if publicLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("public bonus ledger status = %d, body = %s", publicLedgerRecorder.Code, publicLedgerRecorder.Body.String())
	}
	var publicLedger bonusLedgerResponse
	if err := json.Unmarshal(publicLedgerRecorder.Body.Bytes(), &publicLedger); err != nil {
		t.Fatalf("unmarshal public ledger: %v", err)
	}
	if publicLedger.BonusPoints != 2 || len(publicLedger.Ledger) != 1 || publicLedger.Ledger[0].Visibility != "public" {
		t.Fatalf("unexpected public ledger: %+v", publicLedger)
	}

	privateLedgerReq := httptest.NewRequest(http.MethodGet, ledgerPath, nil)
	privateLedgerReq.Header.Set("X-Discord-User-ID", "user-1")
	privateLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(privateLedgerRecorder, privateLedgerReq)
	if privateLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("private bonus ledger status = %d, body = %s", privateLedgerRecorder.Code, privateLedgerRecorder.Body.String())
	}
	var privateLedger bonusLedgerResponse
	if err := json.Unmarshal(privateLedgerRecorder.Body.Bytes(), &privateLedger); err != nil {
		t.Fatalf("unmarshal private ledger: %v", err)
	}
	if privateLedger.BonusPoints != 7 || len(privateLedger.Ledger) != 2 {
		t.Fatalf("unexpected private ledger: %+v", privateLedger)
	}

	historyPath := fmt.Sprintf("/instances/%s/participants/%s/activity-history", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String())
	publicHistoryReq := httptest.NewRequest(http.MethodGet, historyPath, nil)
	publicHistoryRecorder := httptest.NewRecorder()
	router.ServeHTTP(publicHistoryRecorder, publicHistoryReq)
	if publicHistoryRecorder.Code != http.StatusOK {
		t.Fatalf("public history status = %d, body = %s", publicHistoryRecorder.Code, publicHistoryRecorder.Body.String())
	}
	var publicHistory participantActivityHistoryResponse
	if err := json.Unmarshal(publicHistoryRecorder.Body.Bytes(), &publicHistory); err != nil {
		t.Fatalf("unmarshal public history: %v", err)
	}
	if len(publicHistory.Activities) != 1 || len(publicHistory.Activities[0].Occurrences) != 1 || len(publicHistory.Activities[0].Occurrences[0].Ledger) != 1 || publicHistory.Activities[0].Occurrences[0].Ledger[0].Visibility != "public" {
		t.Fatalf("unexpected public history: %+v", publicHistory)
	}

	privateHistoryReq := httptest.NewRequest(http.MethodGet, historyPath, nil)
	privateHistoryReq.Header.Set("X-Discord-User-ID", "user-1")
	privateHistoryRecorder := httptest.NewRecorder()
	router.ServeHTTP(privateHistoryRecorder, privateHistoryReq)
	if privateHistoryRecorder.Code != http.StatusOK {
		t.Fatalf("private history status = %d, body = %s", privateHistoryRecorder.Code, privateHistoryRecorder.Body.String())
	}
	var privateHistory participantActivityHistoryResponse
	if err := json.Unmarshal(privateHistoryRecorder.Body.Bytes(), &privateHistory); err != nil {
		t.Fatalf("unmarshal private history: %v", err)
	}
	if len(privateHistory.Activities[0].Occurrences[0].Ledger) != 2 {
		t.Fatalf("expected private history to include secret ledger entries, got %+v", privateHistory.Activities[0].Occurrences[0].Ledger)
	}

	adminLedgerReq := httptest.NewRequest(http.MethodGet, ledgerPath, nil)
	adminLedgerReq.Header.Set("X-Discord-User-ID", "admin-1")
	adminLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(adminLedgerRecorder, adminLedgerReq)
	if adminLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("admin bonus ledger status = %d, body = %s", adminLedgerRecorder.Code, adminLedgerRecorder.Body.String())
	}
	var adminLedger bonusLedgerResponse
	if err := json.Unmarshal(adminLedgerRecorder.Body.Bytes(), &adminLedger); err != nil {
		t.Fatalf("unmarshal admin ledger: %v", err)
	}
	if adminLedger.BonusPoints != 7 || len(adminLedger.Ledger) != 2 {
		t.Fatalf("unexpected admin ledger: %+v", adminLedger)
	}

	forbiddenUnlinkReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/instances/%s/participants/%s/discord-link", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String()), nil)
	forbiddenUnlinkReq.Header.Set("X-Discord-User-ID", "user-2")
	forbiddenUnlinkRecorder := httptest.NewRecorder()
	router.ServeHTTP(forbiddenUnlinkRecorder, forbiddenUnlinkReq)
	if forbiddenUnlinkRecorder.Code != http.StatusForbidden {
		t.Fatalf("forbidden unlink status = %d, body = %s", forbiddenUnlinkRecorder.Code, forbiddenUnlinkRecorder.Body.String())
	}

	unlinkReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/instances/%s/participants/%s/discord-link", uuid.UUID(instance.ID.Bytes).String(), uuid.UUID(alice.ID.Bytes).String()), nil)
	unlinkReq.Header.Set("X-Discord-User-ID", "user-1")
	unlinkRecorder := httptest.NewRecorder()
	router.ServeHTTP(unlinkRecorder, unlinkReq)
	if unlinkRecorder.Code != http.StatusOK {
		t.Fatalf("unlink status = %d, body = %s", unlinkRecorder.Code, unlinkRecorder.Body.String())
	}

	missingMeReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/me", uuid.UUID(instance.ID.Bytes).String()), nil)
	missingMeReq.Header.Set("X-Discord-User-ID", "user-1")
	missingMeRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingMeRecorder, missingMeReq)
	if missingMeRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing linked participant after unlink, got %d body=%s", missingMeRecorder.Code, missingMeRecorder.Body.String())
	}
}

func TestActivitiesOccurrencesHandlers_BadInputAndNotFound(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Activities API Errors", 50)
	server := httpapi.New(pool)
	router := server.Router()

	badActivityReq := httptest.NewRequest(http.MethodPost, "/instances/not-a-uuid/activities", nil)
	badActivityRecorder := httptest.NewRecorder()
	router.ServeHTTP(badActivityRecorder, badActivityReq)
	if badActivityRecorder.Code != http.StatusBadRequest {
		t.Fatalf("bad instance id status = %d, body = %s", badActivityRecorder.Code, badActivityRecorder.Body.String())
	}

	badBodyReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/instances/%s/activities", uuid.UUID(instance.ID.Bytes).String()), strings.NewReader(`{"name":"missing required fields"}`))
	badBodyReq.Header.Set("Content-Type", "application/json")
	badBodyRecorder := httptest.NewRecorder()
	router.ServeHTTP(badBodyRecorder, badBodyReq)
	if badBodyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("bad body status = %d, body = %s", badBodyRecorder.Code, badBodyRecorder.Body.String())
	}

	notFoundResolveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/occurrences/%s/resolve", uuid.NewString()), nil)
	notFoundResolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(notFoundResolveRecorder, notFoundResolveReq)
	if notFoundResolveRecorder.Code != http.StatusNotFound {
		t.Fatalf("resolve missing occurrence status = %d, body = %s", notFoundResolveRecorder.Code, notFoundResolveRecorder.Body.String())
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

func TestMergeGameplayVerificationFlow(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	seasons, err := seeddata.LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if _, err := appinternal.SeedHistorical(ctx, pool, seasons); err != nil {
		t.Fatalf("seed verification season: %v", err)
	}

	queries := db.New(pool)
	instances, err := queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	var instance db.ListInstancesRow
	foundInstance := false
	for _, candidate := range instances {
		if candidate.Name == "Verification Merge Gameplay" {
			instance = candidate
			foundInstance = true
			break
		}
	}
	if !foundInstance {
		t.Fatalf("verification instance not found")
	}

	participants, err := queries.ListParticipantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	participantIDByName := make(map[string]pgtype.UUID, len(participants))
	for _, participant := range participants {
		participantIDByName[participant.Name] = participant.ID
	}
	contestants, err := queries.ListContestantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list contestants: %v", err)
	}
	contestantIDByName := make(map[string]uuid.UUID, len(contestants))
	for _, contestant := range contestants {
		contestantIDByName[contestant.Name] = uuid.UUID(contestant.ID.Bytes)
	}

	for name, discordUserID := range map[string]string{"Alice": "alice-discord", "Bob": "bob-discord", "Cara": "cara-discord"} {
		if _, err := queries.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: participantIDByName[name], DiscordUserID: pgtype.Text{String: discordUserID, Valid: true}}); err != nil {
			t.Fatalf("link participant %s: %v", name, err)
		}
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}

	server := httpapi.New(pool,
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}),
		httpapi.WithClock(verificationGameplayNow),
	)
	router := server.Router()
	instanceUUID := uuid.UUID(instance.ID.Bytes).String()
	aliceID := uuid.UUID(participantIDByName["Alice"].Bytes).String()
	bobID := uuid.UUID(participantIDByName["Bob"].Bytes).String()
	joeID := contestantIDByName["Joe"].String()

	hiddenActivity := createActivityForTest(t, ctx, queries, instance.ID, verificationGameplayNow().Add(-2*time.Hour), nil, "manual_adjustment", "Verification Hidden Bonus")
	hiddenOccurrence := createOccurrenceForTest(t, ctx, queries, hiddenActivity.ID, "manual_correction", "Verification Hidden Bonus Grant", verificationGameplayNow().Add(-2*time.Hour))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Alice"], hiddenOccurrence.ID, pgtype.UUID{}, "award", 1, "secret", "Verification hidden reward", "verification:hidden:alice")

	episodes, err := queries.ListInstanceEpisodes(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list verification episodes: %v", err)
	}
	nextEpisodeLabel := ""
	nextEpisodeAirsAt := time.Time{}
	now := verificationGameplayNow()
	for _, episode := range episodes {
		if episode.AirsAt.Time.After(now) {
			nextEpisodeLabel = episode.Label
			nextEpisodeAirsAt = episode.AirsAt.Time
			break
		}
	}
	if nextEpisodeLabel == "" || nextEpisodeAirsAt.IsZero() {
		t.Fatal("expected a next episode for verification instance")
	}

	startPotReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/start", instanceUUID), `{}`, "verification-token", "admin-discord")
	startPotRecorder := httptest.NewRecorder()
	router.ServeHTTP(startPotRecorder, startPotReq)
	if startPotRecorder.Code != http.StatusCreated {
		t.Fatalf("start stir the pot status = %d, body = %s", startPotRecorder.Code, startPotRecorder.Body.String())
	}
	var startPotResponse struct {
		Round struct {
			Name string `json:"name"`
		} `json:"round"`
	}
	if err := json.Unmarshal(startPotRecorder.Body.Bytes(), &startPotResponse); err != nil {
		t.Fatalf("unmarshal start stir the pot response: %v", err)
	}
	if !strings.Contains(startPotResponse.Round.Name, nextEpisodeLabel) {
		t.Fatalf("expected stir the pot round name %q to target next episode %q", startPotResponse.Round.Name, nextEpisodeLabel)
	}

	aliceContributeReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/me/contributions", instanceUUID), `{"points":5}`, "verification-token", "alice-discord")
	aliceContributeRecorder := httptest.NewRecorder()
	router.ServeHTTP(aliceContributeRecorder, aliceContributeReq)
	if aliceContributeRecorder.Code != http.StatusOK {
		t.Fatalf("stir the pot contribution status = %d, body = %s", aliceContributeRecorder.Code, aliceContributeRecorder.Body.String())
	}
	var contributionResponse struct {
		MyContributionPoints int `json:"my_contribution_points"`
		BonusPointsAvailable int `json:"bonus_points_available"`
		RevealedSecretPoints int `json:"revealed_secret_points"`
	}
	if err := json.Unmarshal(aliceContributeRecorder.Body.Bytes(), &contributionResponse); err != nil {
		t.Fatalf("unmarshal contribution response: %v", err)
	}
	if contributionResponse.MyContributionPoints != 5 || contributionResponse.BonusPointsAvailable != 6 || contributionResponse.RevealedSecretPoints != 0 {
		t.Fatalf("unexpected stir the pot contribution response: %+v", contributionResponse)
	}

	potShowReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/stir-the-pot/tribes/show?name=Lotus", instanceUUID), "", "verification-token", "admin-discord")
	potShowRecorder := httptest.NewRecorder()
	router.ServeHTTP(potShowRecorder, potShowReq)
	if potShowRecorder.Code != http.StatusOK {
		t.Fatalf("stir the pot tribe show status = %d, body = %s", potShowRecorder.Code, potShowRecorder.Body.String())
	}
	var potShowResponse struct {
		Open                     bool `json:"open"`
		ContributionPoints       int  `json:"contribution_points"`
		BonusPointsIfResolvedNow int  `json:"bonus_points_if_resolved_now"`
		Tribe                    struct {
			Name string `json:"name"`
		} `json:"tribe"`
	}
	if err := json.Unmarshal(potShowRecorder.Body.Bytes(), &potShowResponse); err != nil {
		t.Fatalf("unmarshal stir the pot tribe show response: %v", err)
	}
	if !potShowResponse.Open || potShowResponse.Tribe.Name != "Lotus" || potShowResponse.ContributionPoints != 5 || potShowResponse.BonusPointsIfResolvedNow != 2 {
		t.Fatalf("unexpected stir the pot tribe show response: %+v", potShowResponse)
	}

	closePotReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/close", instanceUUID), "", "verification-token", "admin-discord")
	closePotRecorder := httptest.NewRecorder()
	router.ServeHTTP(closePotRecorder, closePotReq)
	if closePotRecorder.Code != http.StatusOK {
		t.Fatalf("close stir the pot status = %d, body = %s", closePotRecorder.Code, closePotRecorder.Body.String())
	}
	var closePotResponse struct {
		Tribes []struct {
			ContributionPoints       int `json:"contribution_points"`
			BonusPointsEarned        int `json:"bonus_points_earned"`
			TotalPotentialPonyPoints int `json:"total_potential_pony_points"`
			Tribe                    struct {
				Name string `json:"name"`
			} `json:"tribe"`
		} `json:"tribes"`
	}
	if err := json.Unmarshal(closePotRecorder.Body.Bytes(), &closePotResponse); err != nil {
		t.Fatalf("unmarshal close stir the pot response: %v", err)
	}
	foundLotus := false
	for _, tribe := range closePotResponse.Tribes {
		if tribe.Tribe.Name != "Lotus" {
			continue
		}
		foundLotus = true
		if tribe.ContributionPoints != 5 || tribe.BonusPointsEarned != 2 || tribe.TotalPotentialPonyPoints != 3 {
			t.Fatalf("unexpected lotus close response: %+v", tribe)
		}
	}
	if !foundLotus {
		t.Fatalf("expected lotus tribe in close response: %+v", closePotResponse)
	}

	closedStatusReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/stir-the-pot/me", instanceUUID), "", "verification-token", "alice-discord")
	closedStatusRecorder := httptest.NewRecorder()
	router.ServeHTTP(closedStatusRecorder, closedStatusReq)
	if closedStatusRecorder.Code != http.StatusOK {
		t.Fatalf("closed stir the pot status = %d, body = %s", closedStatusRecorder.Code, closedStatusRecorder.Body.String())
	}
	var closedStatusResponse struct {
		Open bool `json:"open"`
	}
	if err := json.Unmarshal(closedStatusRecorder.Body.Bytes(), &closedStatusResponse); err != nil {
		t.Fatalf("unmarshal closed stir the pot status: %v", err)
	}
	if closedStatusResponse.Open {
		t.Fatalf("expected stir the pot to be closed after admin close: %+v", closedStatusResponse)
	}

	repeatedCloseReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/close", instanceUUID), "", "verification-token", "admin-discord")
	repeatedCloseRecorder := httptest.NewRecorder()
	router.ServeHTTP(repeatedCloseRecorder, repeatedCloseReq)
	if repeatedCloseRecorder.Code != http.StatusNotFound {
		t.Fatalf("repeated stir the pot close status = %d, body = %s", repeatedCloseRecorder.Code, repeatedCloseRecorder.Body.String())
	}

	aliceLedgerReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", instanceUUID, aliceID), "", "verification-token", "alice-discord")
	aliceLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	var aliceLedger bonusLedgerResponse
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger after contribution: %v", err)
	}
	if aliceLedger.BonusPoints != 6 {
		t.Fatalf("expected alice combined bonus to be 6 after contribution, got %d", aliceLedger.BonusPoints)
	}
	foundContributionSpend := false
	for _, entry := range aliceLedger.Ledger {
		if entry.Reason == "Stir the Pot contribution" && entry.Points == -5 && entry.Visibility == "secret" {
			foundContributionSpend = true
		}
		if strings.Contains(entry.Reason, "Stir the Pot contribution") && entry.Visibility == "revealed" {
			t.Fatalf("did not expect revealed secret bonus point after public-covered stir the pot contribution: %+v", aliceLedger.Ledger)
		}
	}
	if !foundContributionSpend {
		t.Fatalf("expected secret stir the pot spend in alice ledger: %+v", aliceLedger.Ledger)
	}
	availableSecret, err := queries.GetAvailableSecretBalanceByParticipant(ctx, db.GetAvailableSecretBalanceByParticipantParams{InstanceID: instance.ID, ParticipantID: participantIDByName["Alice"]})
	if err != nil {
		t.Fatalf("get alice available secret balance after close: %v", err)
	}
	if availableSecret != 1 {
		t.Fatalf("expected close to preserve alice's available secret balance, got %d", availableSecret)
	}

	activities, err := queries.ListInstanceActivitiesByType(ctx, db.ListInstanceActivitiesByTypeParams{InstanceID: instance.ID, ActivityType: "tribal_pony"})
	if err != nil || len(activities) != 1 {
		t.Fatalf("list tribal pony activities: len=%d err=%v", len(activities), err)
	}
	tribalPonyActivityID := uuid.UUID(activities[0].ID.Bytes).String()
	tribalImmunityAt := nextEpisodeAirsAt.Add(time.Hour)
	createImmunityReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/activities/%s/occurrences", tribalPonyActivityID), fmt.Sprintf(`{"occurrence_type":"immunity_result","name":"Verification Tribal Immunity","effective_at":"%s","status":"recorded","metadata":{"winning_survivor_tribes":["vatu"]}}`, tribalImmunityAt.Format(time.RFC3339)), "verification-token", "admin-discord")
	createImmunityRecorder := httptest.NewRecorder()
	router.ServeHTTP(createImmunityRecorder, createImmunityReq)
	if createImmunityRecorder.Code != http.StatusCreated {
		t.Fatalf("create tribal immunity occurrence status = %d, body = %s", createImmunityRecorder.Code, createImmunityRecorder.Body.String())
	}
	var createdOccurrence struct {
		Occurrence struct {
			ID string `json:"id"`
		} `json:"occurrence"`
	}
	if err := json.Unmarshal(createImmunityRecorder.Body.Bytes(), &createdOccurrence); err != nil {
		t.Fatalf("unmarshal created tribal immunity occurrence: %v", err)
	}
	resolveImmunityReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/occurrences/%s/resolve", createdOccurrence.Occurrence.ID), "", "verification-token", "admin-discord")
	resolveImmunityRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveImmunityRecorder, resolveImmunityReq)
	if resolveImmunityRecorder.Code != http.StatusOK {
		t.Fatalf("resolve tribal immunity status = %d, body = %s", resolveImmunityRecorder.Code, resolveImmunityRecorder.Body.String())
	}

	aliceLedgerRecorder = httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger after tribal immunity status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger after tribal immunity: %v", err)
	}
	if aliceLedger.BonusPoints != 9 {
		t.Fatalf("expected alice combined bonus to be 9 after tribal immunity, got %d", aliceLedger.BonusPoints)
	}
	foundTribalPonyAward := false
	for _, entry := range aliceLedger.Ledger {
		if entry.Reason == "Lotus pony tribe won immunity with Stir the Pot bonus" && entry.Points == 3 && entry.Visibility == "public" {
			foundTribalPonyAward = true
		}
	}
	if !foundTribalPonyAward {
		t.Fatalf("expected tribal pony + stir the pot public award in alice ledger: %+v", aliceLedger.Ledger)
	}

	startAuctionReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/auction/lots/start", instanceUUID), fmt.Sprintf(`{"contestant_id":"%s"}`, joeID), "verification-token", "admin-discord")
	startAuctionRecorder := httptest.NewRecorder()
	router.ServeHTTP(startAuctionRecorder, startAuctionReq)
	if startAuctionRecorder.Code != http.StatusCreated {
		t.Fatalf("start auction lot status = %d, body = %s", startAuctionRecorder.Code, startAuctionRecorder.Body.String())
	}
	var startAuctionResponse struct {
		Lot struct {
			Name string `json:"name"`
		} `json:"lot"`
	}
	if err := json.Unmarshal(startAuctionRecorder.Body.Bytes(), &startAuctionResponse); err != nil {
		t.Fatalf("unmarshal start auction response: %v", err)
	}
	if !strings.Contains(startAuctionResponse.Lot.Name, nextEpisodeLabel) {
		t.Fatalf("expected auction lot name %q to target next episode %q", startAuctionResponse.Lot.Name, nextEpisodeLabel)
	}

	aliceBidReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/auction/contestants/%s/bid/me", instanceUUID, joeID), `{"points":6}`, "verification-token", "alice-discord")
	aliceBidRecorder := httptest.NewRecorder()
	router.ServeHTTP(aliceBidRecorder, aliceBidReq)
	if aliceBidRecorder.Code != http.StatusOK {
		t.Fatalf("alice bid status = %d, body = %s", aliceBidRecorder.Code, aliceBidRecorder.Body.String())
	}
	bobBidReq := authorizedJSONRequest(http.MethodPut, fmt.Sprintf("/instances/%s/auction/contestants/%s/bid/me", instanceUUID, joeID), `{"points":4}`, "verification-token", "bob-discord")
	bobBidRecorder := httptest.NewRecorder()
	router.ServeHTTP(bobBidRecorder, bobBidReq)
	if bobBidRecorder.Code != http.StatusOK {
		t.Fatalf("bob bid status = %d, body = %s", bobBidRecorder.Code, bobBidRecorder.Body.String())
	}

	aliceLedgerRecorder = httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger after bid status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger after bid: %v", err)
	}
	if aliceLedger.BonusPoints != 3 {
		t.Fatalf("expected alice combined bonus to be 3 after bid debit, got %d", aliceLedger.BonusPoints)
	}

	stopAuctionReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/auction/lots/%s/stop", instanceUUID, joeID), "", "verification-token", "admin-discord")
	stopAuctionRecorder := httptest.NewRecorder()
	router.ServeHTTP(stopAuctionRecorder, stopAuctionReq)
	if stopAuctionRecorder.Code != http.StatusOK {
		t.Fatalf("stop auction lot status = %d, body = %s", stopAuctionRecorder.Code, stopAuctionRecorder.Body.String())
	}
	var stopAuctionResponse struct {
		WinningBidPoints int `json:"winning_bid_points"`
		PricePoints      int `json:"price_points"`
		Winner           *struct {
			ParticipantID   string `json:"participant_id"`
			ParticipantName string `json:"participant_name"`
		} `json:"winner"`
	}
	if err := json.Unmarshal(stopAuctionRecorder.Body.Bytes(), &stopAuctionResponse); err != nil {
		t.Fatalf("unmarshal stop auction response: %v", err)
	}
	if stopAuctionResponse.WinningBidPoints != 6 || stopAuctionResponse.PricePoints != 4 {
		t.Fatalf("unexpected stop auction response: %+v", stopAuctionResponse)
	}
	if stopAuctionResponse.Winner == nil || stopAuctionResponse.Winner.ParticipantID != aliceID {
		t.Fatalf("expected alice to win auction lot, got %+v", stopAuctionResponse.Winner)
	}

	aliceLedgerRecorder = httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger after auction resolution status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger after auction stop: %v", err)
	}
	if aliceLedger.BonusPoints != 5 {
		t.Fatalf("expected alice combined bonus to be 5 after auction resolution, got %d", aliceLedger.BonusPoints)
	}
	alicePoniesReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/ponies/me", instanceUUID), "", "verification-token", "alice-discord")
	alicePoniesRecorder := httptest.NewRecorder()
	router.ServeHTTP(alicePoniesRecorder, alicePoniesReq)
	if alicePoniesRecorder.Code != http.StatusOK {
		t.Fatalf("alice ponies status = %d, body = %s", alicePoniesRecorder.Code, alicePoniesRecorder.Body.String())
	}
	var poniesResponse struct {
		Ponies []struct {
			ContestantName string `json:"contestant_name"`
		} `json:"ponies"`
	}
	if err := json.Unmarshal(alicePoniesRecorder.Body.Bytes(), &poniesResponse); err != nil {
		t.Fatalf("unmarshal ponies response: %v", err)
	}
	if len(poniesResponse.Ponies) != 1 || poniesResponse.Ponies[0].ContestantName != "Joe" {
		t.Fatalf("unexpected ponies response: %+v", poniesResponse)
	}

	borrowReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/loan-shark/me/borrow", instanceUUID), `{"points":3}`, "verification-token", "alice-discord")
	borrowRecorder := httptest.NewRecorder()
	router.ServeHTTP(borrowRecorder, borrowReq)
	if borrowRecorder.Code != http.StatusOK {
		t.Fatalf("borrow loan shark status = %d, body = %s", borrowRecorder.Code, borrowRecorder.Body.String())
	}
	loanStatusReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/loan-shark/me", instanceUUID), "", "verification-token", "alice-discord")
	loanStatusRecorder := httptest.NewRecorder()
	router.ServeHTTP(loanStatusRecorder, loanStatusReq)
	if loanStatusRecorder.Code != http.StatusOK {
		t.Fatalf("loan status after borrow = %d, body = %s", loanStatusRecorder.Code, loanStatusRecorder.Body.String())
	}
	var loanStatusResponse struct {
		Loan struct {
			PrincipalPoints            int `json:"principal_points"`
			InterestPoints             int `json:"interest_points"`
			TotalDuePoints             int `json:"total_due_points"`
			PrincipalOutstandingPoints int `json:"principal_outstanding_points"`
			InterestOutstandingPoints  int `json:"interest_outstanding_points"`
		} `json:"loan"`
	}
	if err := json.Unmarshal(loanStatusRecorder.Body.Bytes(), &loanStatusResponse); err != nil {
		t.Fatalf("unmarshal loan status after borrow: %v", err)
	}
	if loanStatusResponse.Loan.PrincipalPoints != 3 || loanStatusResponse.Loan.InterestPoints != 1 || loanStatusResponse.Loan.TotalDuePoints != 4 {
		t.Fatalf("unexpected loan status after borrow: %+v", loanStatusResponse)
	}

	repayReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/loan-shark/me/repay", instanceUUID), `{"points":2}`, "verification-token", "alice-discord")
	repayRecorder := httptest.NewRecorder()
	router.ServeHTTP(repayRecorder, repayReq)
	if repayRecorder.Code != http.StatusOK {
		t.Fatalf("repay loan shark status = %d, body = %s", repayRecorder.Code, repayRecorder.Body.String())
	}
	var repayResponse struct {
		RevealedSecretPoints int `json:"revealed_secret_points"`
	}
	if err := json.Unmarshal(repayRecorder.Body.Bytes(), &repayResponse); err != nil {
		t.Fatalf("unmarshal loan repay response: %v", err)
	}
	if repayResponse.RevealedSecretPoints != 0 {
		t.Fatalf("expected no secret reveal on repay when visible balance covers it, got %+v", repayResponse)
	}
	loanStatusRecorder = httptest.NewRecorder()
	router.ServeHTTP(loanStatusRecorder, loanStatusReq)
	if loanStatusRecorder.Code != http.StatusOK {
		t.Fatalf("loan status after repay = %d, body = %s", loanStatusRecorder.Code, loanStatusRecorder.Body.String())
	}
	if err := json.Unmarshal(loanStatusRecorder.Body.Bytes(), &loanStatusResponse); err != nil {
		t.Fatalf("unmarshal loan status after repay: %v", err)
	}
	if loanStatusResponse.Loan.PrincipalOutstandingPoints != 2 || loanStatusResponse.Loan.InterestOutstandingPoints != 0 || loanStatusResponse.Loan.TotalDuePoints != 2 {
		t.Fatalf("unexpected loan status after repay: %+v", loanStatusResponse)
	}

	individualImmunityAt := verificationGameplayNow().Add(2 * time.Hour)
	immunityReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/individual-pony/immunity", instanceUUID), fmt.Sprintf(`{"contestant_id":"%s","effective_at":"%s"}`, joeID, individualImmunityAt.Format(time.RFC3339)), "verification-token", "admin-discord")
	immunityRecorder := httptest.NewRecorder()
	router.ServeHTTP(immunityRecorder, immunityReq)
	if immunityRecorder.Code != http.StatusOK {
		t.Fatalf("record individual pony immunity status = %d, body = %s", immunityRecorder.Code, immunityRecorder.Body.String())
	}

	aliceLedgerRecorder = httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger after individual immunity status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger after individual immunity: %v", err)
	}
	if aliceLedger.BonusPoints != 9 {
		t.Fatalf("expected alice combined bonus to be 9 after full flow, got %d", aliceLedger.BonusPoints)
	}

	leaderboardReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/leaderboard", instanceUUID), "", "verification-token", "")
	leaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(leaderboardRecorder, leaderboardReq)
	if leaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("leaderboard status = %d, body = %s", leaderboardRecorder.Code, leaderboardRecorder.Body.String())
	}
	var leaderboard leaderboardResponse
	if err := json.Unmarshal(leaderboardRecorder.Body.Bytes(), &leaderboard); err != nil {
		t.Fatalf("unmarshal leaderboard: %v", err)
	}
	aliceFound := false
	bobFound := false
	for _, row := range leaderboard.Leaderboard {
		switch row.ParticipantID {
		case aliceID:
			aliceFound = true
			if row.BonusPoints != 16 {
				t.Fatalf("expected alice public bonus to remain 16 without unnecessary secret reveals, got %d", row.BonusPoints)
			}
			if row.ParticipantDiscordUserID != "alice-discord" || row.CurrentTribeName != "Lotus" {
				t.Fatalf("unexpected alice leaderboard row: %+v", row)
			}
		case bobID:
			bobFound = true
			if row.BonusPoints != 10 {
				t.Fatalf("expected bob public bonus to be 10 after losing bid refund, got %d", row.BonusPoints)
			}
			if row.ParticipantDiscordUserID != "bob-discord" || row.CurrentTribeName != "Lotus" {
				t.Fatalf("unexpected bob leaderboard row: %+v", row)
			}
		}
	}
	if !aliceFound || !bobFound {
		t.Fatalf("expected alice and bob in leaderboard: %+v", leaderboard.Leaderboard)
	}
}

func TestStirThePotCloseRollsBackMalformedContribution(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	seasons, err := seeddata.LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if _, err := appinternal.SeedHistorical(ctx, pool, seasons); err != nil {
		t.Fatalf("seed verification season: %v", err)
	}

	queries := db.New(pool)
	instances, err := queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	var instance db.ListInstancesRow
	for _, candidate := range instances {
		if candidate.Name == "Verification Merge Gameplay" {
			instance = candidate
			break
		}
	}
	if !instance.ID.Valid {
		t.Fatalf("verification instance not found")
	}
	participants, err := queries.ListParticipantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	var aliceID pgtype.UUID
	for _, participant := range participants {
		if participant.Name == "Alice" {
			aliceID = participant.ID
			break
		}
	}
	if !aliceID.Valid {
		t.Fatalf("alice participant not found")
	}
	groups, err := queries.ListParticipantGroupsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participant groups: %v", err)
	}
	var lotusID pgtype.UUID
	for _, group := range groups {
		if group.Name == "Lotus" {
			lotusID = group.ID
			break
		}
	}
	if !lotusID.Valid {
		t.Fatalf("lotus group not found")
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}

	server := httpapi.New(pool,
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}),
		httpapi.WithClock(verificationGameplayNow),
	)
	router := server.Router()
	instanceUUID := uuid.UUID(instance.ID.Bytes).String()
	startReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/start", instanceUUID), `{}`, "verification-token", "admin-discord")
	startRecorder := httptest.NewRecorder()
	router.ServeHTTP(startRecorder, startReq)
	if startRecorder.Code != http.StatusCreated {
		t.Fatalf("start stir the pot status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	var startResponse struct {
		Round struct {
			ID string `json:"id"`
		} `json:"round"`
	}
	if err := json.Unmarshal(startRecorder.Body.Bytes(), &startResponse); err != nil {
		t.Fatalf("unmarshal start response: %v", err)
	}
	roundUUID, err := uuid.Parse(startResponse.Round.ID)
	if err != nil {
		t.Fatalf("parse round id: %v", err)
	}
	roundID := pgtype.UUID{Bytes: roundUUID, Valid: true}
	createLedgerEntryWithMetadataForTest(t, ctx, queries, instance.ID, aliceID, roundID, pgtype.UUID{}, "conversion", -1, "secret", "Test secret conversion", "test:secret:conversion", []byte(`{"consumes_secret_balance":true}`))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, aliceID, roundID, pgtype.UUID{}, "reveal", 1, "revealed", "Test secret reveal", "test:secret:reveal")
	createLedgerEntryWithMetadataForTest(t, ctx, queries, instance.ID, aliceID, roundID, pgtype.UUID{}, "spend", -1, "secret", "Stir the Pot contribution", "test:secret:spend", []byte(`{"consumes_secret_balance":false}`))

	ledgerSnapshot := func() []string {
		rows, err := pool.Query(ctx, `
			SELECT bple.entry_kind, bple.points, bple.visibility, bple.reason,
			       COALESCE(bple.award_key, ''), bple.metadata::text
			FROM bonus_point_ledger_entries bple
			JOIN activity_occurrences ao ON ao.id = bple.activity_occurrence_id
			WHERE ao.public_id = $1
			ORDER BY bple.id
		`, roundID)
		if err != nil {
			t.Fatalf("list round ledger: %v", err)
		}
		defer rows.Close()
		var snapshot []string
		for rows.Next() {
			var entryKind, visibility, reason, awardKey, metadata string
			var points int32
			if err := rows.Scan(&entryKind, &points, &visibility, &reason, &awardKey, &metadata); err != nil {
				t.Fatalf("scan round ledger: %v", err)
			}
			snapshot = append(snapshot, fmt.Sprintf("%s|%d|%s|%s|%s|%s", entryKind, points, visibility, reason, awardKey, metadata))
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("read round ledger: %v", err)
		}
		return snapshot
	}
	beforeLedger := ledgerSnapshot()
	beforeVisible, err := queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instance.ID, ParticipantID: aliceID})
	if err != nil {
		t.Fatalf("get visible balance before close: %v", err)
	}
	beforeSecret, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instance.ID, ParticipantID: aliceID})
	if err != nil {
		t.Fatalf("get secret balance before close: %v", err)
	}

	if _, err := queries.UpsertActivityOccurrenceParticipant(ctx, db.UpsertActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: roundID,
		ParticipantID:        aliceID,
		ParticipantGroupID:   lotusID,
		Role:                 "contributor",
		Metadata:             []byte(`{"contribution":"invalid"}`),
	}); err != nil {
		t.Fatalf("insert malformed contribution: %v", err)
	}

	closePot := func() *httptest.ResponseRecorder {
		req := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/close", instanceUUID), "", "verification-token", "admin-discord")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	failedClose := closePot()
	if failedClose.Code != http.StatusInternalServerError {
		t.Fatalf("malformed contribution close status = %d, body = %s", failedClose.Code, failedClose.Body.String())
	}
	failedRound, err := queries.GetActivityOccurrence(ctx, roundID)
	if err != nil {
		t.Fatalf("get round after failed close: %v", err)
	}
	if failedRound.Status != "recorded" || failedRound.EndsAt.Valid {
		t.Fatalf("close failure partially changed round: %+v", failedRound)
	}
	var failedMetadata map[string]any
	if err := json.Unmarshal(failedRound.Metadata, &failedMetadata); err != nil {
		t.Fatalf("unmarshal round metadata after failed close: %v", err)
	}
	if _, closed := failedMetadata["closed_at"]; closed {
		t.Fatalf("close failure left closed_at metadata: %+v", failedMetadata)
	}

	if _, err := queries.UpsertActivityOccurrenceParticipant(ctx, db.UpsertActivityOccurrenceParticipantParams{
		ActivityOccurrenceID: roundID,
		ParticipantID:        aliceID,
		ParticipantGroupID:   lotusID,
		Role:                 "contributor",
		Metadata:             []byte(`{"contribution":0}`),
	}); err != nil {
		t.Fatalf("repair malformed contribution: %v", err)
	}
	successfulClose := closePot()
	if successfulClose.Code != http.StatusOK {
		t.Fatalf("repaired close status = %d, body = %s", successfulClose.Code, successfulClose.Body.String())
	}
	closedRound, err := queries.GetActivityOccurrence(ctx, roundID)
	if err != nil {
		t.Fatalf("get round after successful close: %v", err)
	}
	if closedRound.Status != "recorded" || !closedRound.EndsAt.Valid {
		t.Fatalf("unexpected closed round state: %+v", closedRound)
	}
	var closedMetadata map[string]any
	if err := json.Unmarshal(closedRound.Metadata, &closedMetadata); err != nil {
		t.Fatalf("unmarshal round metadata after successful close: %v", err)
	}
	if closedAt, ok := closedMetadata["closed_at"].(string); !ok || closedAt == "" {
		t.Fatalf("expected closed_at metadata after successful close: %+v", closedMetadata)
	}
	afterLedger := ledgerSnapshot()
	if fmt.Sprint(afterLedger) != fmt.Sprint(beforeLedger) {
		t.Fatalf("close changed round ledger rows: before=%v after=%v", beforeLedger, afterLedger)
	}
	afterVisible, err := queries.GetVisibleBonusTotalByParticipant(ctx, db.GetVisibleBonusTotalByParticipantParams{InstanceID: instance.ID, ParticipantID: aliceID})
	if err != nil {
		t.Fatalf("get visible balance after close: %v", err)
	}
	afterSecret, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{InstanceID: instance.ID, ParticipantID: aliceID})
	if err != nil {
		t.Fatalf("get secret balance after close: %v", err)
	}
	if afterVisible != beforeVisible || afterSecret != beforeSecret {
		t.Fatalf("close changed balances: before=(%d,%d) after=(%d,%d)", beforeVisible, beforeSecret, afterVisible, afterSecret)
	}

	retryClose := closePot()
	if retryClose.Code != http.StatusNotFound {
		t.Fatalf("repeated close status = %d, body = %s", retryClose.Code, retryClose.Body.String())
	}
	retryRound, err := queries.GetActivityOccurrence(ctx, roundID)
	if err != nil {
		t.Fatalf("get round after repeated close: %v", err)
	}
	if string(retryRound.Metadata) != string(closedRound.Metadata) || !retryRound.EndsAt.Time.Equal(closedRound.EndsAt.Time) || !retryRound.UpdatedAt.Time.Equal(closedRound.UpdatedAt.Time) {
		t.Fatalf("repeated close mutated persisted round: before=%+v after=%+v", closedRound, retryRound)
	}
	retryLedger := ledgerSnapshot()
	if fmt.Sprint(retryLedger) != fmt.Sprint(afterLedger) {
		t.Fatalf("repeated close mutated round ledger: before=%v after=%v", afterLedger, retryLedger)
	}
}

func TestStirThePotContributionDoesNotRevealSecretWhenVisibleBalanceCoversSpend(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	seasons, err := seeddata.LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if _, err := appinternal.SeedHistorical(ctx, pool, seasons); err != nil {
		t.Fatalf("seed verification season: %v", err)
	}

	queries := db.New(pool)
	instances, err := queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	var instance db.ListInstancesRow
	for _, candidate := range instances {
		if candidate.Name == "Verification Merge Gameplay" {
			instance = candidate
			break
		}
	}
	if !instance.ID.Valid {
		t.Fatalf("verification instance not found")
	}

	participants, err := queries.ListParticipantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	participantIDByName := make(map[string]pgtype.UUID, len(participants))
	for _, participant := range participants {
		participantIDByName[participant.Name] = participant.ID
	}
	if _, err := queries.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: participantIDByName["Bob"], DiscordUserID: pgtype.Text{String: "bob-discord", Valid: true}}); err != nil {
		t.Fatalf("link bob: %v", err)
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}

	server := httpapi.New(pool,
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}),
		httpapi.WithClock(verificationGameplayNow),
	)
	router := server.Router()
	instanceUUID := uuid.UUID(instance.ID.Bytes).String()
	bobID := uuid.UUID(participantIDByName["Bob"].Bytes).String()

	publicActivity := createActivityForTest(t, ctx, queries, instance.ID, verificationGameplayNow().Add(-3*time.Hour), nil, "manual_adjustment", "Verification Public Bonus")
	publicOccurrence := createOccurrenceForTest(t, ctx, queries, publicActivity.ID, "manual_correction", "Verification Public Bonus Grant", verificationGameplayNow().Add(-3*time.Hour))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Bob"], publicOccurrence.ID, pgtype.UUID{}, "award", 3, "public", "Verification public reward", "verification:public:bob")

	hiddenActivity := createActivityForTest(t, ctx, queries, instance.ID, verificationGameplayNow().Add(-2*time.Hour), nil, "manual_adjustment", "Verification Hidden Bonus")
	hiddenOccurrence := createOccurrenceForTest(t, ctx, queries, hiddenActivity.ID, "manual_correction", "Verification Hidden Bonus Grant", verificationGameplayNow().Add(-2*time.Hour))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Bob"], hiddenOccurrence.ID, pgtype.UUID{}, "award", 1, "secret", "Verification hidden reward", "verification:hidden:bob")

	startPotReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/start", instanceUUID), `{}`, "verification-token", "admin-discord")
	startPotRecorder := httptest.NewRecorder()
	router.ServeHTTP(startPotRecorder, startPotReq)
	if startPotRecorder.Code != http.StatusCreated {
		t.Fatalf("start stir the pot status = %d, body = %s", startPotRecorder.Code, startPotRecorder.Body.String())
	}

	bobContributeReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/stir-the-pot/me/contributions", instanceUUID), `{"points":2}`, "verification-token", "bob-discord")
	bobContributeRecorder := httptest.NewRecorder()
	router.ServeHTTP(bobContributeRecorder, bobContributeReq)
	if bobContributeRecorder.Code != http.StatusOK {
		t.Fatalf("bob stir the pot contribution status = %d, body = %s", bobContributeRecorder.Code, bobContributeRecorder.Body.String())
	}
	var contributionResponse struct {
		RevealedSecretPoints int `json:"revealed_secret_points"`
	}
	if err := json.Unmarshal(bobContributeRecorder.Body.Bytes(), &contributionResponse); err != nil {
		t.Fatalf("unmarshal bob contribution response: %v", err)
	}
	if contributionResponse.RevealedSecretPoints != 0 {
		t.Fatalf("expected no secret reveal when visible balance covers spend, got %+v", contributionResponse)
	}

	bobLedgerReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", instanceUUID, bobID), "", "verification-token", "bob-discord")
	bobLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(bobLedgerRecorder, bobLedgerReq)
	if bobLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("bob bonus ledger status = %d, body = %s", bobLedgerRecorder.Code, bobLedgerRecorder.Body.String())
	}
	var bobLedger bonusLedgerResponse
	if err := json.Unmarshal(bobLedgerRecorder.Body.Bytes(), &bobLedger); err != nil {
		t.Fatalf("unmarshal bob ledger after contribution: %v", err)
	}
	foundContributionSpend := false
	for _, entry := range bobLedger.Ledger {
		if entry.Reason == "Stir the Pot contribution" && entry.Points == -2 && entry.Visibility == "secret" {
			foundContributionSpend = true
		}
		if strings.Contains(entry.Reason, "Stir the Pot contribution") && entry.Visibility == "revealed" {
			t.Fatalf("did not expect revealed secret bonus point after public-covered contribution: %+v", bobLedger.Ledger)
		}
	}
	if !foundContributionSpend {
		t.Fatalf("expected secret stir the pot spend in bob ledger: %+v", bobLedger.Ledger)
	}
}

func TestRecordMergeAuctionResults_CreatesOwnershipsAndPublicSpends(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	seasons, err := seeddata.LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if _, err := appinternal.SeedHistorical(ctx, pool, seasons); err != nil {
		t.Fatalf("seed verification season: %v", err)
	}

	queries := db.New(pool)
	instances, err := queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	var instance db.ListInstancesRow
	for _, candidate := range instances {
		if candidate.Name == "Verification Merge Gameplay" {
			instance = candidate
			break
		}
	}
	if !instance.ID.Valid {
		t.Fatalf("verification instance not found")
	}

	participants, err := queries.ListParticipantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	participantIDByName := make(map[string]pgtype.UUID, len(participants))
	for _, participant := range participants {
		participantIDByName[participant.Name] = participant.ID
	}
	contestants, err := queries.ListContestantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list contestants: %v", err)
	}
	if len(contestants) < 2 {
		t.Fatalf("expected at least 2 contestants, got %d", len(contestants))
	}
	contestantOne := contestants[0]
	contestantTwo := contestants[1]
	if _, err := pool.Exec(ctx, `
		DELETE FROM bonus_point_ledger_entries b
		USING instances i
		WHERE b.instance_id = i.id
		  AND i.public_id = $1
		  AND b.award_key LIKE 'verification:starting-bonus:%'
	`, instance.ID); err != nil {
		t.Fatalf("remove seeded starting bonuses: %v", err)
	}

	if _, err := queries.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: participantIDByName["Alice"], DiscordUserID: pgtype.Text{String: "alice-discord", Valid: true}}); err != nil {
		t.Fatalf("link alice: %v", err)
	}
	if _, err := queries.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: participantIDByName["Bob"], DiscordUserID: pgtype.Text{String: "bob-discord", Valid: true}}); err != nil {
		t.Fatalf("link bob: %v", err)
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}

	publicActivity := createActivityForTest(t, ctx, queries, instance.ID, verificationGameplayNow().Add(-3*time.Hour), nil, "manual_adjustment", "Verification Public Bonus")
	publicOccurrence := createOccurrenceForTest(t, ctx, queries, publicActivity.ID, "manual_correction", "Verification Public Bonus Grant", verificationGameplayNow().Add(-3*time.Hour))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Alice"], publicOccurrence.ID, pgtype.UUID{}, "award", 3, "public", "Verification public reward", "verification:public:alice")
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Bob"], publicOccurrence.ID, pgtype.UUID{}, "award", 2, "public", "Verification public reward", "verification:public:bob")

	hiddenActivity := createActivityForTest(t, ctx, queries, instance.ID, verificationGameplayNow().Add(-2*time.Hour), nil, "manual_adjustment", "Verification Hidden Bonus")
	hiddenOccurrence := createOccurrenceForTest(t, ctx, queries, hiddenActivity.ID, "manual_correction", "Verification Hidden Bonus Grant", verificationGameplayNow().Add(-2*time.Hour))
	createLedgerEntryForTest(t, ctx, queries, instance.ID, participantIDByName["Alice"], hiddenOccurrence.ID, pgtype.UUID{}, "award", 2, "secret", "Verification hidden reward", "verification:hidden:alice")

	server := httpapi.New(pool,
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}),
		httpapi.WithClock(verificationGameplayNow),
	)
	router := server.Router()
	instanceUUID := uuid.UUID(instance.ID.Bytes).String()
	aliceID := uuid.UUID(participantIDByName["Alice"].Bytes).String()
	bobID := uuid.UUID(participantIDByName["Bob"].Bytes).String()

	body := fmt.Sprintf(`{"name":"Merge Auction","mode":"three_round_blind_fallthrough","raw_csv":"test csv","results":[{"round":1,"contestant":%q,"winner":"Alice","price":4},{"round":2,"contestant":%q,"winner":"Bob","price":2}]}`,
		contestantOne.Name,
		contestantTwo.Name,
	)
	recordReq := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/merge-auction/record", instanceUUID), body, "verification-token", "admin-discord")
	recordRecorder := httptest.NewRecorder()
	router.ServeHTTP(recordRecorder, recordReq)
	if recordRecorder.Code != http.StatusOK {
		t.Fatalf("record merge auction status = %d, body = %s", recordRecorder.Code, recordRecorder.Body.String())
	}
	var recordResponse struct {
		Occurrence struct {
			ID             string          `json:"id"`
			OccurrenceType string          `json:"occurrence_type"`
			Name           string          `json:"name"`
			Metadata       json.RawMessage `json:"metadata"`
		} `json:"occurrence"`
		Results []struct {
			ContestantName       string `json:"contestant_name"`
			WinnerName           string `json:"winner_name"`
			Round                int    `json:"round"`
			Price                int    `json:"price"`
			RevealedSecretPoints int    `json:"revealed_secret_points"`
		} `json:"results"`
	}
	if err := json.Unmarshal(recordRecorder.Body.Bytes(), &recordResponse); err != nil {
		t.Fatalf("unmarshal merge auction response: %v", err)
	}
	if recordResponse.Occurrence.OccurrenceType != "merge_auction_result" || recordResponse.Occurrence.Name != "Merge Auction" {
		t.Fatalf("unexpected merge auction occurrence: %+v", recordResponse.Occurrence)
	}
	if len(recordResponse.Results) != 2 {
		t.Fatalf("expected 2 applied merge auction results, got %+v", recordResponse.Results)
	}
	if recordResponse.Results[0].WinnerName != "Alice" || recordResponse.Results[0].ContestantName != contestantOne.Name || recordResponse.Results[0].Price != 4 || recordResponse.Results[0].RevealedSecretPoints != 1 {
		t.Fatalf("unexpected first merge auction result: %+v", recordResponse.Results[0])
	}
	if recordResponse.Results[1].WinnerName != "Bob" || recordResponse.Results[1].ContestantName != contestantTwo.Name || recordResponse.Results[1].Price != 2 || recordResponse.Results[1].RevealedSecretPoints != 0 {
		t.Fatalf("unexpected second merge auction result: %+v", recordResponse.Results[1])
	}

	alicePonies, err := queries.ListActiveParticipantPonyOwnershipsByOwnerAt(ctx, db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{InstanceID: instance.ID, OwnerParticipantID: participantIDByName["Alice"], At: timestamptz(verificationGameplayNow())})
	if err != nil {
		t.Fatalf("list alice ponies: %v", err)
	}
	if len(alicePonies) != 1 || alicePonies[0].ContestantName != contestantOne.Name {
		t.Fatalf("unexpected alice ponies: %+v", alicePonies)
	}
	bobPonies, err := queries.ListActiveParticipantPonyOwnershipsByOwnerAt(ctx, db.ListActiveParticipantPonyOwnershipsByOwnerAtParams{InstanceID: instance.ID, OwnerParticipantID: participantIDByName["Bob"], At: timestamptz(verificationGameplayNow())})
	if err != nil {
		t.Fatalf("list bob ponies: %v", err)
	}
	if len(bobPonies) != 1 || bobPonies[0].ContestantName != contestantTwo.Name {
		t.Fatalf("unexpected bob ponies: %+v", bobPonies)
	}

	aliceLedgerReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", instanceUUID, aliceID), "", "verification-token", "alice-discord")
	aliceLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(aliceLedgerRecorder, aliceLedgerReq)
	if aliceLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("alice bonus ledger status = %d, body = %s", aliceLedgerRecorder.Code, aliceLedgerRecorder.Body.String())
	}
	var aliceLedger bonusLedgerResponse
	if err := json.Unmarshal(aliceLedgerRecorder.Body.Bytes(), &aliceLedger); err != nil {
		t.Fatalf("unmarshal alice ledger: %v", err)
	}
	if aliceLedger.BonusPoints != 1 {
		t.Fatalf("expected alice total bonus to be 1 after merge auction, got %d", aliceLedger.BonusPoints)
	}
	foundAliceSpend := false
	foundAliceReveal := false
	foundAliceConversion := false
	for _, entry := range aliceLedger.Ledger {
		if entry.Reason == fmt.Sprintf("Merge Auction: won %s for %d", contestantOne.Name, 4) && entry.Points == -4 && entry.Visibility == "public" {
			foundAliceSpend = true
		}
		if strings.Contains(entry.Reason, fmt.Sprintf("Merge Auction bid on %s", contestantOne.Name)) && entry.EntryKind == "reveal" && entry.Points == 1 && entry.Visibility == "revealed" {
			foundAliceReveal = true
		}
		if strings.Contains(entry.Reason, fmt.Sprintf("Merge Auction bid on %s", contestantOne.Name)) && entry.EntryKind == "conversion" && entry.Points == -1 && entry.Visibility == "secret" {
			foundAliceConversion = true
		}
	}
	if !foundAliceSpend || !foundAliceReveal || !foundAliceConversion {
		t.Fatalf("unexpected alice ledger entries: %+v", aliceLedger.Ledger)
	}

	bobLedgerReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/participants/%s/bonus-ledger", instanceUUID, bobID), "", "verification-token", "bob-discord")
	bobLedgerRecorder := httptest.NewRecorder()
	router.ServeHTTP(bobLedgerRecorder, bobLedgerReq)
	if bobLedgerRecorder.Code != http.StatusOK {
		t.Fatalf("bob bonus ledger status = %d, body = %s", bobLedgerRecorder.Code, bobLedgerRecorder.Body.String())
	}
	var bobLedger bonusLedgerResponse
	if err := json.Unmarshal(bobLedgerRecorder.Body.Bytes(), &bobLedger); err != nil {
		t.Fatalf("unmarshal bob ledger: %v", err)
	}
	if bobLedger.BonusPoints != 0 {
		t.Fatalf("expected bob total bonus to be 0 after merge auction, got %d", bobLedger.BonusPoints)
	}
	foundBobSpend := false
	for _, entry := range bobLedger.Ledger {
		if entry.Reason == fmt.Sprintf("Merge Auction: won %s for %d", contestantTwo.Name, 2) && entry.Points == -2 && entry.Visibility == "public" {
			foundBobSpend = true
		}
		if strings.Contains(entry.Reason, fmt.Sprintf("Merge Auction bid on %s", contestantTwo.Name)) && entry.Visibility == "revealed" {
			t.Fatalf("did not expect bob secret reveal entries: %+v", bobLedger.Ledger)
		}
	}
	if !foundBobSpend {
		t.Fatalf("unexpected bob ledger entries: %+v", bobLedger.Ledger)
	}

	leaderboardReq := authorizedJSONRequest(http.MethodGet, fmt.Sprintf("/instances/%s/leaderboard", instanceUUID), "", "verification-token", "admin-discord")
	leaderboardRecorder := httptest.NewRecorder()
	router.ServeHTTP(leaderboardRecorder, leaderboardReq)
	if leaderboardRecorder.Code != http.StatusOK {
		t.Fatalf("leaderboard status = %d, body = %s", leaderboardRecorder.Code, leaderboardRecorder.Body.String())
	}
	var leaderboard leaderboardResponse
	if err := json.Unmarshal(leaderboardRecorder.Body.Bytes(), &leaderboard); err != nil {
		t.Fatalf("unmarshal leaderboard: %v", err)
	}
	foundAliceLeaderboard := false
	foundBobLeaderboard := false
	for _, row := range leaderboard.Leaderboard {
		switch row.ParticipantName {
		case "Alice":
			foundAliceLeaderboard = true
			if row.BonusPoints != 0 {
				t.Fatalf("expected alice visible bonus 0 after public spend, got %+v", row)
			}
		case "Bob":
			foundBobLeaderboard = true
			if row.BonusPoints != 0 {
				t.Fatalf("expected bob visible bonus 0 after public spend, got %+v", row)
			}
		}
	}
	if !foundAliceLeaderboard || !foundBobLeaderboard {
		t.Fatalf("expected alice and bob on leaderboard: %+v", leaderboard.Leaderboard)
	}
}

func TestRecordMergeAuctionResults_RejectsDuplicateContestantResult(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)

	seasons, err := seeddata.LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if _, err := appinternal.SeedHistorical(ctx, pool, seasons); err != nil {
		t.Fatalf("seed verification season: %v", err)
	}

	queries := db.New(pool)
	instances, err := queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	var instance db.ListInstancesRow
	for _, candidate := range instances {
		if candidate.Name == "Verification Merge Gameplay" {
			instance = candidate
			break
		}
	}
	if !instance.ID.Valid {
		t.Fatalf("verification instance not found")
	}
	participants, err := queries.ListParticipantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	participantNames := make([]string, 0, len(participants))
	for _, participant := range participants {
		participantNames = append(participantNames, participant.Name)
	}
	contestants, err := queries.ListContestantsByInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("list contestants: %v", err)
	}
	if len(participantNames) < 2 || len(contestants) == 0 {
		t.Fatalf("expected verification participants and contestants")
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}))
	router := server.Router()
	instanceUUID := uuid.UUID(instance.ID.Bytes).String()
	body := fmt.Sprintf(`{"results":[{"round":1,"contestant":%q,"winner":%q,"price":1},{"round":2,"contestant":%q,"winner":%q,"price":2}]}`,
		contestants[0].Name,
		participantNames[0],
		contestants[0].Name,
		participantNames[1],
	)
	req := authorizedJSONRequest(http.MethodPost, fmt.Sprintf("/instances/%s/merge-auction/record", instanceUUID), body, "verification-token", "admin-discord")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate contestant result to fail with 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "duplicate contestant result") {
		t.Fatalf("expected duplicate contestant error, got %s", recorder.Body.String())
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

	databaseName := "castaway_httpapi_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedDatabaseName := pgx.Identifier{databaseName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+quotedDatabaseName); err != nil {
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
		if _, dropErr := adminPool.Exec(ctx, "DROP DATABASE IF EXISTS "+quotedDatabaseName+" WITH (FORCE)"); dropErr != nil {
			t.Logf("drop temp database after pool creation failure: %v", dropErr)
		}
		adminPool.Close()
		t.Fatalf("create temp pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if _, err := adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+quotedDatabaseName+" WITH (FORCE)"); err != nil {
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

func authorizedJSONRequest(method, path, body, bearerToken, discordUserID string) *http.Request {
	var requestBody *strings.Reader
	if body == "" {
		requestBody = strings.NewReader("")
	} else {
		requestBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, requestBody)
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if discordUserID != "" {
		req.Header.Set("X-Discord-User-ID", discordUserID)
	}
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
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

func createParticipantGroupForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, name, kind string) db.CreateParticipantGroupRow {
	t.Helper()
	group, err := queries.CreateParticipantGroup(ctx, db.CreateParticipantGroupParams{
		InstanceID: instanceID,
		Name:       name,
		Kind:       kind,
		Metadata:   testEmptyJSONB,
	})
	if err != nil {
		t.Fatalf("create participant group %q: %v", name, err)
	}
	return group
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
	createLedgerEntryWithMetadataForTest(t, ctx, queries, instanceID, participantID, occurrenceID, sourceGroupID, entryKind, points, visibility, reason, awardKey, testEmptyJSONB)
}

func createLedgerEntryWithMetadataForTest(t *testing.T, ctx context.Context, queries *db.Queries, instanceID, participantID, occurrenceID, sourceGroupID pgtype.UUID, entryKind string, points int32, visibility string, reason string, awardKey string, metadata []byte) {
	t.Helper()
	if len(metadata) == 0 {
		metadata = testEmptyJSONB
	}
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instanceID,
		ParticipantID:        participantID,
		ActivityOccurrenceID: occurrenceID,
		SourceGroupID:        sourceGroupID,
		EntryKind:            entryKind,
		Points:               points,
		Visibility:           visibility,
		Reason:               reason,
		EffectiveAt:          timestamptz(verificationGameplayNow()),
		AwardKey:             pgtype.Text{String: awardKey, Valid: true},
		Metadata:             metadata,
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
