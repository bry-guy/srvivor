package httpapi_test

import (
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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type managedProgressionFixture struct {
	router     http.Handler
	queries    *db.Queries
	instanceID string
	instance   db.GetInstanceRow
}

func newManagedProgressionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string, season int32, contestantNames []string, episodes int, clocks ...time.Time) managedProgressionFixture {
	t.Helper()
	payloadEpisodes := make([]map[string]any, 0, episodes)
	start := time.Date(2026, time.January, 1, 20, 0, 0, 0, time.FixedZone("EST", -5*60*60))
	for number := 0; number < episodes; number++ {
		payloadEpisodes = append(payloadEpisodes, map[string]any{
			"episode_number": number,
			"label":          fmt.Sprintf("Episode %d", number),
			"airs_at":        start.AddDate(0, 0, number*7).Format(time.RFC3339),
		})
	}
	body, err := json.Marshal(map[string]any{
		"name":                name,
		"season":              season,
		"managed_progression": true,
		"contestants":         contestantNames,
		"episodes":            payloadEpisodes,
	})
	if err != nil {
		t.Fatalf("marshal managed fixture: %v", err)
	}
	options := []httpapi.Option{httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      true,
		BearerTokens: []string{"progression-token"},
	})}
	if len(clocks) > 0 {
		clock := clocks[0]
		options = append(options, httpapi.WithClock(func() time.Time { return clock }))
	}
	server := httpapi.New(pool, options...)
	req := authorizedJSONRequest(http.MethodPost, "/instances", string(body), "progression-token", "progression-admin")
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create managed fixture status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode managed fixture: %v", err)
	}
	publicID, err := uuid.Parse(response.Instance.ID)
	if err != nil {
		t.Fatalf("parse managed fixture id: %v", err)
	}
	queries := db.New(pool)
	instance, err := queries.GetInstance(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
	if err != nil {
		t.Fatalf("load managed fixture: %v", err)
	}
	return managedProgressionFixture{
		router:     server.Router(),
		queries:    queries,
		instanceID: response.Instance.ID,
		instance:   instance,
	}
}

func progressionCommand(key, effectiveAt string) string {
	return fmt.Sprintf(`{"idempotency_key":%q,"effective_at":%q}`, key, effectiveAt)
}

func serve(f managedProgressionFixture, method, path, body, token, discordUserID string) *httptest.ResponseRecorder {
	req := authorizedJSONRequest(method, path, body, token, discordUserID)
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	return recorder
}

