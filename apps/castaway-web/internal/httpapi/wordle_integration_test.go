package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	wordleActivityType  = "tribe_wordle"
	wordleRoundKeyLimit = 64
)

type wordleFixtureClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *wordleFixtureClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *wordleFixtureClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func TestWordleRoundLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)

	base := time.Date(2030, time.January, 6, 12, 0, 0, 0, time.UTC)
	clock := &wordleFixtureClock{now: base.Add(-time.Minute)}
	instance := createInstanceForTest(t, ctx, queries, "Wordle Lifecycle", 901)
	otherInstance := createInstanceForTest(t, ctx, queries, "Wordle Other", 902)
	activity := createActivityForTest(t, ctx, queries, instance.ID, base.Add(-time.Hour), nil, wordleActivityType, "Weekly Wordle")
	historicalActivity := createActivityForTest(t, ctx, queries, instance.ID, base.Add(-2*time.Hour), nil, "manual_adjustment", "Historical secret balance")
	historicalOccurrence, err := queries.CreateActivityOccurrence(ctx, db.CreateActivityOccurrenceParams{
		ActivityID:     historicalActivity.ID,
		OccurrenceType: "historical_secret",
		Name:           "Historical secret balance",
		EffectiveAt:    wordleTimestamp(base.Add(-2 * time.Hour)),
		Status:         "recorded",
		Metadata:       []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create historical occurrence: %v", err)
	}
	participants := map[string]db.CreateParticipantRow{
		"alice":    createParticipantForTest(t, ctx, queries, instance.ID, "Alice"),
		"bob":      createParticipantForTest(t, ctx, queries, instance.ID, "Bob"),
		"charlie":  createParticipantForTest(t, ctx, queries, instance.ID, "Charlie"),
		"dana":     createParticipantForTest(t, ctx, queries, instance.ID, "Dana"),
		"outsider": createParticipantForTest(t, ctx, queries, instance.ID, "Outsider"),
		"other":    createParticipantForTest(t, ctx, queries, otherInstance.ID, "Other"),
	}
	if _, err := queries.CreateBonusPointLedgerEntry(ctx, db.CreateBonusPointLedgerEntryParams{
		InstanceID:           instance.ID,
		ParticipantID:        participants["alice"].ID,
		ActivityOccurrenceID: historicalOccurrence.ID,
		EntryKind:            "award",
		Points:               2,
		Visibility:           "secret",
		Reason:               "historical test balance",
		EffectiveAt:          wordleTimestamp(base.Add(-2 * time.Hour)),
		AwardKey:             pgtype.Text{String: "test:historical-secret", Valid: true},
		Metadata:             []byte(`{}`),
	}); err != nil {
		t.Fatalf("create historical secret ledger entry: %v", err)
	}
	groups := map[string]db.CreateParticipantGroupRow{
		"lotus":   createParticipantGroupForTest(t, ctx, queries, instance.ID, "Lotus", "tribe"),
		"leaf":    createParticipantGroupForTest(t, ctx, queries, instance.ID, "Leaf", "tribe"),
		"invalid": createParticipantGroupForTest(t, ctx, queries, instance.ID, "Invalid", "bench"),
		"other":   createParticipantGroupForTest(t, ctx, queries, otherInstance.ID, "Other", "tribe"),
	}
	for _, participant := range []db.CreateParticipantRow{participants["alice"], participants["bob"], participants["dana"]} {
		createWordleMembership(t, ctx, queries, groups["lotus"].ID, participant.ID, base.Add(-time.Hour))
	}
	createWordleMembership(t, ctx, queries, groups["leaf"].ID, participants["charlie"].ID, base.Add(-time.Hour))
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create instance admin: %v", err)
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: otherInstance.ID, DiscordUserID: "admin-discord"}); err != nil {
		t.Fatalf("create other instance admin: %v", err)
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: otherInstance.ID, DiscordUserID: "other-admin"}); err != nil {
		t.Fatalf("create second other instance admin: %v", err)
	}
	wrongActivity := createActivityForTest(t, ctx, queries, instance.ID, base.Add(-time.Hour), nil, "journey", "Not Wordle")
	managedActivity := createActivityForTest(t, ctx, queries, otherInstance.ID, base.Add(-time.Hour), nil, wordleActivityType, "Managed Wordle")
	if err := queries.SetInstanceProgressionMode(ctx, db.SetInstanceProgressionModeParams{InstanceID: otherInstance.ID, ProgressionMode: "managed"}); err != nil {
		t.Fatalf("set managed instance: %v", err)
	}

	server := httpapi.New(pool,
		httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"verification-token"}}),
		httpapi.WithClock(clock.Now),
	)
	router := server.Router()
	opensAt := base.Add(123456 * time.Nanosecond)
	cutoffAt := base.Add(time.Hour + 654321*time.Nanosecond)
	endedGroup := createParticipantGroupForTest(t, ctx, queries, instance.ID, "Ended", "tribe")
	startsAtCutoffGroup := createParticipantGroupForTest(t, ctx, queries, instance.ID, "Starts At Cutoff", "tribe")
	if _, err := queries.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{
		ParticipantGroupID: endedGroup.ID,
		ParticipantID:      participants["alice"].ID,
		Role:               "member",
		StartsAt:           wordleTimestamp(base.Add(-time.Hour)),
		EndsAt:             wordleTimestamp(cutoffAt),
		Metadata:           []byte(`{}`),
	}); err != nil {
		t.Fatalf("create ended Wordle membership: %v", err)
	}
	if _, err := queries.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{
		ParticipantGroupID: startsAtCutoffGroup.ID,
		ParticipantID:      participants["alice"].ID,
		Role:               "member",
		StartsAt:           wordleTimestamp(cutoffAt),
		Metadata:           []byte(`{}`),
	}); err != nil {
		t.Fatalf("create cutoff-start Wordle membership: %v", err)
	}
	createBody := fmt.Sprintf(`{"round_key":"week-1","name":"Weekly Wordle","opens_at":"%s","cutoff_at":"%s"}`, opensAt.Format(time.RFC3339Nano), cutoffAt.Format(time.RFC3339Nano))

	disabledRouter := httpapi.New(pool, httpapi.WithClock(clock.Now)).Router()
	response := wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(wrongActivity.ID.Bytes)), createBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusBadRequest)
	managedBody := fmt.Sprintf(`{"round_key":"managed-1","name":"Managed Wordle","opens_at":"%s","cutoff_at":"%s"}`, opensAt.Format(time.RFC3339Nano), cutoffAt.Format(time.RFC3339Nano))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(managedActivity.ID.Bytes)), managedBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)

	response = wordleServe(disabledRouter, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), createBody, "forged-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusUnauthorized)

	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), createBody, "verification-token", "not-an-admin")
	wordleRequireStatus(t, response, http.StatusForbidden)

	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), createBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusCreated)
	roundID := wordleRoundID(t, response)
	var createdRound struct {
		Round struct {
			OpensAt string `json:"opens_at"`
		} `json:"round"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &createdRound); err != nil {
		t.Fatalf("unmarshal created fractional Wordle round: %v", err)
	}
	if createdRound.Round.OpensAt != opensAt.Truncate(time.Microsecond).Format(time.RFC3339Nano) {
		t.Fatalf("fractional opens_at = %s, want %s", createdRound.Round.OpensAt, opensAt.Truncate(time.Microsecond).Format(time.RFC3339Nano))
	}
	response = wordleServe(disabledRouter, http.MethodGet, fmt.Sprintf("/wordle-rounds/%s", roundID), "", "forged-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusUnauthorized)

	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), createBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	if got := wordleRoundID(t, response); got != roundID {
		t.Fatalf("retry round id = %s, want %s", got, roundID)
	}
	concurrentCreateBody := fmt.Sprintf(`{"round_key":"week-concurrent","name":"Concurrent Wordle","opens_at":"%s","cutoff_at":"%s"}`, opensAt.Format(time.RFC3339Nano), cutoffAt.Format(time.RFC3339Nano))
	createResults := make(chan *httptest.ResponseRecorder, 2)
	var createWait sync.WaitGroup
	for range 2 {
		createWait.Add(1)
		go func() {
			defer createWait.Done()
			createResults <- wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), concurrentCreateBody, "verification-token", "admin-discord")
		}()
	}
	createWait.Wait()
	firstCreate := <-createResults
	secondCreate := <-createResults
	if !((firstCreate.Code == http.StatusCreated && secondCreate.Code == http.StatusOK) || (firstCreate.Code == http.StatusOK && secondCreate.Code == http.StatusCreated)) {
		t.Fatalf("concurrent create statuses = %d and %d", firstCreate.Code, secondCreate.Code)
	}
	if wordleRoundID(t, firstCreate) != wordleRoundID(t, secondCreate) {
		t.Fatalf("concurrent create returned different round ids")
	}

	conflictingCreate := fmt.Sprintf(`{"round_key":"week-1","name":"Different Name","opens_at":"%s","cutoff_at":"%s"}`, opensAt.Format(time.RFC3339Nano), cutoffAt.Format(time.RFC3339Nano))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), conflictingCreate, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)

	tooLongKey := fmt.Sprintf(`{"round_key":"%s","name":"Too Long","opens_at":"%s","cutoff_at":"%s"}`, string(bytes.Repeat([]byte("x"), wordleRoundKeyLimit+1)), opensAt.Format(time.RFC3339Nano), cutoffAt.Format(time.RFC3339Nano))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activity.ID.Bytes)), tooLongKey, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusBadRequest)

	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)

	putRound := func(targetRoundID string, participantID pgtype.UUID, groupID pgtype.UUID, guessCount int32, token, discordUserID string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"participant_group_id":"%s","guess_count":%d}`, uuid.UUID(groupID.Bytes), guessCount)
		path := fmt.Sprintf("/wordle-rounds/%s/participants/%s", targetRoundID, uuid.UUID(participantID.Bytes))
		return wordleServe(router, http.MethodPut, path, body, token, discordUserID)
	}
	put := func(participantID pgtype.UUID, groupID pgtype.UUID, guessCount int32) *httptest.ResponseRecorder {
		return putRound(roundID, participantID, groupID, guessCount, "verification-token", "admin-discord")
	}

	response = put(participants["alice"].ID, groups["lotus"].ID, 3)
	wordleRequireStatus(t, response, http.StatusConflict)
	clock.Set(opensAt.Truncate(time.Microsecond))
	response = putRound(roundID, participants["alice"].ID, groups["lotus"].ID, 3, "verification-token", "not-an-admin")
	wordleRequireStatus(t, response, http.StatusForbidden)
	response = put(participants["alice"].ID, groups["invalid"].ID, 3)
	wordleRequireStatus(t, response, http.StatusBadRequest)
	response = put(participants["alice"].ID, groups["other"].ID, 3)
	wordleRequireStatus(t, response, http.StatusBadRequest)
	response = put(participants["other"].ID, groups["lotus"].ID, 3)
	wordleRequireStatus(t, response, http.StatusBadRequest)
	response = put(participants["outsider"].ID, groups["lotus"].ID, 3)
	wordleRequireStatus(t, response, http.StatusBadRequest)
	response = put(participants["alice"].ID, endedGroup.ID, 3)
	wordleRequireStatus(t, response, http.StatusBadRequest)
	response = put(participants["alice"].ID, startsAtCutoffGroup.ID, 3)
	wordleRequireStatus(t, response, http.StatusOK)
	genericParticipantBody := fmt.Sprintf(`{"participant_id":"%s","participant_group_id":"%s","role":"participant","metadata":{"guess_count":1}}`, uuid.UUID(participants["alice"].ID.Bytes), uuid.UUID(groups["lotus"].ID.Bytes))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/occurrences/%s/participants", roundID), genericParticipantBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	response = put(participants["alice"].ID, groups["lotus"].ID, 4)
	wordleRequireStatus(t, response, http.StatusOK)
	response = put(participants["alice"].ID, groups["lotus"].ID, 2)
	wordleRequireStatus(t, response, http.StatusOK)

	clock.Set(base.Add(30 * time.Minute))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	response = put(participants["alice"].ID, groups["lotus"].ID, 3)
	wordleRequireStatus(t, response, http.StatusOK)
	response = put(participants["bob"].ID, groups["lotus"].ID, 1)
	wordleRequireStatus(t, response, http.StatusOK)
	response = put(participants["charlie"].ID, groups["leaf"].ID, 2)
	wordleRequireStatus(t, response, http.StatusOK)

	participantsInRound, err := queries.ListActivityOccurrenceParticipants(ctx, wordlePGUUID(uuid.MustParse(roundID)))
	if err != nil {
		t.Fatalf("list wordle participants: %v", err)
	}
	if len(participantsInRound) != 3 {
		t.Fatalf("wordle participant count = %d, want 3", len(participantsInRound))
	}
	for _, participant := range participantsInRound {
		if participant.ParticipantID == participants["alice"].ID {
			var metadata struct {
				GuessCount int `json:"guess_count"`
			}
			if err := json.Unmarshal(participant.Metadata, &metadata); err != nil {
				t.Fatalf("unmarshal Alice metadata: %v", err)
			}
			if metadata.GuessCount != 3 {
				t.Fatalf("Alice guess count = %d, want 3", metadata.GuessCount)
			}
		}
	}

	clock.Set(cutoffAt)
	response = put(participants["alice"].ID, groups["lotus"].ID, 1)
	wordleRequireStatus(t, response, http.StatusConflict)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	closedBody := append([]byte(nil), response.Body.Bytes()...)
	clock.Set(cutoffAt.Add(time.Minute))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	if !bytes.Equal(closedBody, response.Body.Bytes()) {
		t.Fatalf("idempotent close response changed: first=%s second=%s", closedBody, response.Body.Bytes())
	}
	response = put(participants["alice"].ID, groups["lotus"].ID, 1)
	wordleRequireStatus(t, response, http.StatusConflict)

	genericParticipantBody = fmt.Sprintf(`{"participant_id":"%s","participant_group_id":"%s","role":"participant","metadata":{"guess_count":1}}`, uuid.UUID(participants["alice"].ID.Bytes), uuid.UUID(groups["lotus"].ID.Bytes))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/occurrences/%s/participants", roundID), genericParticipantBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	genericGroupBody := fmt.Sprintf(`{"participant_group_id":"%s","role":"tribe"}`, uuid.UUID(groups["lotus"].ID.Bytes))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/occurrences/%s/groups", roundID), genericGroupBody, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/occurrences/%s/resolve", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)

	secretBefore := wordleSecretBalances(t, ctx, queries, instance.ID, participants)
	results := make(chan *httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", roundID), "", "verification-token", "admin-discord")
		}()
	}
	wait.Wait()
	firstResult := <-results
	wordleRequireStatus(t, firstResult, http.StatusOK)
	firstResolution := append([]byte(nil), firstResult.Body.Bytes()...)
	for range 1 {
		result := <-results
		wordleRequireStatus(t, result, http.StatusOK)
		if !bytes.Equal(firstResolution, result.Body.Bytes()) {
			t.Fatalf("concurrent resolution changed response: %s", result.Body.Bytes())
		}
	}
	var resolution struct {
		CreatedEntries []struct {
			ParticipantID string `json:"participant_id"`
			Points        int32  `json:"points"`
			Visibility    string `json:"visibility"`
		} `json:"created_entries"`
		CreatedCount int `json:"created_count"`
	}
	if err := json.Unmarshal(firstResolution, &resolution); err != nil {
		t.Fatalf("unmarshal wordle resolution: %v", err)
	}
	if resolution.CreatedCount != 4 || len(resolution.CreatedEntries) != 4 {
		t.Fatalf("wordle resolution entries = %+v, want four public entries", resolution)
	}
	seen := map[string]bool{}
	for _, entry := range resolution.CreatedEntries {
		if entry.Points != 1 || entry.Visibility != "public" || seen[entry.ParticipantID] {
			t.Fatalf("unexpected wordle award entries: %+v", resolution.CreatedEntries)
		}
		seen[entry.ParticipantID] = true
	}
	for _, winner := range []string{"alice", "bob", "charlie", "dana"} {
		if !seen[uuid.UUID(participants[winner].ID.Bytes).String()] {
			t.Fatalf("expected %s to receive the tied public award, got %+v", winner, resolution.CreatedEntries)
		}
	}
	if got := wordleSecretBalances(t, ctx, queries, instance.ID, participants); !equalWordleBalances(secretBefore, got) {
		t.Fatalf("secret balances changed: before=%v after=%v", secretBefore, got)
	}
	ledger, err := queries.ListVisibleBonusPointLedgerEntriesByOccurrence(ctx, wordlePGUUID(uuid.MustParse(roundID)))
	if err != nil {
		t.Fatalf("list wordle ledger: %v", err)
	}
	if len(ledger) != 4 {
		t.Fatalf("visible Wordle ledger count = %d, want 4", len(ledger))
	}

	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	if !bytes.Equal(firstResolution, response.Body.Bytes()) {
		t.Fatalf("resolution retry changed response: first=%s second=%s", firstResolution, response.Body.Bytes())
	}
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/occurrences/%s/resolve", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	if ledger, err := queries.ListVisibleBonusPointLedgerEntriesByOccurrence(ctx, wordlePGUUID(uuid.MustParse(roundID))); err != nil || len(ledger) != 4 {
		t.Fatalf("concurrent resolution ledger = %d, err=%v, want 4", len(ledger), err)
	}

	response = wordleServe(router, http.MethodGet, fmt.Sprintf("/wordle-rounds/%s", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	var inspection struct {
		Round struct {
			Resolved bool `json:"resolved"`
		} `json:"round"`
		Participants []json.RawMessage `json:"participants"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &inspection); err != nil {
		t.Fatalf("unmarshal Wordle inspection: %v", err)
	}
	if !inspection.Round.Resolved || len(inspection.Participants) != 3 {
		t.Fatalf("unexpected Wordle inspection: %+v", inspection)
	}

	earlyStart := base.Add(2 * time.Hour)
	earlyRoundID := createWordleRoundOnly(t, router, clock, activity.ID, earlyStart)
	clock.Set(earlyStart)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", earlyRoundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	response = putRound(earlyRoundID, participants["alice"].ID, groups["lotus"].ID, 2, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", earlyRoundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
	response = wordleServe(disabledRouter, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", earlyRoundID), "", "forged-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusUnauthorized)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", earlyRoundID), "", "verification-token", "not-an-admin")
	wordleRequireStatus(t, response, http.StatusForbidden)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", earlyRoundID), "", "verification-token", "other-admin")
	wordleRequireStatus(t, response, http.StatusForbidden)

	rollbackStart := base.Add(3 * time.Hour)
	rollbackRoundID := createAndSubmitWordleRound(t, router, clock, activity.ID, participants["alice"].ID, groups["lotus"].ID, participants["bob"].ID, rollbackStart)
	rollbackRoundKey := fmt.Sprintf("round-%d", rollbackStart.Hour())
	constraintSQL := fmt.Sprintf("ALTER TABLE wordle_rounds ADD CONSTRAINT wordle_test_reject_resolution CHECK (round_key <> '%s' OR resolution_response IS NULL)", rollbackRoundKey)
	if _, err := pool.Exec(ctx, constraintSQL); err != nil {
		t.Fatalf("add rollback constraint: %v", err)
	}
	constraintActive := true
	defer func() {
		if constraintActive {
			if _, err := pool.Exec(context.Background(), "ALTER TABLE wordle_rounds DROP CONSTRAINT wordle_test_reject_resolution"); err != nil {
				t.Errorf("drop Wordle test constraint: %v", err)
			}
		}
	}()
	rollbackSecrets := wordleSecretBalances(t, ctx, queries, instance.ID, participants)
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", rollbackRoundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusBadRequest)
	if _, err := pool.Exec(ctx, "ALTER TABLE wordle_rounds DROP CONSTRAINT wordle_test_reject_resolution"); err != nil {
		t.Fatalf("drop rollback constraint: %v", err)
	}
	constraintActive = false
	failedRound, err := queries.GetWordleRound(ctx, wordlePGUUID(uuid.MustParse(rollbackRoundID)))
	if err != nil {
		t.Fatalf("get rolled back Wordle round: %v", err)
	}
	if failedRound.OccurrenceStatus != "recorded" || len(failedRound.ResolutionResponse) != 0 {
		t.Fatalf("Wordle resolution failure partially committed: %+v", failedRound)
	}
	if ledger, err := queries.ListVisibleBonusPointLedgerEntriesByOccurrence(ctx, wordlePGUUID(uuid.MustParse(rollbackRoundID))); err != nil || len(ledger) != 0 {
		t.Fatalf("rolled back Wordle ledger = %d, err=%v, want 0", len(ledger), err)
	}
	if got := wordleSecretBalances(t, ctx, queries, instance.ID, participants); !equalWordleBalances(rollbackSecrets, got) {
		t.Fatalf("secret balances changed after rollback: before=%v after=%v", rollbackSecrets, got)
	}
	failedOccurrence, err := queries.GetActivityOccurrence(ctx, wordlePGUUID(uuid.MustParse(rollbackRoundID)))
	if err != nil {
		t.Fatalf("get rolled back Wordle occurrence: %v", err)
	}
	if failedOccurrence.Status != "recorded" || failedOccurrence.EndsAt.Valid {
		t.Fatalf("Wordle occurrence changed despite rollback: %+v", failedOccurrence)
	}
	clock.Set(rollbackStart.Add(time.Hour))
	response = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/resolve", rollbackRoundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	var recoveredResolution struct {
		CreatedCount int `json:"created_count"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &recoveredResolution); err != nil {
		t.Fatalf("unmarshal recovered Wordle resolution: %v", err)
	}
	if recoveredResolution.CreatedCount != 3 {
		t.Fatalf("recovered Wordle resolution count = %d, want 3", recoveredResolution.CreatedCount)
	}
	rollbackOccurrence, err := queries.GetActivityOccurrence(ctx, wordlePGUUID(uuid.MustParse(rollbackRoundID)))
	if err != nil {
		t.Fatalf("get recovered Wordle occurrence: %v", err)
	}
	if !rollbackOccurrence.EndsAt.Valid {
		t.Fatalf("recovered Wordle occurrence should have a resolver end timestamp")
	}

	lockedStart := base.Add(7 * time.Hour)
	lockedRoundID := createWordleRoundOnly(t, router, clock, activity.ID, lockedStart)
	clock.Set(lockedStart)
	holdTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin Wordle lock test transaction: %v", err)
	}
	holdQueries := db.New(holdTx)
	if _, err := holdQueries.LockInstanceForProgression(ctx, instance.ID); err != nil {
		t.Fatalf("lock Wordle test instance: %v", err)
	}
	if _, err := holdQueries.LockWordleRound(ctx, wordlePGUUID(uuid.MustParse(lockedRoundID))); err != nil {
		t.Fatalf("lock Wordle test round: %v", err)
	}
	lockedResponseCh := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		lockedResponseCh <- putRound(lockedRoundID, participants["alice"].ID, groups["lotus"].ID, 2, "verification-token", "admin-discord")
	}()
	time.Sleep(50 * time.Millisecond)
	clock.Set(lockedStart.Add(time.Hour))
	if err := holdTx.Commit(ctx); err != nil {
		t.Fatalf("commit Wordle lock test transaction: %v", err)
	}
	lockedResponse := <-lockedResponseCh
	wordleRequireStatus(t, lockedResponse, http.StatusConflict)
	lockedParticipants, err := queries.ListActivityOccurrenceParticipants(ctx, wordlePGUUID(uuid.MustParse(lockedRoundID)))
	if err != nil {
		t.Fatalf("list locked Wordle submissions: %v", err)
	}
	if len(lockedParticipants) != 0 {
		t.Fatalf("locked cutoff submission wrote %d participants", len(lockedParticipants))
	}

	concurrentRoundID := createWordleRoundOnly(t, router, clock, activity.ID, base.Add(5*time.Hour))
	clock.Set(base.Add(5*time.Hour + 30*time.Minute))
	var closeResponse, submissionResponse *httptest.ResponseRecorder
	wait = sync.WaitGroup{}
	wait.Add(2)
	go func() {
		defer wait.Done()
		closeResponse = wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", concurrentRoundID), "", "verification-token", "admin-discord")
	}()
	go func() {
		defer wait.Done()
		submissionResponse = putRound(concurrentRoundID, participants["alice"].ID, groups["lotus"].ID, 2, "verification-token", "admin-discord")
	}()
	wait.Wait()
	wordleRequireStatus(t, closeResponse, http.StatusOK)
	wantParticipants := 0
	switch submissionResponse.Code {
	case http.StatusOK:
		wantParticipants = 1
	case http.StatusConflict:
	default:
		t.Fatalf("concurrent submission status = %d: %s", submissionResponse.Code, submissionResponse.Body.String())
	}
	concurrentParticipants, err := queries.ListActivityOccurrenceParticipants(ctx, wordlePGUUID(uuid.MustParse(concurrentRoundID)))
	if err != nil {
		t.Fatalf("list serialized Wordle submissions: %v", err)
	}
	if len(concurrentParticipants) != wantParticipants {
		t.Fatalf("serialized submission wrote %d participants, want %d", len(concurrentParticipants), wantParticipants)
	}
	closedRound, err := queries.GetWordleRound(ctx, wordlePGUUID(uuid.MustParse(concurrentRoundID)))
	if err != nil || !closedRound.ClosedAt.Valid {
		t.Fatalf("concurrent round not closed: %v", err)
	}
	response = putRound(concurrentRoundID, participants["alice"].ID, groups["lotus"].ID, 3, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusConflict)
}

