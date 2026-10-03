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

	if strings.Contains(serve("GET", "/button", "", "most", 200), "the-button") {
		t.Fatal("button shown before the game opens")
	}
	serve("POST", "/button", "", "most", 409, "X-Press", "1")
	now = time.Date(2026, 10, 8, 1, 0, 0, 0, time.UTC)
	page := serve("GET", "/button", "", "most", 200)
	if !strings.Contains(page, `class="the-button"`) || strings.Contains(page, "<p") {
		t.Fatalf("button page should be just the button: %s", page)
	}
	press := func(user string, n int) {
		for range n {
			serve("POST", "/button", "", user, 204, "X-Press", "1")
		}
	}
	press("most", 9)
	press("second", 6)
	press("twinA", 4)
	press("twinB", 4)
	press("least", 1)
	serve("POST", "/button", "", "least", 303)                               // plain form post redirects back
	press("least", 0)                                                        // least now has 2 presses
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 409) // before cutoff

	now = time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)
	serve("POST", "/button", "", "idle", 409) // closed
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 200)
	serve("POST", "/api/button-games/"+game.ID+"/resolve", "", "admin", 200)
	serve("POST", create, window, "admin", 409) // resolved games can't be rescheduled

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
	want := map[string]int{"most": 2, "second": -1, "twinA": 2, "twinB": 2, "least": 1, "idle": 0, "admin": 0}
	for user, points := range want {
		if totals[ids[user]] != points {
			t.Errorf("%s earned %d, want %d", user, totals[ids[user]], points)
		}
	}
}
