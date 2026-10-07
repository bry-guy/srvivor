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
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Press the Button: only open-window presses count, awards follow the rules at cutoff, and resolving twice is a no-op.
func TestPressTheButton(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Button season", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	now := time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC)
	ids := map[string]string{}
	for _, user := range []string{"admin", "most", "second", "twinA", "twinB", "least", "idle"} {
		player := createParticipantForTest(t, ctx, q, instance.ID, user)
		ids[user] = uuid.UUID(player.ID.Bytes).String()
		if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: player.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user + "-button"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	const base = "https://castaway.example"
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID})).PublicRouter()
	serve := func(method, path, body, user string, status int, headers ...string) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-button"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: HTTP %d, want %d: %s", method, path, user, rec.Code, status, rec.Body)
		}
		return rec.Body.String()
	}
	create := "/api/instances/" + instanceID + "/button-games"
	window := `{"episode_number":3,"opens_at":"2026-10-08T00:00:00Z","cutoff_at":"2026-10-14T15:00:00Z"}`
	serve("POST", create, window, "most", 403)
	var game struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(serve("POST", create, window, "admin", 200)), &game); err != nil {
		t.Fatal(err)
	}
	serve("POST", create, window, "admin", 200) // re-apply is fine

	gameURL := "/games/button/" + game.ID
	serve("GET", "/games/button", "", "most", 302) // nothing open yet: back to the games list
	if strings.Contains(serve("GET", gameURL, "", "most", 200), "the-button") {
		t.Fatal("button shown before the game opens")
	}
	serve("POST", gameURL, "", "most", 409, "X-Press", "1")
	now = time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC)
	page := serve("GET", "/games/button", "", "most", 200)
	if !strings.Contains(page, `class="the-button"`) || !strings.Contains(page, "count: 0") || !strings.Contains(page, `action="`+gameURL+`"`) ||
		strings.Contains(page, "The tribe has spoken") || !strings.Contains(page, `<p class="button-hint shown" role="status" aria-live="polite">Will you press the button?</p>`) {
		t.Fatalf("button page should be the button, its count, and a hidden hint: %s", page)
	}
	hints := map[string][]string{}
	press := func(user string, n int) {
		for range n {
			var body struct {
				Count int    `json:"count"`
				Hint  string `json:"hint"`
			}
			if err := json.Unmarshal([]byte(serve("POST", gameURL, "", user, 200, "X-Press", "1")), &body); err != nil || body.Count == 0 {
				t.Fatalf("press response: %v %v", body, err)
			}
			hints[user] = append(hints[user], body.Hint)
			now = now.Add(time.Second)
		}
	}
	press("most", 9) // first presser
	if hints["most"][0] != "Get this trailblazer a machete" {
		t.Fatalf("first press hint: %q", hints["most"][0])
	}
	press("second", 6)
	press("twinA", 4)
	press("twinB", 4)
	if h := hints["twinB"][3]; h != "There's something in the air" && h != "You're not alone out here" && h != "Footprints in the sand" {
		t.Fatalf("landing on a shared count should hint at it, got %q", h)
	}
	press("least", 1)
	if page := serve("GET", "/games/button", "", "most", 200); !strings.Contains(page, "count: 9") || strings.Contains(page, "Will you press") {
		t.Fatalf("count after 9 presses: %s", page)
	}
	// second presses on two more Eastern days: a 3-day streak.
	now = time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	press("second", 1)
	now = time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC)
	press("second", 1)
	serve("POST", gameURL, "", "least", 303)                                 // plain form post redirects back; least is now the last presser
	serve("GET", gameURL, "", "idle", 200)                                   // others' counts aren't shown while it's open
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 409) // before cutoff

	now = time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)
	serve("POST", gameURL, "", "idle", 409) // closed
	if page := serve("GET", gameURL, "", "idle", 200); strings.Contains(page, "twinA") || !strings.Contains(page, "once the game is scored") {
		t.Fatalf("results shown before scoring: %s", page)
	}
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 200)
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 200)
	serve("POST", create, window, "admin", 409) // resolved games can't be rescheduled
	// Once scored, every player in the season sees everyone's results, and each player's profile lists it.
	if page := serve("GET", gameURL, "", "idle", 200); !strings.Contains(page, "twinA") || !strings.Contains(page, "first +1") || !strings.Contains(page, "last −1") {
		t.Fatalf("results page: %s", page)
	}
	if page := serve("GET", "/players/"+ids["most"], "", "idle", 200); !strings.Contains(page, `href="`+gameURL+`">Episode 3 · Press the Button`) || !strings.Contains(page, "9 presses · &#43;3 pts") {
		t.Fatalf("profile games: %s", page)
	}
	if page := serve("GET", "/games", "", "idle", 200); !strings.Contains(page, gameURL) || !strings.Contains(page, "Episode 3") {
		t.Fatalf("games list: %s", page)
	}

	totals := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT p.public_id::text, COALESCE(SUM(l.points),0) FROM participants p LEFT JOIN bonus_point_ledger_entries l ON l.participant_id = p.id GROUP BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		var points int
		if err := rows.Scan(&id, &points); err != nil {
			t.Fatal(err)
		}
		totals[id] = points
	}
	rows.Close()
	// most: most presses +2, first presser +1. second: second most -1, 3-day streak +1. twins: pair +2.
	// least: fewest +1, last presser -1. idle never pressed.
	want := map[string]int{"most": 3, "second": 0, "twinA": 2, "twinB": 2, "least": 0, "idle": 0, "admin": 0}
	for user, points := range want {
		if totals[ids[user]] != points {
			t.Errorf("%s earned %d, want %d", user, totals[ids[user]], points)
		}
	}
}