func createWordleMembership(t *testing.T, ctx context.Context, queries *db.Queries, groupID, participantID pgtype.UUID, startsAt time.Time) {
	t.Helper()
	if _, err := queries.CreateParticipantGroupMembershipPeriod(ctx, db.CreateParticipantGroupMembershipPeriodParams{
		ParticipantGroupID: groupID,
		ParticipantID:      participantID,
		Role:               "member",
		StartsAt:           wordleTimestamp(startsAt),
		Metadata:           []byte(`{}`),
	}); err != nil {
		t.Fatalf("create Wordle membership: %v", err)
	}
}

func wordleServe(router http.Handler, method, path, body, token, discordUserID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, wordleAuthorizedJSONRequest(method, path, body, token, discordUserID))
	return recorder
}

func wordleRequireStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response == nil || response.Code != want {
		if response == nil {
			t.Fatalf("response = nil, want status %d", want)
		}
		t.Fatalf("status = %d, want %d, body = %s", response.Code, want, response.Body.String())
	}
}

func wordleRoundID(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Round struct {
			ID string `json:"id"`
		} `json:"round"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal Wordle round response: %v", err)
	}
	if _, err := uuid.Parse(payload.Round.ID); err != nil {
		t.Fatalf("Wordle round id = %q: %v", payload.Round.ID, err)
	}
	return payload.Round.ID
}

func wordleSecretBalances(t *testing.T, ctx context.Context, queries *db.Queries, instanceID pgtype.UUID, participants map[string]db.CreateParticipantRow) map[string]int32 {
	t.Helper()
	balances := make(map[string]int32, len(participants))
	for name, participant := range participants {
		balance, err := queries.GetSecretBonusTotalByParticipant(ctx, db.GetSecretBonusTotalByParticipantParams{
			InstanceID:    instanceID,
			ParticipantID: participant.ID,
		})
		if err != nil {
			t.Fatalf("get secret balance for %s: %v", name, err)
		}
		balances[name] = balance
	}
	return balances
}

func equalWordleBalances(left, right map[string]int32) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func createWordleRoundOnly(t *testing.T, router http.Handler, clock *wordleFixtureClock, activityID pgtype.UUID, start time.Time) string {
	t.Helper()
	clock.Set(start.Add(-time.Minute))
	cutoff := start.Add(time.Hour)
	body := fmt.Sprintf(`{"round_key":"round-%d","name":"Wordle %s","opens_at":"%s","cutoff_at":"%s"}`, start.Hour(), start.Format(time.RFC3339), start.Format(time.RFC3339), cutoff.Format(time.RFC3339))
	response := wordleServe(router, http.MethodPost, fmt.Sprintf("/activities/%s/wordle-rounds", uuid.UUID(activityID.Bytes)), body, "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusCreated)
	return wordleRoundID(t, response)
}

func createAndSubmitWordleRound(t *testing.T, router http.Handler, clock *wordleFixtureClock, activityID, firstParticipantID, groupID, secondParticipantID pgtype.UUID, start time.Time) string {
	t.Helper()
	roundID := createWordleRoundOnly(t, router, clock, activityID, start)
	clock.Set(start)
	put := func(participantID pgtype.UUID, guessCount int) {
		body := fmt.Sprintf(`{"participant_group_id":"%s","guess_count":%d}`, uuid.UUID(groupID.Bytes), guessCount)
		response := wordleServe(router, http.MethodPut, fmt.Sprintf("/wordle-rounds/%s/participants/%s", roundID, uuid.UUID(participantID.Bytes)), body, "verification-token", "admin-discord")
		wordleRequireStatus(t, response, http.StatusOK)
	}
	put(firstParticipantID, 2)
	put(secondParticipantID, 1)
	clock.Set(start.Add(time.Hour))
	response := wordleServe(router, http.MethodPost, fmt.Sprintf("/wordle-rounds/%s/close", roundID), "", "verification-token", "admin-discord")
	wordleRequireStatus(t, response, http.StatusOK)
	return roundID
}

func wordleTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func wordlePGUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func wordleAuthorizedJSONRequest(method, path, body, bearerToken, discordUserID string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if discordUserID != "" {
		request.Header.Set("X-Discord-User-ID", discordUserID)
	}
	if method != http.MethodGet && method != http.MethodDelete {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}
