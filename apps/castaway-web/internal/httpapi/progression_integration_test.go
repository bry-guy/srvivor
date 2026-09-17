package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestManagedOutcomeCorrectionRejectsUnpublishedPosition(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Unpublished correction", 115, []string{"Unpublished C1", "Unpublished C2"}, 3)
	participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Unpublished Alice")
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list unpublished contestants: %v", err)
	}
	first := uuid.UUID(contestants[0].ID.Bytes).String()
	second := uuid.UUID(contestants[1].ID.Bytes).String()
	path := "/instances/" + fixture.instanceID
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("unpublished-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("open unpublished draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("unpublished-start-1", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start unpublished episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	draft := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"unpublished-draft","effective_at":"2026-01-03T20:01:00Z"}`, first, second)
	if recorder := serve(fixture, http.MethodPut, path+"/drafts/"+uuid.UUID(participant.ID.Bytes).String(), draft, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save unpublished draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	outcome := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"unpublished-outcome","effective_at":"2026-01-04T20:00:00Z"}`, first)
	if recorder := serve(fixture, http.MethodPut, path+"/outcomes/1", outcome, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save unpublished outcome status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("unpublished-complete-1", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("complete unpublished episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("unpublished-score-1", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("score unpublished episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/2/start", progressionCommand("unpublished-start-2", "2026-01-08T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start unpublished next episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	beforeOutcomes, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes before unpublished correction: %v", err)
	}
	var beforeRevisions, beforeCommands int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&beforeRevisions); err != nil {
		t.Fatalf("count revisions before unpublished correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&beforeCommands); err != nil {
		t.Fatalf("count commands before unpublished correction: %v", err)
	}
	correction := fmt.Sprintf(`{"contestant_id":%q,"correction":true,"idempotency_key":"unpublished-correction","effective_at":"2026-01-08T20:01:00Z"}`, second)
	recorder := serve(fixture, http.MethodPut, path+"/outcomes/2", correction, "progression-token", "progression-admin")
	if recorder.Code != http.StatusConflict || recorder.Body.String() != `{"error":"outcome correction must target a published outcome position"}` {
		t.Fatalf("unpublished correction = %d %s", recorder.Code, recorder.Body.String())
	}
	afterOutcomes, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes after unpublished correction: %v", err)
	}
	if !reflect.DeepEqual(afterOutcomes, beforeOutcomes) {
		t.Fatalf("unpublished correction changed live outcomes: before=%+v after=%+v", beforeOutcomes, afterOutcomes)
	}
	var afterRevisions, afterCommands int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&afterRevisions); err != nil {
		t.Fatalf("count revisions after unpublished correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&afterCommands); err != nil {
		t.Fatalf("count commands after unpublished correction: %v", err)
	}
	if afterRevisions != beforeRevisions || afterCommands != beforeCommands {
		t.Fatalf("unpublished correction changed persisted state: revisions=%d/%d commands=%d/%d", beforeRevisions, afterRevisions, beforeCommands, afterCommands)
	}
	published := serve(fixture, http.MethodGet, path+"/outcomes", "", "progression-token", "progression-admin")
	var publishedResponse struct {
		Outcomes []struct {
			Position     int    `json:"position"`
			ContestantID string `json:"contestant_id"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(published.Body.Bytes(), &publishedResponse); err != nil {
		t.Fatalf("decode published outcomes after unpublished correction: %v", err)
	}
	if published.Code != http.StatusOK || len(publishedResponse.Outcomes) != 1 || publishedResponse.Outcomes[0].Position != 1 || publishedResponse.Outcomes[0].ContestantID != first {
		t.Fatalf("unpublished correction changed published outcomes: %d %s", published.Code, published.Body.String())
	}
}

func TestManagedScoringRejectsFutureEffectiveOutcome(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Future outcome", 116, []string{"Future C1", "Future C2"}, 2)
	participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Future Alice")
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list future contestants: %v", err)
	}
	first := uuid.UUID(contestants[0].ID.Bytes).String()
	second := uuid.UUID(contestants[1].ID.Bytes).String()
	path := "/instances/" + fixture.instanceID
	if recorder := serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("future-effective-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("open future draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("future-effective-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("start future episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	draft := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"future-effective-draft","effective_at":"2026-01-03T20:01:00Z"}`, first, second)
	if recorder := serve(fixture, http.MethodPut, path+"/drafts/"+uuid.UUID(participant.ID.Bytes).String(), draft, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save future draft status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	futureOutcome := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"future-effective-outcome","effective_at":"2026-01-10T20:00:00Z"}`, first)
	if recorder := serve(fixture, http.MethodPut, path+"/outcomes/1", futureOutcome, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("save future outcome status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("future-effective-complete", "2026-01-08T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("complete future episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	beforeOutcomes, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes before future score: %v", err)
	}
	var beforeRevisions, beforeCommands int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&beforeRevisions); err != nil {
		t.Fatalf("count revisions before future score: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&beforeCommands); err != nil {
		t.Fatalf("count commands before future score: %v", err)
	}
	earlyScore := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("future-effective-early-score", "2026-01-09T20:00:00Z"), "progression-token", "progression-admin")
	if earlyScore.Code != http.StatusConflict || earlyScore.Body.String() != `{"error":"episode scoring cannot precede an outcome effective time"}` {
		t.Fatalf("early future score = %d %s", earlyScore.Code, earlyScore.Body.String())
	}
	afterOutcomes, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes after future score rejection: %v", err)
	}
	if !reflect.DeepEqual(afterOutcomes, beforeOutcomes) {
		t.Fatalf("future score rejection changed outcomes: before=%+v after=%+v", beforeOutcomes, afterOutcomes)
	}
	progress, err := fixture.queries.GetInstanceEpisodeProgress(ctx, db.GetInstanceEpisodeProgressParams{InstanceID: fixture.instance.ID, EpisodeNumber: 1})
	if err != nil {
		t.Fatalf("read episode after future score rejection: %v", err)
	}
	if progress.Status != "completed" {
		t.Fatalf("future score rejection changed episode status to %q", progress.Status)
	}
	var afterRevisions, afterCommands int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&afterRevisions); err != nil {
		t.Fatalf("count revisions after future score rejection: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&afterCommands); err != nil {
		t.Fatalf("count commands after future score rejection: %v", err)
	}
	if afterRevisions != beforeRevisions || afterCommands != beforeCommands {
		t.Fatalf("future score rejection changed persisted state: revisions=%d/%d commands=%d/%d", beforeRevisions, afterRevisions, beforeCommands, afterCommands)
	}
	validScore := serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("future-effective-valid-score", "2026-01-11T20:00:00Z"), "progression-token", "progression-admin")
	if validScore.Code != http.StatusOK {
		t.Fatalf("valid chronological score = %d %s", validScore.Code, validScore.Body.String())
	}
	var snapshot []byte
	if err := pool.QueryRow(ctx, `SELECT input_snapshot FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&snapshot); err != nil {
		t.Fatalf("read valid chronological snapshot: %v", err)
	}
	var decoded struct {
		Outcomes map[string]int `json:"outcomes"`
	}
	if err := json.Unmarshal(snapshot, &decoded); err != nil {
		t.Fatalf("decode valid chronological snapshot: %v", err)
	}
	if decoded.Outcomes[first] != 1 || len(decoded.Outcomes) != 1 {
		t.Fatalf("valid chronological score omitted accepted outcome: %+v", decoded.Outcomes)
	}
}