func TestManagedProgressionCommandsSerializeAndRejectBackdating(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Progression concurrency", 101, []string{"Concurrency C1", "Concurrency C2"}, 3)
	path := "/instances/" + fixture.instanceID

	var wait sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("same-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin")
		}()
	}
	wait.Wait()
	close(results)
	for result := range results {
		if result.Code != http.StatusOK {
			t.Fatalf("identical concurrent command status = %d, body = %s", result.Code, result.Body.String())
		}
	}
	var commandCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandCount); err != nil {
		t.Fatalf("count progression commands: %v", err)
	}
	if commandCount != 1 {
		t.Fatalf("expected one persisted command for identical retries, got %d", commandCount)
	}
	conflict := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("same-open", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin")
	if conflict.Code != http.StatusConflict || conflict.Body.String() == "" {
		t.Fatalf("conflicting retry status = %d, body = %s", conflict.Code, conflict.Body.String())
	}
	draft, err := fixture.queries.GetInstanceDraftProgress(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read draft after conflicting retry: %v", err)
	}
	if draft.Status != "open" {
		t.Fatalf("conflicting retry changed draft state to %q", draft.Status)
	}

	backdatedClose := serve(fixture, http.MethodPost, path+"/progression/draft/close", progressionCommand("close-backdated", "2026-01-01T20:00:00Z"), "progression-token", "progression-admin")
	if backdatedClose.Code != http.StatusConflict {
		t.Fatalf("backdated close status = %d, body = %s", backdatedClose.Code, backdatedClose.Body.String())
	}
	closeDraft := serve(fixture, http.MethodPost, path+"/progression/draft/close", progressionCommand("close-valid", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin")
	if closeDraft.Code != http.StatusOK {
		t.Fatalf("valid close status = %d, body = %s", closeDraft.Code, closeDraft.Body.String())
	}
	start := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("episode-1-start", "2026-01-04T20:00:00Z"), "progression-token", "progression-admin")
	if start.Code != http.StatusOK {
		t.Fatalf("episode start status = %d, body = %s", start.Code, start.Body.String())
	}
	backdatedComplete := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("complete-backdated", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin")
	if backdatedComplete.Code != http.StatusConflict {
		t.Fatalf("backdated complete status = %d, body = %s", backdatedComplete.Code, backdatedComplete.Body.String())
	}
	complete := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("complete-valid", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin")
	if complete.Code != http.StatusOK {
		t.Fatalf("valid complete status = %d, body = %s", complete.Code, complete.Body.String())
	}
	backdatedScore := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("score-backdated", "2026-01-04T20:00:00Z"), "progression-token", "progression-admin")
	if backdatedScore.Code != http.StatusConflict {
		t.Fatalf("backdated score status = %d, body = %s", backdatedScore.Code, backdatedScore.Body.String())
	}
	results = make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("same-score", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin")
		}()
	}
	wait.Wait()
	close(results)
	for result := range results {
		if result.Code != http.StatusOK {
			t.Fatalf("identical concurrent score status = %d, body = %s", result.Code, result.Body.String())
		}
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandCount); err != nil {
		t.Fatalf("count commands after score retries: %v", err)
	}
	if commandCount != 5 {
		t.Fatalf("expected one additional score command after four earlier commands, got %d", commandCount)
	}
	var revisionCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisionCount); err != nil {
		t.Fatalf("count score revisions after retries: %v", err)
	}
	if revisionCount != 1 {
		t.Fatalf("expected one score publication after retries, got %d", revisionCount)
	}
	scoreConflict := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("same-score", "2026-01-07T20:00:00Z"), "progression-token", "progression-admin")
	if scoreConflict.Code != http.StatusConflict || !strings.Contains(scoreConflict.Body.String(), "idempotency key was already used with a different payload") {
		t.Fatalf("specific score idempotency conflict = %d, body = %s", scoreConflict.Code, scoreConflict.Body.String())
	}
}

func TestManagedConcurrentConflictingCommandsHaveOneEffect(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Conflicting commands", 110, []string{"Conflict C1"}, 2)
	path := "/instances/" + fixture.instanceID
	if open := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("conflict-open", "2026-01-02T19:00:00Z"), "progression-token", "progression-admin"); open.Code != http.StatusOK {
		t.Fatalf("open conflicting-command draft status = %d, body = %s", open.Code, open.Body.String())
	}
	responses := make(chan *httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for _, effectiveAt := range []string{"2026-01-02T20:00:00Z", "2026-01-03T20:00:00Z"} {
		effectiveAt := effectiveAt
		go func() {
			defer wait.Done()
			responses <- serve(fixture, http.MethodPost, path+"/progression/draft/close", progressionCommand("same-close", effectiveAt), "progression-token", "progression-admin")
		}()
	}
	wait.Wait()
	close(responses)
	successes, conflicts := 0, 0
	for response := range responses {
		switch response.Code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
			if !strings.Contains(response.Body.String(), "idempotency key was already used with a different payload") {
				t.Fatalf("unexpected concurrent conflict body: %s", response.Body.String())
			}
		default:
			t.Fatalf("unexpected concurrent conflict status = %d, body = %s", response.Code, response.Body.String())
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one concurrent success and one payload conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
	var commands int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commands); err != nil {
		t.Fatalf("count conflicting commands: %v", err)
	}
	if commands != 2 {
		t.Fatalf("expected one open and one close command row after conflicting retries, got %d", commands)
	}
}

