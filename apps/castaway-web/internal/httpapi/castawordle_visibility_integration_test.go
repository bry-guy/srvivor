package httpapi_test

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCastawordleVisibilityAndSchedule(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Scheduled puzzles", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, instance.ID, 51); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)
	for _, user := range []string{"admin", "player", "unlinked-admin"} {
		if user != "unlinked-admin" {
			player := createParticipantForTest(t, ctx, q, instance.ID, user)
			if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: player.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
				t.Fatal(err)
			}
		}
		if user != "player" {
			if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: user}); err != nil {
				t.Fatal(err)
			}
		}
		hash := sha256.Sum256([]byte(user + "-visibility"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 0, 30)); err != nil {
			t.Fatal(err)
		}
	}
	const base = "https://castaway.example"
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID})).PublicRouter()
	serve := func(method, path, body, user string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		req.Header.Set("X-Discord-User-ID", "admin")
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-visibility"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: HTTP %d, want %d: %s", method, path, user, rec.Code, status, rec.Body)
		}
		return rec
	}
	createPath := "/api/instances/" + instanceID + "/castawordle"
	create := func(body string) string {
		t.Helper()
		rec := serve("POST", createPath, body, "admin", 201)
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.ID == "" {
			t.Fatalf("creation: %v: %s", err, rec.Body)
		}
		if strings.Contains(rec.Body.String(), `"answer"`) {
			t.Fatal("creation exposed the answer")
		}
		return result.ID
	}
	testID := create(`{"name":"Private practice","answer":"TORCH"}`)
	testPage := "/castawordle/" + testID
	testPlay := "/api/castawordle/" + testID + "/play"
	for _, path := range []string{testPage, testPlay} {
		serve("GET", path, "", "admin", 200)
		serve("GET", path, "", "player", 404)
	}
	serve("POST", testPlay+"/guesses", `{"guess":"CRANE","position":1}`, "player", 404)
	serve("POST", testPlay+"/guesses", `{"guess":"CRANE","position":1}`, "admin", 200)
	serve("GET", testPlay, "", "unlinked-admin", 403)
	serve("POST", testPlay+"/guesses", `{"guess":"TORCH","position":2}`, "player", 404)
	if rec := serve("GET", testPlay, "", "admin", 200); strings.Count(rec.Body.String(), `"word":`) != 1 {
		t.Fatal("private test progress was changed or lost")
	}
	if page := serve("GET", "/castawordle", "", "player", 200); strings.Contains(page.Body.String(), testID) || strings.Contains(page.Body.String(), "Private practice") || strings.Contains(page.Body.String(), "create-game") {
		t.Fatal("ordinary player discovered an admin test or creation form")
	}
	if page := serve("GET", "/castawordle", "", "admin", 200); !strings.Contains(page.Body.String(), testID) || !strings.Contains(page.Body.String(), `name="episode_number"`) {
		t.Fatal("admin cannot discover tests or prepare episode puzzles")
	}
	serve("POST", createPath, `{"name":"No","answer":"TORCH","episode_number":2}`, "player", 403)
	for _, body := range []string{
		`{"name":"No","answer":"TORCH","episode_number":-1}`,
		`{"name":"No","answer":"TORCH","episode_number":99}`,
		`{"name":"No","answer":"TORCH","episode_number":13}`,
		`{"name":"No","answer":"TORCH","episode_number":1}`,
		`{"name":"No","answer":"TORCH","episode_number":2,"opens_at":"2026-09-30T16:00:00Z"}`,
		`{"name":"No","answer":"TORCH","episode_number":2,"cutoff_at":"2026-10-07T18:00:00Z"}`,
	} {
		serve("POST", createPath, body, "admin", 400)
	}
	gameID := create(`{"name":"Episode challenge","answer":"TORCH","episode_number":2}`)
	gameUUID := pgtype.UUID{Bytes: uuid.MustParse(gameID), Valid: true}
	game, err := q.GetCastawordleGame(ctx, gameUUID)
	if err != nil {
		t.Fatal(err)
	}
	if !game.EpisodeNumber.Valid || game.EpisodeNumber.Int32 != 2 || !game.OpensAt.Time.Equal(time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)) || !game.CutoffAt.Time.Equal(time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("incorrect schedule-derived game: episode %v, %s → %s", game.EpisodeNumber, game.OpensAt.Time, game.CutoffAt.Time)
	}
	serve("POST", createPath, `{"name":"Overwrite","answer":"OUTCAST","episode_number":2}`, "admin", 409)
	unchanged, err := q.GetCastawordleGame(ctx, gameUUID)
	if err != nil || unchanged.Answer != "TORCH" {
		t.Fatalf("duplicate preparation overwrote existing puzzle: %v", err)
	}
	if page := serve("GET", "/castawordle", "", "player", 200); !strings.Contains(page.Body.String(), gameID) || strings.Contains(page.Body.String(), testID) {
		t.Fatal("player list did not distinguish scheduled games and tests")
	}
	serve("GET", "/castawordle/"+gameID, "", "player", 200)
	playPath := "/api/castawordle/" + gameID + "/play"
	if rec := serve("GET", playPath, "", "player", 200); !strings.Contains(rec.Body.String(), `"status":"not_open"`) || strings.Contains(rec.Body.String(), `"answer"`) {
		t.Fatal("prepared game leaked answer or failed to report not_open")
	}
	serve("POST", playPath+"/guesses", `{"guess":"CRANE","position":1}`, "player", 409)
	var plays int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM castawordle_plays WHERE game_id = (SELECT id FROM castawordle_games WHERE public_id = $1)`, gameUUID).Scan(&plays); err != nil || plays != 0 {
		t.Fatalf("premature guess wrote a play: count %d, error %v", plays, err)
	}
	now = game.OpensAt.Time
	serve("POST", playPath+"/guesses", `{"guess":"CRANE","position":1}`, "player", 200)
	now = game.CutoffAt.Time
	serve("POST", playPath+"/guesses", `{"guess":"TORCH","position":2}`, "player", 409)
	serve("POST", playPath+"/guesses", `{"guess":"CRANE","position":1}`, "player", 200)
	foreign := createInstanceForTest(t, ctx, q, "Foreign scheduled puzzles", 51)
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, foreign.ID, 51); err != nil {
		t.Fatal(err)
	}
	foreignGame, err := q.CreateCastawordleGame(ctx, db.CreateCastawordleGameParams{
		InstanceID: foreign.ID, Name: "Foreign episode", Answer: "TORCH", DictionaryVersion: "test",
		EpisodeNumber: pgtype.Int4{Int32: 2, Valid: true}, OpensAt: game.OpensAt, CutoffAt: game.CutoffAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	foreignID := uuid.UUID(foreignGame.ID.Bytes).String()
	serve("GET", "/castawordle/"+foreignID, "", "player", 404)
	serve("GET", "/api/castawordle/"+foreignID+"/play", "", "player", 403)
	serve("POST", "/api/castawordle/"+foreignID+"/play/guesses", `{"guess":"TORCH","position":1}`, "player", 403)
	var scoringRows int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM instance_activities) +
		(SELECT count(*) FROM activity_occurrences) +
		(SELECT count(*) FROM activity_occurrence_participants) +
		(SELECT count(*) FROM bonus_point_ledger_entries)`).Scan(&scoringRows); err != nil || scoringRows != 0 {
		t.Fatalf("scheduling/gameplay changed scoring: count %d, error %v", scoringRows, err)
	}
}