func TestManagedOutcomeCorrectionBoundariesAfterScoring(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Outcome correction boundaries", 114, []string{"Boundary C1", "Boundary C2"}, 2)
	contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("list boundary contestants: %v", err)
	}
	createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Boundary Alice")
	path := "/instances/" + fixture.instanceID
	first := uuid.UUID(contestants[0].ID.Bytes).String()
	second := uuid.UUID(contestants[1].ID.Bytes).String()
	mustOK := func(recorder *httptest.ResponseRecorder, action string) {
		t.Helper()
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", action, recorder.Code, recorder.Body.String())
		}
	}
	mustOK(serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("boundary-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"), "open boundary draft")
	mustOK(serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("boundary-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"), "start boundary episode")
	outcomeBody := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"boundary-outcome","effective_at":"2026-01-04T20:00:00Z"}`, first)
	originalOutcome := serve(fixture, http.MethodPut, path+"/outcomes/1", outcomeBody, "progression-token", "progression-admin")
	mustOK(originalOutcome, "save boundary outcome")
	mustOK(serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("boundary-complete", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin"), "complete boundary episode")
	mustOK(serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("boundary-score", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin"), "score boundary episode")
	var originalOutcomeResponse, retriedOutcomeResponse map[string]any
	if err := json.Unmarshal(originalOutcome.Body.Bytes(), &originalOutcomeResponse); err != nil {
		t.Fatalf("decode original boundary outcome: %v", err)
	}
	retriedOutcome := serve(fixture, http.MethodPut, path+"/outcomes/1", outcomeBody, "progression-token", "progression-admin")
	if err := json.Unmarshal(retriedOutcome.Body.Bytes(), &retriedOutcomeResponse); err != nil {
		t.Fatalf("decode retried boundary outcome: %v", err)
	}
	if retriedOutcome.Code != http.StatusOK || !reflect.DeepEqual(retriedOutcomeResponse, originalOutcomeResponse) {
		t.Fatalf("successful outcome retry after scoring = %d %s, original = %s", retriedOutcome.Code, retriedOutcome.Body.String(), originalOutcome.Body.String())
	}
	outcomesBeforeFalse, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes before false correction: %v", err)
	}
	var revisionsBeforeFalse, commandsBeforeFalse int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisionsBeforeFalse); err != nil {
		t.Fatalf("count revisions before false correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandsBeforeFalse); err != nil {
		t.Fatalf("count commands before false correction: %v", err)
	}
	falseCorrection := fmt.Sprintf(`{"contestant_id":%q,"correction":false,"idempotency_key":"boundary-false-correction","effective_at":"2026-01-06T20:00:00Z"}`, second)
	falseCorrectionRecorder := serve(fixture, http.MethodPut, path+"/outcomes/1", falseCorrection, "progression-token", "progression-admin")
	var falseCorrectionResponse struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(falseCorrectionRecorder.Body.Bytes(), &falseCorrectionResponse); err != nil {
		t.Fatalf("decode false correction response: %v", err)
	}
	if falseCorrectionRecorder.Code != http.StatusConflict || falseCorrectionResponse.Error != "use correction: true to fix published outcomes or start the next episode to record new outcomes" {
		t.Fatalf("explicit false correction = %d %s", falseCorrectionRecorder.Code, falseCorrectionRecorder.Body.String())
	}
	outcomesAfterFalse, err := fixture.queries.ListOutcomePositionsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read outcomes after false correction: %v", err)
	}
	if !reflect.DeepEqual(outcomesAfterFalse, outcomesBeforeFalse) {
		t.Fatalf("false correction changed outcomes: before=%+v after=%+v", outcomesBeforeFalse, outcomesAfterFalse)
	}
	var revisionsAfterFalse, commandsAfterFalse int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisionsAfterFalse); err != nil {
		t.Fatalf("count revisions after false correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandsAfterFalse); err != nil {
		t.Fatalf("count commands after false correction: %v", err)
	}
	if revisionsAfterFalse != revisionsBeforeFalse || commandsAfterFalse != commandsBeforeFalse {
		t.Fatalf("false correction changed persisted counts: revisions=%d/%d commands=%d/%d", revisionsBeforeFalse, revisionsAfterFalse, commandsBeforeFalse, commandsAfterFalse)
	}
	trueCorrection := fmt.Sprintf(`{"contestant_id":%q,"correction":true,"idempotency_key":"boundary-true-correction","effective_at":"2026-01-06T20:01:00Z"}`, second)
	trueCorrectionRecorder := serve(fixture, http.MethodPut, path+"/outcomes/1", trueCorrection, "progression-token", "progression-admin")
	mustOK(trueCorrectionRecorder, "true correction after scoring")
	if !strings.Contains(trueCorrectionRecorder.Body.String(), "revision_number") {
		t.Fatalf("true correction did not publish a revision: %s", trueCorrectionRecorder.Body.String())
	}
	trueCorrectionRetry := serve(fixture, http.MethodPut, path+"/outcomes/1", trueCorrection, "progression-token", "progression-admin")
	var trueCorrectionResponse, trueCorrectionRetryResponse map[string]any
	if err := json.Unmarshal(trueCorrectionRecorder.Body.Bytes(), &trueCorrectionResponse); err != nil {
		t.Fatalf("decode true correction: %v", err)
	}
	if err := json.Unmarshal(trueCorrectionRetry.Body.Bytes(), &trueCorrectionRetryResponse); err != nil {
		t.Fatalf("decode true correction retry: %v", err)
	}
	if trueCorrectionRetry.Code != http.StatusOK || !reflect.DeepEqual(trueCorrectionRetryResponse, trueCorrectionResponse) {
		t.Fatalf("true correction retry = %d %s, original = %s", trueCorrectionRetry.Code, trueCorrectionRetry.Body.String(), trueCorrectionRecorder.Body.String())
	}
	var revisionsAfterTrue, commandsAfterTrue int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_score_revisions WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&revisionsAfterTrue); err != nil {
		t.Fatalf("count revisions after true correction: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM instance_progression_commands WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)`, fixture.instance.ID).Scan(&commandsAfterTrue); err != nil {
		t.Fatalf("count commands after true correction: %v", err)
	}
	if revisionsAfterTrue != 2 || commandsAfterTrue != 6 {
		t.Fatalf("true correction or retries persisted unexpected counts: revisions=%d commands=%d", revisionsAfterTrue, commandsAfterTrue)
	}
}