func TestManagedLateDraftBeforePublicationAndGuardedPaths(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Late draft boundary", 102, []string{"Late C1", "Late C2"}, 2)
	path := "/instances/" + fixture.instanceID
	participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Late Alice")
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list late draft contestants: %v", err)
	}
	order := []string{uuid.UUID(contestants[0].ID.Bytes).String(), uuid.UUID(contestants[1].ID.Bytes).String()}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("open-late", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("open late draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("start-late", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start late episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	draftBody := func(key, effectiveAt string) string {
		return fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":%q,"effective_at":%q}`, order[0], order[1], key, effectiveAt)
	}
	participantPath := path + "/drafts/" + uuid.UUID(participant.ID.Bytes).String()
	if recorder := serve(fixture, http.MethodPut, participantPath, draftBody("late-initial", "2026-01-03T20:01:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("initial late draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/close", progressionCommand("close-late", "2026-01-04T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("close late draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	backdatedLate := serve(fixture, http.MethodPost, participantPath+"/late", draftBody("late-backdated", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin")
	if backdatedLate.Code != http.StatusConflict {
		t.Fatalf("backdated late draft status = %d, body = %s", backdatedLate.Code, backdatedLate.Body.String())
	}
	validLate := serve(fixture, http.MethodPost, participantPath+"/late", draftBody("late-valid", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin")
	if validLate.Code != http.StatusOK {
		t.Fatalf("late draft before first publication status = %d, body = %s", validLate.Code, validLate.Body.String())
	}
	var revisions int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisions); err != nil {
		t.Fatalf("count prepublication revisions: %v", err)
	}
	if revisions != 0 {
		t.Fatalf("late draft before first publication created %d revisions", revisions)
	}

	noAdmin := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("no-admin", "2026-01-06T20:00:00Z"), "progression-token", "not-admin")
	if noAdmin.Code != http.StatusForbidden {
		t.Fatalf("non-admin progression status = %d, body = %s", noAdmin.Code, noAdmin.Body.String())
	}
	guardedStir := serve(fixture, http.MethodPost, path+"/stir-the-pot/start", `{"name":"unsupported"}`, "progression-token", "progression-admin")
	if guardedStir.Code != http.StatusConflict {
		t.Fatalf("managed Stir the Pot bypass status = %d, body = %s", guardedStir.Code, guardedStir.Body.String())
	}
	activity := createActivityForTest(t, ctx, fixture.queries, fixture.instance.ID, time.Date(2026, time.January, 8, 20, 0, 0, 0, time.UTC), nil, "manual_adjustment", "blocked activity")
	occurrence := createOccurrenceForTest(t, ctx, fixture.queries, activity.ID, "blocked", "blocked occurrence", time.Date(2026, time.January, 8, 20, 0, 0, 0, time.UTC))
	guardedWrites := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/activities/" + uuid.UUID(activity.ID.Bytes).String() + "/occurrences", `{}`},
		{http.MethodPost, "/occurrences/" + uuid.UUID(occurrence.ID.Bytes).String() + "/participants", `{}`},
		{http.MethodPost, "/occurrences/" + uuid.UUID(occurrence.ID.Bytes).String() + "/groups", `{}`},
		{http.MethodPost, "/occurrences/" + uuid.UUID(occurrence.ID.Bytes).String() + "/resolve", `{}`},
		{http.MethodPost, path + "/auction/lots/start", `{}`},
		{http.MethodPut, path + "/auction/contestants/" + uuid.NewString() + "/bid/me", `{}`},
		{http.MethodPost, path + "/auction/lots/" + uuid.NewString() + "/stop", `{}`},
		{http.MethodPost, path + "/loan-shark/me/borrow", `{}`},
		{http.MethodPost, path + "/loan-shark/me/repay", `{}`},
		{http.MethodPost, path + "/individual-pony/immunity", `{}`},
		{http.MethodPost, path + "/merge-auction/record", `{}`},
		{http.MethodPost, path + "/finale-bingo/loan-sharks", `{}`},
		{http.MethodPost, path + "/finale-bingo/scores/preview", `{}`},
		{http.MethodPost, path + "/finale-bingo/scores", `{}`},
	}
	for _, guarded := range guardedWrites {
		recorder := serve(fixture, guarded.method, guarded.path, guarded.body, "progression-token", "progression-admin")
		if recorder.Code != http.StatusConflict {
			t.Fatalf("managed %s %s bypass status = %d, body = %s", guarded.method, guarded.path, recorder.Code, recorder.Body.String())
		}
	}

	other := newManagedProgressionFixture(t, ctx, pool, "Cross instance", 106, []string{"Cross C1", "Cross C2"}, 2)
	otherParticipant := createParticipantForTest(t, ctx, other.queries, other.instance.ID, "Cross Alice")
	otherContestants, err := other.queries.ListContestantsByInstance(ctx, other.instance.ID)
	if err != nil {
		t.Fatalf("list cross-instance contestants: %v", err)
	}
	crossBody := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"cross-instance","effective_at":"2026-01-07T20:00:00Z"}`, uuid.UUID(otherContestants[0].ID.Bytes).String(), uuid.UUID(otherContestants[1].ID.Bytes).String())
	cross := serve(fixture, http.MethodPost, path+"/drafts/"+uuid.UUID(otherParticipant.ID.Bytes).String()+"/late", crossBody, "progression-token", "progression-admin")
	if cross.Code != http.StatusBadRequest {
		t.Fatalf("cross-instance draft status = %d, body = %s", cross.Code, cross.Body.String())
	}
	crossDrafts, err := other.queries.ListDraftPicksForParticipant(ctx, otherParticipant.ID)
	if err != nil {
		t.Fatalf("read cross-instance draft: %v", err)
	}
	if len(crossDrafts) != 0 {
		t.Fatalf("cross-instance draft write created %d picks", len(crossDrafts))
	}
}

func TestManagedDraftCorrectionCannotPrecedeNextEpisodeCheckpoint(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Correction checkpoint", 107, []string{"Checkpoint C1", "Checkpoint C2"}, 3)
	path := "/instances/" + fixture.instanceID
	participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Checkpoint Alice")
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list checkpoint contestants: %v", err)
	}
	first := uuid.UUID(contestants[0].ID.Bytes).String()
	second := uuid.UUID(contestants[1].ID.Bytes).String()
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("checkpoint-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("open checkpoint draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("checkpoint-start-1", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start checkpoint episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	participantPath := path + "/drafts/" + uuid.UUID(participant.ID.Bytes).String()
	draft := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"checkpoint-draft","effective_at":"2026-01-03T20:01:00Z"}`, first, second)
	if recorder := serve(fixture, http.MethodPut, participantPath, draft, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save checkpoint draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	outcome := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"checkpoint-outcome","effective_at":"2026-01-04T20:00:00Z"}`, first)
	if recorder := serve(fixture, http.MethodPut, path+"/outcomes/1", outcome, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save checkpoint outcome status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("checkpoint-complete-1", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("complete checkpoint episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("checkpoint-score-1", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("score checkpoint episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/2/start", progressionCommand("checkpoint-start-2", "2026-01-08T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start next checkpoint episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	backdatedCorrection := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"checkpoint-backdated","effective_at":"2026-01-07T20:00:00Z"}`, second, first)
	if recorder := serve(fixture, http.MethodPut, participantPath, backdatedCorrection, "progression-token", "progression-admin"); recorder.Code != http.StatusConflict {
		t.Fatalf("backdated next-episode correction status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	validCorrection := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"checkpoint-valid","effective_at":"2026-01-08T20:30:00Z"}`, second, first)
	if recorder := serve(fixture, http.MethodPut, participantPath, validCorrection, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("valid next-episode correction status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestManagedDraftCorrectionRollsBackBeforePublication(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Rollback correction", 103, []string{"Rollback C1", "Rollback C2"}, 2)
	path := "/instances/" + fixture.instanceID
	participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Rollback Alice")
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list rollback contestants: %v", err)
	}
	first := uuid.UUID(contestants[0].ID.Bytes).String()
	second := uuid.UUID(contestants[1].ID.Bytes).String()
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("rollback-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("open rollback draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("rollback-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start rollback episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	participantPath := path + "/drafts/" + uuid.UUID(participant.ID.Bytes).String()
	draftBody := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"rollback-draft","effective_at":"2026-01-03T20:01:00Z"}`, first, second)
	if recorder := serve(fixture, http.MethodPut, participantPath, draftBody, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save rollback draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	outcomeBody := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"rollback-outcome","effective_at":"2026-01-04T20:00:00Z"}`, first)
	if recorder := serve(fixture, http.MethodPut, path+"/outcomes/1", outcomeBody, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save rollback outcome status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("rollback-complete", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("complete rollback episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("rollback-score", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("score rollback episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	before, err := fixture.queries.ListDraftPicksForParticipant(ctx, participant.ID)
	if err != nil {
		t.Fatalf("read draft before forced rollback: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE instance_score_revisions SET input_snapshot = '[]'::jsonb WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID); err != nil {
		t.Fatalf("corrupt disposable snapshot: %v", err)
	}
	correctionBody := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"rollback-correction","effective_at":"2026-01-07T20:00:00Z"}`, second, first)
	correction := serve(fixture, http.MethodPut, participantPath, correctionBody, "progression-token", "progression-admin")
	if correction.Code != http.StatusInternalServerError {
		t.Fatalf("forced rollback correction status = %d, body = %s", correction.Code, correction.Body.String())
	}
	after, err := fixture.queries.ListDraftPicksForParticipant(ctx, participant.ID)
	if err != nil {
		t.Fatalf("read draft after forced rollback: %v", err)
	}
	if len(after) != len(before) || after[0].ContestantID != before[0].ContestantID || after[1].ContestantID != before[1].ContestantID {
		t.Fatalf("draft mutation was not rolled back: before=%+v after=%+v", before, after)
	}
	var commandCount, revisionCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandCount); err != nil {
		t.Fatalf("count commands after forced rollback: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisionCount); err != nil {
		t.Fatalf("count revisions after forced rollback: %v", err)
	}
	if commandCount != 6 || revisionCount != 1 {
		t.Fatalf("forced rollback persisted command/revision state: commands=%d revisions=%d", commandCount, revisionCount)
	}
}

func TestManagedRosterSetupAndEpisodeStartSerialize(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Setup race", 108, []string{"Setup C1"}, 2)
	path := "/instances/" + fixture.instanceID
	statuses := make(chan int, 2)
	ready := make(chan struct{}, 2)
	begin := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		ready <- struct{}{}
		<-begin
		statuses <- serve(fixture, http.MethodPost, path+"/participants", `{"name":"Racing participant"}`, "progression-token", "progression-admin").Code
	}()
	go func() {
		defer wait.Done()
		ready <- struct{}{}
		<-begin
		statuses <- serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("setup-race-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin").Code
	}()
	<-ready
	<-ready
	close(begin)
	wait.Wait()
	close(statuses)
	gotStart, gotParticipant := false, false
	for status := range statuses {
		if status == http.StatusOK {
			gotStart = true
		}
		if status == http.StatusCreated {
			gotParticipant = true
		}
		if status != http.StatusOK && status != http.StatusCreated && status != http.StatusConflict {
			t.Fatalf("unexpected setup/start race status %d", status)
		}
	}
	if !gotStart {
		t.Fatal("episode start did not commit during setup race")
	}
	if !gotParticipant {
		participants, err := fixture.queries.ListParticipantsByInstance(ctx, fixture.instance.ID)
		if err != nil {
			t.Fatalf("list setup race participants: %v", err)
		}
		if len(participants) != 0 {
			t.Fatalf("participant setup returned conflict but participant was inserted")
		}
	}
}

func TestManagedCreateAndImportSerializeOnNameSeason(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	server := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{
		Enabled:      true,
		BearerTokens: []string{"progression-token"},
	}))
	router := server.Router()
	createBody := `{"name":"Concurrent collision","season":105,"managed_progression":true,"contestants":["Concurrent C1"],"episodes":[{"episode_number":0,"label":"Preseason","airs_at":"2026-01-01T20:00:00-05:00"},{"episode_number":1,"label":"Episode 1","airs_at":"2026-01-08T20:00:00-05:00"}]}`
	importBody := `{"name":"Concurrent collision","season":105,"submissions":[{"participant_name":"Imported","rankings":["Concurrent C1"]}]}`
	responses := make(chan int, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		req := authorizedJSONRequest(http.MethodPost, "/instances", createBody, "progression-token", "progression-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		responses <- recorder.Code
	}()
	go func() {
		defer wait.Done()
		req := authorizedJSONRequest(http.MethodPost, "/instances/import", importBody, "progression-token", "progression-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		responses <- recorder.Code
	}()
	wait.Wait()
	close(responses)
	statuses := make(map[int]int)
	for status := range responses {
		statuses[status]++
	}
	if statuses[http.StatusCreated] < 1 || statuses[http.StatusCreated]+statuses[http.StatusConflict] != 2 {
		t.Fatalf("concurrent create/import statuses = %+v", statuses)
	}
	var count, managedCount, legacyCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE progression_mode = 'managed'), COUNT(*) FILTER (WHERE progression_mode = 'legacy') FROM instances WHERE name = 'Concurrent collision' AND season = 105`).Scan(&count, &managedCount, &legacyCount); err != nil {
		t.Fatalf("read concurrent collision state: %v", err)
	}
	if count < 1 || count > 2 || managedCount > 1 || legacyCount > 1 || managedCount+legacyCount != count {
		t.Fatalf("concurrent create/import left invalid state: count=%d managed=%d legacy=%d", count, managedCount, legacyCount)
	}
}

func TestManagedImportCollisionPreservesLegacyAndManagedInstances(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Collision instance", 104, []string{"Collision C1"}, 2)
	legacy, err := fixture.queries.CreateInstance(ctx, db.CreateInstanceParams{Name: "Collision instance", Season: 104})
	if err != nil {
		t.Fatalf("create legacy collision instance: %v", err)
	}
	importBody := `{"name":"Collision instance","season":104,"submissions":[{"participant_name":"Imported","rankings":["Collision C1"]}]}`
	imported := serve(fixture, http.MethodPost, "/instances/import", importBody, "progression-token", "progression-admin")
	if imported.Code != http.StatusConflict {
		t.Fatalf("collision import status = %d, body = %s", imported.Code, imported.Body.String())
	}
	instances, err := fixture.queries.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list collision instances: %v", err)
	}
	matches := 0
	for _, instance := range instances {
		if instance.Name == "Collision instance" && instance.Season == 104 {
			matches++
		}
	}
	if matches != 2 {
		t.Fatalf("collision import changed matching instance count to %d", matches)
	}
	if _, err := fixture.queries.GetInstance(ctx, legacy.ID); err != nil {
		t.Fatalf("legacy collision instance was removed: %v", err)
	}
	if _, err := fixture.queries.GetInstance(ctx, fixture.instance.ID); err != nil {
		t.Fatalf("managed collision instance was removed: %v", err)
	}
}
