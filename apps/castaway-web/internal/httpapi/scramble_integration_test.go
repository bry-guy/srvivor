package httpapi_test

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Island Scramble: the phrase never reaches players before they solve or the game closes, the clock runs from
// Start, checks only say right or wrong, the fastest three score +3/+2/+1 once, and tests never score.
func TestSpellItOut(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Scramble season", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	now := time.Date(2026, 10, 14, 23, 0, 0, 0, time.UTC)
	ids := map[string]string{}
	for _, user := range []string{"admin", "fast", "clean", "messy", "slow", "late", "idle"} {
		player := createParticipantForTest(t, ctx, q, instance.ID, user)
		ids[user] = uuid.UUID(player.ID.Bytes).String()
		if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: player.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []string{"admin", "fast", "clean", "messy", "slow", "late", "idle", "outsider"} {
		hash := sha256.Sum256([]byte(user + "-scramble"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	const base = "https://castaway.example"
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID})).PublicRouter()
	serve := func(method, path, body, user string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-scramble"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: HTTP %d, want %d: %s", method, path, user, rec.Code, status, rec.Body)
		}
		return rec.Body.String()
	}
	create := "/api/instances/" + instanceID + "/scramble-games"
	window := `{"episode_number":4,"phrase":"the tribe has spoken","decoy_letters":"QZXJ","opens_at":"2026-10-15T00:00:00Z","cutoff_at":"2026-10-21T15:59:00Z"}`
	serve("POST", create, window, "fast", 403)
	serve("POST", create, `{"episode_number":4,"phrase":"let's go","opens_at":"2026-10-15T00:00:00Z","cutoff_at":"2026-10-21T15:59:00Z"}`, "admin", 400)
	var game struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(serve("POST", create, window, "admin", 200)), &game); err != nil {
		t.Fatal(err)
	}
	serve("POST", create, window, "admin", 200) // re-apply before anyone starts is fine
	gameURL := "/games/scramble/" + game.ID
	noPhrase := func(page string) {
		t.Helper()
		if strings.Contains(page, "SPOKEN") || strings.Contains(page, "TRIBE") {
			t.Fatalf("phrase leaked: %s", page)
		}
	}

	noPhrase(serve("GET", gameURL, "", "fast", 200)) // upcoming
	serve("POST", gameURL+"/start", "", "fast", 409)
	now = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	page := serve("GET", "/games/scramble", "", "fast", 200)
	noPhrase(page)
	if !strings.Contains(page, "Your clock starts when you reveal the tiles") || strings.Contains(page, "scramble-tile") || strings.Count(page, `class="scramble-slot"`) != 17 {
		t.Fatalf("before Start: empty slots only: %s", page)
	}
	serve("GET", gameURL, "", "outsider", 200)                                 // not a player: access-request page
	serve("POST", gameURL+"/start", "", "outsider", 409)                       // and can't play
	serve("POST", gameURL+"/check", `{"guess":"X"}`, "fast", 409)              // must Start first
	serve("POST", "/api/scramble-games/"+game.ID+"/resolve", "", "admin", 409) // before cutoff

	tile := regexp.MustCompile(`data-tile="\d+">([A-Z])<`)
	start := func(user string) {
		t.Helper()
		serve("POST", gameURL+"/start", "", user, 303)
		page := serve("GET", gameURL, "", user, 200)
		noPhrase(page)
		var tray []string
		for _, m := range tile.FindAllStringSubmatch(page, -1) {
			tray = append(tray, m[1])
		}
		if strings.Join(sortedLetters(tray), "") != "ABEEEHHIJKNOPQRSSTTXZ" {
			t.Fatalf("%s's tray %v", user, tray)
		}
	}
	check := func(user, guess string, status int) map[string]any {
		t.Helper()
		var body map[string]any
		if err := json.Unmarshal([]byte(serve("POST", gameURL+"/check", `{"guess":"`+guess+`"}`, user, status)), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	const answer = "THETRIBEHASSPOKEN"
	const wrong = "THETRIBEHASSPOKNE"
	start("fast")
	start("clean")
	start("messy")
	start("slow")
	start("late")
	serve("POST", gameURL+"/start", "", "fast", 303) // starting again doesn't restart the clock
	check("fast", "THETRIBE", 400)                   // incomplete: no wrong check
	check("fast", "THETRIBEHASSPOKEQ", 200)          // a decoy: wrong
	check("fast", "AAAAAAAAAAAAAAAAA", 400)          // not your tiles
	now = now.Add(2 * time.Minute)
	if body := check("fast", answer, 200); body["solved"] != true || body["time"] != "2:00" || body["wrong_checks"] != float64(1) {
		t.Fatalf("fast solve: %v", body)
	}
	if body := check("fast", wrong, 200); body["solved"] != true { // after solving, checks change nothing
		t.Fatalf("re-check: %v", body)
	}
	if body := check("clean", answer, 200); body["time"] != "2:00" || body["wrong_checks"] != float64(0) {
		t.Fatalf("clean solve: %v", body) // same time as fast, fewer wrong checks: first
	}
	now = now.Add(time.Minute)
	check("messy", wrong, 200)
	check("messy", answer, 200) // 3:00, third
	now = now.Add(time.Hour)
	check("slow", answer, 200) // fourth: no points
	if page := serve("GET", gameURL, "", "fast", 200); !strings.Contains(page, "Solved in 2:00") || !strings.Contains(page, ">S<") {
		t.Fatalf("solved page: %s", page)
	}
	serve("POST", create, window, "admin", 409) // started games can't be replaced

	now = time.Date(2026, 10, 21, 15, 59, 0, 0, time.UTC)
	check("late", answer, 409) // closed
	if page := serve("GET", gameURL, "", "idle", 200); !strings.Contains(page, "once the game is scored") || strings.Contains(page, "SPOKEN") {
		t.Fatalf("closed before scoring: %s", page)
	}
	serve("POST", "/api/scramble-games/"+game.ID+"/resolve", "", "fast", 403)
	serve("POST", "/api/scramble-games/"+game.ID+"/resolve", "", "admin", 200)
	serve("POST", "/api/scramble-games/"+game.ID+"/resolve", "", "admin", 200) // once
	page = serve("GET", gameURL, "", "idle", 200)
	if !strings.Contains(page, "THE TRIBE HAS SPOKEN") || !strings.Contains(page, ">late</a>") || !strings.Contains(page, "Unsolved") {
		t.Fatalf("results page: %s", page)
	}
	if page := serve("GET", "/players/"+ids["clean"], "", "idle", 200); !strings.Contains(page, "Episode 4 · Island Scramble") || !strings.Contains(page, "Solved in 2:00 · &#43;3 pts") {
		t.Fatalf("profile: %s", page)
	}

	// An admin-only test: hidden from players, and it never scores.
	testURL := ""
	{
		req := httptest.NewRequest("POST", "/games/scramble/tests", strings.NewReader("phrase=fire+represents+life&decoys=3"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", base)
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: "admin-scramble"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("create test: %d %s", rec.Code, rec.Body)
		}
		testURL = rec.Header().Get("Location")
	}
	serve("GET", testURL, "", "fast", 404)
	if strings.Contains(serve("GET", "/games", "", "fast", 200), testURL) {
		t.Fatal("players see the test game")
	}
	serve("POST", testURL+"/start", "", "admin", 303)
	serve("POST", testURL+"/check", `{"guess":"FIREREPRESENTSLIFE"}`, "admin", 200)
	now = now.Add(2 * time.Hour)
	if err := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now })).ResolveDueGames(ctx); err != nil {
		t.Fatal(err)
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
	want := map[string]int{"clean": 3, "fast": 2, "messy": 1, "slow": 0, "late": 0, "idle": 0, "admin": 0}
	for user, points := range want {
		if totals[ids[user]] != points {
			t.Errorf("%s earned %d, want %d", user, totals[ids[user]], points)
		}
	}
}

func sortedLetters(letters []string) []string {
	out := append([]string(nil), letters...)
	sort.Strings(out)
	return out
}