func TestManagedProgressionNormalizesAcrossWallClocks(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	early := newManagedProgressionFixture(t, ctx, pool, "Clock early", 111, []string{"Clock C1", "Clock C2"}, 2, time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC))
	late := newManagedProgressionFixture(t, ctx, pool, "Clock late", 112, []string{"Clock Late C1", "Clock Late C2"}, 2, time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC))
	type normalizedProgression struct {
		checkpoints []byte
		publication []byte
		scores      []byte
	}
	run := func(fixture managedProgressionFixture) normalizedProgression {
		contestants, err := fixture.queries.ListContestantsByInstance(ctx, fixture.instance.ID)
		if err != nil {
			t.Fatalf("list clock contestants: %v", err)
		}
		participant := createParticipantForTest(t, ctx, fixture.queries, fixture.instance.ID, "Clock Alice")
		path := "/instances/" + fixture.instanceID
		must := func(recorder *httptest.ResponseRecorder, action string) {
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, body = %s", action, recorder.Code, recorder.Body.String())
			}
		}
		must(serve(fixture, http.MethodPost, path+"/progression/draft/open", progressionCommand("clock-open", "2026-01-02T20:00:00Z"), "progression-token", "progression-admin"), "clock open")
		must(serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("clock-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin"), "clock start")
		first := uuid.UUID(contestants[0].ID.Bytes).String()
		second := uuid.UUID(contestants[1].ID.Bytes).String()
		draft := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"clock-draft","effective_at":"2026-01-03T20:01:00Z"}`, first, second)
		must(serve(fixture, http.MethodPut, path+"/drafts/"+uuid.UUID(participant.ID.Bytes).String(), draft, "progression-token", "progression-admin"), "clock draft")
		outcome := fmt.Sprintf(`{"contestant_id":%q,"idempotency_key":"clock-outcome","effective_at":"2026-01-04T20:00:00Z"}`, first)
		must(serve(fixture, http.MethodPut, path+"/outcomes/1", outcome, "progression-token", "progression-admin"), "clock outcome")
		must(serve(fixture, http.MethodPost, path+"/progression/episodes/1/complete", progressionCommand("clock-complete", "2026-01-05T20:00:00Z"), "progression-token", "progression-admin"), "clock complete")
		must(serve(fixture, http.MethodPost, path+"/progression/episodes/1/score", progressionCommand("clock-score", "2026-01-06T20:00:00Z"), "progression-token", "progression-admin"), "clock score")
		var checkpoints []byte
		if err := pool.QueryRow(ctx, `SELECT json_build_object(
			'draft', (SELECT json_build_object('status', status, 'opened_at', opened_at, 'closed_at', closed_at) FROM instance_draft_progress WHERE instance_id = i.id),
			'episodes', COALESCE((SELECT json_agg(json_build_object('episode_number', episode_number, 'status', status, 'started_at', started_at, 'completed_at', completed_at, 'scored_at', scored_at) ORDER BY episode_number) FROM instance_episode_progress WHERE instance_id = i.id), '[]'::json)
		)::json FROM instances i WHERE i.public_id = $1`, fixture.instance.ID).Scan(&checkpoints); err != nil {
			t.Fatalf("read normalized clock checkpoints: %v", err)
		}
		var publication []byte
		if err := pool.QueryRow(ctx, `SELECT json_build_object('revision_number', revision_number, 'episode_number', episode_number, 'reason', reason, 'effective_at', effective_at, 'input_snapshot', input_snapshot)::json FROM instance_score_revisions r JOIN instances i ON i.id = r.instance_id WHERE i.public_id = $1 ORDER BY revision_number DESC LIMIT 1`, fixture.instance.ID).Scan(&publication); err != nil {
			t.Fatalf("read normalized clock publication: %v", err)
		}
		publication = bytes.ReplaceAll(publication, []byte(uuid.UUID(participant.ID.Bytes).String()), []byte("participant-1"))
		for index, contestant := range contestants {
			publication = bytes.ReplaceAll(publication, []byte(uuid.UUID(contestant.ID.Bytes).String()), []byte(fmt.Sprintf("contestant-%d", index+1)))
			publication = bytes.ReplaceAll(publication, []byte(contestant.Name), []byte(fmt.Sprintf("contestant-%d", index+1)))
		}
		var scores []byte
		if err := pool.QueryRow(ctx, `SELECT COALESCE(json_agg(row_to_json(rows) ORDER BY rows.total_points DESC, rows.draft_points DESC, rows.bonus_points DESC), '[]'::json) FROM (SELECT participant_name, score, draft_points, bonus_points, total_points, points_available FROM instance_score_revision_rows WHERE revision_id = (SELECT MAX(isr.id) FROM instance_score_revisions isr JOIN instances i ON i.id = isr.instance_id WHERE i.public_id = $1)) rows`, fixture.instance.ID).Scan(&scores); err != nil {
			t.Fatalf("read normalized clock scores: %v", err)
		}
		return normalizedProgression{checkpoints: checkpoints, publication: publication, scores: scores}
	}
	earlyResult := run(early)
	lateResult := run(late)
	compareJSON := func(label string, left, right []byte) {
		var leftValue, rightValue any
		if err := json.Unmarshal(left, &leftValue); err != nil {
			t.Fatalf("decode early %s: %v", label, err)
		}
		if err := json.Unmarshal(right, &rightValue); err != nil {
			t.Fatalf("decode late %s: %v", label, err)
		}
		if !reflect.DeepEqual(leftValue, rightValue) {
			t.Fatalf("equivalent managed progression %s changed with wall clock: early=%s late=%s", label, left, right)
		}
	}
	compareJSON("checkpoints", earlyResult.checkpoints, lateResult.checkpoints)
	compareJSON("publication", earlyResult.publication, lateResult.publication)
	compareJSON("scores", earlyResult.scores, lateResult.scores)
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
	if recorder := serve(fixture, http.MethodPost, path+"/progression/episodes/2/complete", progressionCommand("checkpoint-complete-2", "2026-01-09T20:00:00Z"), "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("complete next checkpoint episode status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	backdatedAfterCompletion := fmt.Sprintf(`{"contestant_ids":[%q,%q],"idempotency_key":"checkpoint-after-complete-backdated","effective_at":"2026-01-08T21:00:00Z"}`, first, second)
	if recorder := serve(fixture, http.MethodPut, participantPath, backdatedAfterCompletion, "progression-token", "progression-admin"); recorder.Code != http.StatusConflict {
		t.Fatalf("backdated completed-episode correction status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := serve(fixture, http.MethodPut, participantPath, validCorrection, "progression-token", "progression-admin"); recorder.Code != http.StatusOK {
		t.Fatalf("retry of successful correction status = %d, body = %s", recorder.Code, recorder.Body.String())
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

func TestManagedSetupWaitsForHeldInstanceLock(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	fixture := newManagedProgressionFixture(t, ctx, pool, "Held lock setup", 113, []string{"Held C1"}, 2)
	path := "/instances/" + fixture.instanceID
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}
	if _, err := db.New(tx).LockInstanceForProgression(ctx, fixture.instance.ID); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			t.Logf("rollback after lock failure: %v", rollbackErr)
		}
		t.Fatalf("hold instance lock: %v", err)
	}
	ready := make(chan struct{}, 2)
	begin := make(chan struct{})
	results := make(chan struct {
		operation string
		status    int
	}, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		ready <- struct{}{}
		<-begin
		results <- struct {
			operation string
			status    int
		}{operation: "participant", status: serve(fixture, http.MethodPost, path+"/participants", `{"name":"Held Alice"}`, "progression-token", "progression-admin").Code}
	}()
	go func() {
		defer wait.Done()
		ready <- struct{}{}
		<-begin
		results <- struct {
			operation string
			status    int
		}{operation: "start", status: serve(fixture, http.MethodPost, path+"/progression/episodes/1/start", progressionCommand("held-start", "2026-01-03T20:00:00Z"), "progression-token", "progression-admin").Code}
	}()
	<-ready
	<-ready
	close(begin)
	deadline := time.NewTimer(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	waiting := false
	for !waiting {
		if err := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1
			FROM pg_stat_activity activity
			JOIN pg_locks lock ON lock.pid = activity.pid AND NOT lock.granted
			JOIN pg_class relation ON relation.oid = lock.relation
			WHERE activity.pid <> pg_backend_pid()
			  AND relation.relname = 'instances'
		)`).Scan(&waiting); err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Logf("rollback after lock inspection failure: %v", rollbackErr)
			}
			t.Fatalf("inspect lock wait: %v", err)
		}
		if waiting {
			break
		}
		select {
		case <-deadline.C:
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				t.Logf("rollback after lock wait timeout: %v", rollbackErr)
			}
			t.Fatalf("setup/start requests did not contend on the held instance lock")
		case <-ticker.C:
		}
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("release held instance lock: %v", err)
	}
	wait.Wait()
	close(results)
	statuses := make(map[string]int, 2)
	for result := range results {
		statuses[result.operation] = result.status
	}
	if statuses["start"] != http.StatusOK {
		t.Fatalf("episode start did not succeed after releasing held lock: %+v", statuses)
	}
	if statuses["participant"] != http.StatusCreated && statuses["participant"] != http.StatusConflict {
		t.Fatalf("unexpected held-lock participant status: %+v", statuses)
	}
	participants, err := fixture.queries.ListParticipantsByInstance(ctx, fixture.instance.ID)
	if err != nil {
		t.Fatalf("read roster after held lock: %v", err)
	}
	if statuses["participant"] == http.StatusCreated && len(participants) != 1 {
		t.Fatalf("created participant missing after held lock: %+v", participants)
	}
	if statuses["participant"] == http.StatusConflict && len(participants) != 0 {
		t.Fatalf("conflicted participant setup inserted a participant: %+v", participants)
	}
	progress, err := fixture.queries.GetInstanceEpisodeProgress(ctx, db.GetInstanceEpisodeProgressParams{InstanceID: fixture.instance.ID, EpisodeNumber: 1})
	if err != nil {
		t.Fatalf("read episode after held lock: %v", err)
	}
	if progress.Status != "started" || !progress.StartedAt.Valid || !progress.StartedAt.Time.Equal(time.Date(2026, time.January, 3, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected serialized episode state: %+v", progress)
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
	managedFirstCreateBody := `{"name":"Managed-first collision","season":105,"managed_progression":true,"contestants":["Managed-first C1"],"episodes":[{"episode_number":0,"label":"Preseason","airs_at":"2026-01-01T20:00:00-05:00"},{"episode_number":1,"label":"Episode 1","airs_at":"2026-01-08T20:00:00-05:00"}]}`
	managedFirstImportBody := `{"name":"Managed-first collision","season":105,"submissions":[{"participant_name":"Imported","rankings":["Managed-first C1"]}]}`
	createFirst := httptest.NewRecorder()
	router.ServeHTTP(createFirst, authorizedJSONRequest(http.MethodPost, "/instances", managedFirstCreateBody, "progression-token", "progression-admin"))
	if createFirst.Code != http.StatusCreated {
		t.Fatalf("managed-first create status = %d, body = %s", createFirst.Code, createFirst.Body.String())
	}
	importAfterManaged := httptest.NewRecorder()
	router.ServeHTTP(importAfterManaged, authorizedJSONRequest(http.MethodPost, "/instances/import", managedFirstImportBody, "progression-token", "progression-admin"))
	if importAfterManaged.Code != http.StatusConflict {
		t.Fatalf("managed-first import status = %d, body = %s", importAfterManaged.Code, importAfterManaged.Body.String())
	}
	var managedFirstCount, managedFirstManaged, managedFirstLegacy int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE progression_mode = 'managed'), COUNT(*) FILTER (WHERE progression_mode = 'legacy') FROM instances WHERE name = 'Managed-first collision' AND season = 105`).Scan(&managedFirstCount, &managedFirstManaged, &managedFirstLegacy); err != nil {
		t.Fatalf("read managed-first collision state: %v", err)
	}
	if managedFirstCount != 1 || managedFirstManaged != 1 || managedFirstLegacy != 0 {
		t.Fatalf("managed-first collision left invalid state: count=%d managed=%d legacy=%d", managedFirstCount, managedFirstManaged, managedFirstLegacy)
	}

	createBody := `{"name":"Concurrent collision","season":106,"managed_progression":true,"contestants":["Concurrent C1"],"episodes":[{"episode_number":0,"label":"Preseason","airs_at":"2026-01-01T20:00:00-05:00"},{"episode_number":1,"label":"Episode 1","airs_at":"2026-01-08T20:00:00-05:00"}]}`
	importBody := `{"name":"Concurrent collision","season":106,"submissions":[{"participant_name":"Imported","rankings":["Concurrent C1"]}]}`
	responses := make(chan struct {
		operation string
		status    int
	}, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		req := authorizedJSONRequest(http.MethodPost, "/instances", createBody, "progression-token", "progression-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		responses <- struct {
			operation string
			status    int
		}{operation: "create", status: recorder.Code}
	}()
	go func() {
		defer wait.Done()
		req := authorizedJSONRequest(http.MethodPost, "/instances/import", importBody, "progression-token", "progression-admin")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		responses <- struct {
			operation string
			status    int
		}{operation: "import", status: recorder.Code}
	}()
	wait.Wait()
	close(responses)
	statuses := make(map[string]int)
	for response := range responses {
		statuses[response.operation] = response.status
	}
	if statuses["create"] != http.StatusCreated || (statuses["import"] != http.StatusCreated && statuses["import"] != http.StatusConflict) {
		t.Fatalf("concurrent create/import statuses = %+v", statuses)
	}
	var count, managedCount, legacyCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE progression_mode = 'managed'), COUNT(*) FILTER (WHERE progression_mode = 'legacy') FROM instances WHERE name = 'Concurrent collision' AND season = 106`).Scan(&count, &managedCount, &legacyCount); err != nil {
		t.Fatalf("read concurrent collision state: %v", err)
	}
	expectedLegacy := 0
	if statuses["import"] == http.StatusCreated {
		expectedLegacy = 1
	}
	if count != 1+expectedLegacy || managedCount != 1 || legacyCount != expectedLegacy {
		t.Fatalf("concurrent create/import left invalid state: count=%d managed=%d legacy=%d expected_legacy=%d", count, managedCount, legacyCount, expectedLegacy)
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
