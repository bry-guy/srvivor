package httpapi_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

// A private Castawordle is visible only to its player and replaces their result in the original's round;
// Press the Button test games are admin-only and never score.
func TestPrivateCastawordleAndButtonTests(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Private puzzles", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, instance.ID, 51); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	ids := map[string]string{}
	for _, user := range []string{"bryan", "other", "admin2", "player"} {
		participant := createParticipantForTest(t, ctx, q, instance.ID, user)
		ids[user] = uuid.UUID(participant.ID.Bytes).String()
		if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: participant.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user + "-private"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	for _, admin := range []string{"bryan", "admin2"} {
		if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: admin}); err != nil {
			t.Fatal(err)
		}
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
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-private"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: HTTP %d, want %d: %s", method, path, user, rec.Code, status, rec.Body)
		}
		if rec.Code == http.StatusSeeOther {
			return rec.Header().Get("Location")
		}
		return rec.Body.String()
	}
	serve("PUT", "/api/instances/"+instanceID+"/tribes", fmt.Sprintf(`{"effective_at":"2026-09-24T00:01:00Z","tribes":[{"name":"Savu","participant_ids":[%q,%q]},{"name":"Toka","participant_ids":[%q,%q]}]}`,
		ids["bryan"], ids["other"], ids["admin2"], ids["player"]), "bryan", 200)

	var game, mine struct {
		ID string `json:"id"`
	}
	created := serve("POST", "/api/instances/"+instanceID+"/castawordle", `{"name":"Episode 2","answer":"SCHISM","episode_number":2,"scored":true,"window":{"opens_at":"2026-10-01T00:00:00Z","cutoff_at":"2026-10-07T15:59:00Z"}}`, "bryan", 201)
	if err := json.Unmarshal([]byte(created), &game); err != nil {
		t.Fatal(err)
	}
	private := "/api/castawordle/" + game.ID + "/private"
	serve("POST", private, fmt.Sprintf(`{"participant_id":%q,"name":"Bryan's puzzle","answer":"GARDEN"}`, ids["bryan"]), "player", 403)
	serve("POST", "/api/castawordle/"+game.ID+"/play/guesses", `{"guess":"CRANES","position":1}`, "player", 200)
	serve("POST", private, fmt.Sprintf(`{"participant_id":%q,"name":"x","answer":"GARDEN"}`, ids["player"]), "bryan", 409) // already started
	body := serve("POST", private, fmt.Sprintf(`{"participant_id":%q,"name":"Bryan's puzzle","answer":"GARDEN"}`, ids["bryan"]), "bryan", 201)
	if strings.Contains(body, "GARDEN") {
		t.Fatalf("answer leaked: %s", body)
	}
	if err := json.Unmarshal([]byte(body), &mine); err != nil {
		t.Fatal(err)
	}
	serve("POST", private, fmt.Sprintf(`{"participant_id":%q,"name":"again","answer":"GARDEN"}`, ids["bryan"]), "bryan", 409)

	// Only Bryan sees his puzzle; he no longer sees the original. Other admins can't see or play his.
	list := serve("GET", "/games", "", "bryan", 200)
	if !strings.Contains(list, mine.ID) || strings.Contains(list, game.ID) || !strings.Contains(list, "Just for you") {
		t.Fatalf("bryan's list: %s", list)
	}
	for _, user := range []string{"admin2", "player"} {
		if l := serve("GET", "/games", "", user, 200); strings.Contains(l, mine.ID) || !strings.Contains(l, game.ID) {
			t.Fatalf("%s's list: %s", user, l)
		}
		serve("GET", "/games/castawordle/"+mine.ID, "", user, 404)
		serve("GET", "/api/castawordle/"+mine.ID+"/play", "", user, 404)
		serve("POST", "/api/castawordle/"+mine.ID+"/play/guesses", `{"guess":"CRANES","position":1}`, user, 404)
	}
	serve("GET", "/api/castawordle/"+game.ID+"/play", "", "bryan", 404)
	if page := serve("GET", "/games/castawordle/"+mine.ID, "", "bryan", 200); !strings.Contains(page, "counts as your Episode 2") {
		t.Fatalf("bryan's puzzle page: %s", page)
	}
	minePlay := "/api/castawordle/" + mine.ID + "/play/guesses"
	serve("POST", minePlay, `{"guess":"CRANES","position":1}`, "bryan", 200)
	serve("POST", minePlay, `{"guess":"GARDEN","position":2}`, "bryan", 200)
	serve("POST", "/api/castawordle/"+game.ID+"/play/guesses", `{"guess":"SCHISM","position":1}`, "admin2", 200)
	for i := 2; i <= 6; i++ {
		serve("POST", "/api/castawordle/"+game.ID+"/play/guesses", fmt.Sprintf(`{"guess":"CRANES","position":%d}`, i), "player", 200)
	}

	// Press the Button tests: admin-only, a stable URL, and no points.
	serve("POST", "/games/button/tests", "", "player", 404)
	testURL := serve("POST", "/games/button/tests", "", "admin2", 303)
	if !strings.HasPrefix(testURL, "/games/button/") {
		t.Fatalf("test game redirect: %q", testURL)
	}
	serve("GET", testURL, "", "player", 404)
	serve("POST", testURL, "", "player", 404, "X-Press", "1")
	if page := serve("GET", testURL, "", "bryan", 200); !strings.Contains(page, `action="`+testURL+`"`) {
		t.Fatalf("test page should post to itself: %s", page)
	}
	for range 3 {
		serve("POST", testURL, "", "bryan", 200, "X-Press", "1")
	}
	serve("POST", testURL, "", "admin2", 200, "X-Press", "1")
	serve("GET", "/games/button", "", "bryan", 302) // the test game isn't the scheduled one

	now = time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC)
	serve("POST", "/api/castawordle/"+game.ID+"/resolve", "", "admin2", 200)
	// After close, the season's players see the answer and everyone's result (Bryan's private result stands in).
	if page := serve("GET", "/games/castawordle/"+game.ID, "", "admin2", 200); !strings.Contains(page, "SCHISM") || !strings.Contains(page, "Torch out") || !strings.Contains(page, ">bryan<") {
		t.Fatalf("closed castawordle results: %s", page)
	}
	if _, err := pool.Exec(ctx, `UPDATE button_games SET cutoff_at = $1, opens_at = $2`, now.Add(-time.Minute), now.Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var buttonID string
	if err := pool.QueryRow(ctx, `SELECT public_id::text FROM button_games`).Scan(&buttonID); err != nil {
		t.Fatal(err)
	}
	serve("POST", "/api/button-games/"+buttonID+"/resolve", "", "bryan", 200)

	totals := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT p.public_id::text, l.reason, l.points FROM bonus_point_ledger_entries l JOIN participants p ON p.id = l.participant_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, reason string
		var points int
		if err := rows.Scan(&id, &reason, &points); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(reason, "Button") {
			t.Errorf("test button game scored: %s %d", reason, points)
		}
		totals[id] += points
	}
	rows.Close()
	// admin2 solved the original in 1 (best individual, +2); Bryan solved his own in 2. Savu averages 2
	// (Bryan) against Toka's 4 (admin2 1, player exhausted 7), so Bryan's private result wins Savu +1.
	var bryanResults int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity_occurrence_participants op JOIN participants p ON p.id = op.participant_id WHERE p.public_id = $1`, ids["bryan"]).Scan(&bryanResults); err != nil {
		t.Fatal(err)
	}
	if bryanResults != 1 {
		t.Errorf("bryan has %d round results, want exactly 1", bryanResults)
	}
	want := map[string]int{"admin2": 2, "bryan": 1, "other": 0, "player": 0}
	for user, points := range want {
		if totals[ids[user]] != points {
			t.Errorf("%s earned %d, want %d (all %v)", user, totals[ids[user]], points, totals)
		}
	}
}
