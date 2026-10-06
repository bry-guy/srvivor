package httpapi_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCastawordle(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Castawordle preview", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, instance.ID, 51); err != nil {
		t.Fatal(err)
	}
	episodes := gameplay.DefaultEpisodeScheduleForSeason(51)
	for _, user := range []string{"cw-player", "cw-other"} {
		p := createParticipantForTest(t, ctx, q, instance.ID, user)
		if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: p.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "cw-player"}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)
	for _, user := range []string{"cw-player", "cw-other", "cw-stranger"} {
		hash := sha256.Sum256([]byte(user + "-session"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 0, 90)); err != nil {
			t.Fatal(err)
		}
	}
	const base = "https://castaway.example"
	server := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID}))
	router := server.PublicRouter()
	serve := func(method, path, body, user, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		if user != "" {
			req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-session"})
		}
		req.Header.Set("X-Discord-User-ID", "cw-other")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	want := func(rec *httptest.ResponseRecorder, status int) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("HTTP %d, want %d: %s", rec.Code, status, rec.Body)
		}
	}
	episodeNumber := 2
	create := func(answer string) string {
		t.Helper()
		now = episodes[episodeNumber].AirsAt.Add(-7 * time.Hour)
		rec := serve("POST", "/api/instances/"+instanceID+"/castawordle", fmt.Sprintf(`{"name":"Scheduled","answer":%q,"episode_number":%d}`, answer, episodeNumber), "cw-player", base)
		episodeNumber++
		want(rec, 201)
		var result struct {
			ID       string `json:"id"`
			Unscored bool   `json:"unscored"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.ID == "" || !result.Unscored || strings.Contains(rec.Body.String(), `"answer"`) {
			t.Fatalf("unsafe creation response: %s", rec.Body)
		}
		return result.ID
	}
	gameID := create("CACAO")
	playPath := "/api/castawordle/" + gameID + "/play"
	guess := func(word string, position int, user string) *httptest.ResponseRecorder {
		return serve("POST", playPath+"/guesses", fmt.Sprintf(`{"guess":%q,"position":%d}`, word, position), user, base)
	}
	for _, path := range []string{"/", "/games", "/games/castawordle/" + gameID} {
		rec := serve("GET", path, "", "", "")
		want(rec, 302)
		if !strings.HasPrefix(rec.Header().Get("Location"), "/auth/login?next=") {
			t.Fatalf("page not protected: %s", rec.Header().Get("Location"))
		}
		want(serve("GET", path, "", "cw-player", ""), 200)
		stranger := serve("GET", path, "", "cw-stranger", "")
		want(stranger, 200)
		if !strings.Contains(stranger.Body.String(), "Request access") || strings.Contains(stranger.Body.String(), "castawordle-game") {
			t.Fatalf("unlinked content leak: %s", stranger.Body)
		}
	}
	want(serve("GET", "/healthz", "", "", ""), 200)
	want(serve("GET", "/assets/site.css", "", "", ""), 200)
	want(serve("GET", playPath, "", "", ""), 401)
	want(serve("GET", playPath, "", "cw-stranger", ""), 403)
	want(serve("POST", playPath+"/guesses", `{"guess":"CACAO","position":1}`, "cw-player", "https://evil.example"), 403)
	want(serve("POST", "/api/instances/"+instanceID+"/castawordle", `{"name":"No","answer":"TORCH"}`, "cw-other", base), 403)
	want(guess("ZZZZZ", 1, "cw-player"), 400)
	want(guess("TORCHES", 1, "cw-player"), 400)
	var untouched int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM castawordle_plays").Scan(&untouched); err != nil || untouched != 0 {
		t.Fatalf("invalid-only requests created play: count=%d error=%v", untouched, err)
	}
	initial := serve("GET", playPath, "", "cw-player", "")
	want(initial, 200)
	if strings.Contains(initial.Body.String(), "CACAO") || !strings.Contains(initial.Body.String(), `"not_started"`) {
		t.Fatalf("initial state leaked or changed: %s", initial.Body)
	}
	page := serve("GET", "/games/castawordle/"+gameID, "", "cw-player", "")
	if strings.Contains(page.Body.String(), "CACAO") {
		t.Fatal("HTML leaked answer")
	}
	first := guess("COCOA", 1, "cw-player")
	want(first, 200)
	if !strings.Contains(first.Body.String(), `"feedback":["correct","present","correct","absent","present"]`) || strings.Contains(first.Body.String(), `"answer"`) {
		t.Fatalf("feedback: %s", first.Body)
	}
	want(guess("COCOA", 1, "cw-player"), 200)
	want(guess("CRANE", 1, "cw-player"), 409)
	other := serve("GET", playPath, "", "cw-other", "")
	if strings.Contains(other.Body.String(), "COCOA") {
		t.Fatal("another player's progress leaked")
	}
	want(guess("CACAO", 2, "cw-player"), 200)
	want(guess("CACAO", 2, "cw-player"), 200)
	want(guess("CRANE", 3, "cw-player"), 409)
	router = httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID})).PublicRouter()
	resumed := serve("GET", playPath, "", "cw-player", "")
	if !strings.Contains(resumed.Body.String(), `"status":"solved"`) || !strings.Contains(resumed.Body.String(), `"answer":"CACAO"`) {
		t.Fatalf("resume: %s", resumed.Body)
	}

	gameID = create("TORCH")
	playPath = "/api/castawordle/" + gameID + "/play"
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- guess("CRANE", 1, "cw-player")
		}()
	}
	wg.Wait()
	close(responses)
	for rec := range responses {
		want(rec, 200)
	}
	state := serve("GET", playPath, "", "cw-player", "")
	if strings.Count(state.Body.String(), `"word":`) != 1 {
		t.Fatalf("concurrent retry consumed extra guess: %s", state.Body)
	}
	for pos := 2; pos <= 5; pos++ {
		want(guess("CRANE", pos, "cw-player"), 200)
	}
	last := guess("TORCH", 6, "cw-player")
	want(last, 200)
	if !strings.Contains(last.Body.String(), `"status":"solved"`) {
		t.Fatalf("sixth guess should win: %s", last.Body)
	}
	for pos := 1; pos <= 6; pos++ {
		want(guess("CRANE", pos, "cw-other"), 200)
	}
	failed := serve("GET", playPath, "", "cw-other", "")
	if !strings.Contains(failed.Body.String(), `"status":"exhausted"`) || !strings.Contains(failed.Body.String(), `"answer":"TORCH"`) {
		t.Fatalf("exhaustion: %s", failed.Body)
	}
	gameID = create("TORCH")
	playPath = "/api/castawordle/" + gameID + "/play"
	conflicts := make(chan int, 2)
	for _, word := range []string{"CRANE", "SLATE"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conflicts <- guess(word, 1, "cw-player").Code
		}()
	}
	wg.Wait()
	close(conflicts)
	accepted, rejected := 0, 0
	for code := range conflicts {
		switch code {
		case 200:
			accepted++
		case 409:
			rejected++
		default:
			t.Fatalf("conflicting guess HTTP %d", code)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("conflicting guesses: accepted=%d rejected=%d", accepted, rejected)
	}
	for _, answer := range []string{"OUTCAST", "SURVIVOR"} {
		gameID = create(answer)
		playPath = "/api/castawordle/" + gameID + "/play"
		want(guess(answer, 1, "cw-player"), 200)
	}
	gameID = create("TORCH")
	playPath = "/api/castawordle/" + gameID + "/play"
	want(guess("CRANE", 1, "cw-player"), 200)
	now = now.AddDate(0, 0, 8)
	want(guess("TORCH", 2, "cw-player"), 409)
	want(guess("CRANE", 1, "cw-player"), 200)
	if expired := serve("GET", playPath, "", "cw-player", ""); !strings.Contains(expired.Body.String(), `"status":"expired"`) || strings.Contains(expired.Body.String(), `"answer"`) {
		t.Fatalf("expired play: %s", expired.Body)
	}
	foreign := createInstanceForTest(t, ctx, q, "Other season", 50)
	foreignGame, err := q.CreateCastawordleGame(ctx, db.CreateCastawordleGameParams{
		InstanceID: foreign.ID, Name: "Foreign", Answer: "TORCH", DictionaryVersion: "test",
		OpensAt: pgtype.Timestamptz{Time: now, Valid: true}, CutoffAt: pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want(serve("GET", "/api/castawordle/"+uuid.UUID(foreignGame.ID.Bytes).String()+"/play", "", "cw-player", ""), 403)
	for _, table := range []string{"instance_activities", "activity_occurrences", "activity_occurrence_participants", "bonus_point_ledger_entries"} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("unscored preview changed %s: %d rows", table, count)
		}
	}
}
