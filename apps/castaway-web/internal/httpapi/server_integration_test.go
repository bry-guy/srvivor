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

	createOccurrenceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/activities/%s/occurrences", createdActivity.Activity.ID), strings.NewReader(`{"occurrence_type":"manual_correction","name":"Episode 1 Correction","effective_at":"2026-03-22T09:00:00Z","status":"pending","metadata":{"note":"adjustment"}}`))
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

	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{DiscordAdminUserIDs: []string{"admin-1"}}))
	router := server.Router()

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
