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

// Scored Castawordle: only completed games count, awards land only after cutoff, and resolution is idempotent.
func TestCastawordleScored(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Scored puzzles", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, instance.ID, 51); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 23, 0, 0, 0, time.UTC)
	ids := map[string]string{}
	for _, user := range []string{"admin", "partial", "failer", "idle"} {
		player := createParticipantForTest(t, ctx, q, instance.ID, user)
		ids[user] = uuid.UUID(player.ID.Bytes).String()
		if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: player.ID, DiscordUserID: pgtype.Text{String: user, Valid: true}}); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(user + "-scored"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	const base = "https://castaway.example"
	server := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{BaseURL: base, InstanceID: instanceID}))
	router := server.PublicRouter()
	serve := func(method, path, body, user string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-scored"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s as %s: HTTP %d, want %d: %s", method, path, user, rec.Code, status, rec.Body)
		}
		return rec.Body.String()
	}
	serve("PUT", "/api/instances/"+instanceID+"/tribes", fmt.Sprintf(`{"effective_at":"2026-09-24T00:01:00Z","tribes":[{"name":"Savu","participant_ids":[%q,%q,%q]},{"name":"Toka","participant_ids":[%q]}]}`,
		ids["admin"], ids["partial"], ids["idle"], ids["failer"]), "admin", 200)

	createPath := "/api/instances/" + instanceID + "/castawordle"
	body := `{"name":"Episode 2","answer":"SCHISM","episode_number":2,"scored":true,"window":{"opens_at":"2026-10-01T00:00:00Z","cutoff_at":"2026-10-07T15:59:00Z"}}`
	created := serve("POST", createPath, body, "admin", 201)
	if strings.Contains(created, "SCHISM") || !strings.Contains(created, `"unscored":false`) {
		t.Fatalf("scored creation response: %s", created)
	}
	var game struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(created), &game); err != nil {
		t.Fatal(err)
	}
	serve("POST", createPath, body, "admin", 200) // identical re-apply is a no-op
	serve("POST", createPath, strings.Replace(body, "15:59", "15:58", 1), "admin", 409)
	serve("POST", createPath, `{"name":"x","answer":"SCHISM","episode_number":3,"scored":true,"window":{"opens_at":"2026-09-30T00:00:00Z","cutoff_at":"2026-10-14T15:59:00Z"}}`, "admin", 400)

	play := "/api/castawordle/" + game.ID + "/play"
	serve("POST", play+"/guesses", `{"guess":"CRANES","position":1}`, "admin", 409) // not open yet
	now = time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	serve("POST", play+"/guesses", `{"guess":"CRANES","position":1}`, "admin", 200)
	serve("POST", play+"/guesses", `{"guess":"SCHISM","position":2}`, "admin", 200)
	serve("POST", play+"/guesses", `{"guess":"CRANES","position":1}`, "partial", 200)
	for i := 1; i <= 6; i++ {
		serve("POST", play+"/guesses", fmt.Sprintf(`{"guess":"CRANES","position":%d}`, i), "failer", 200)
	}
	resolve := "/api/castawordle/" + game.ID + "/resolve"
	serve("POST", resolve, "", "admin", 409) // before cutoff
	serve("POST", resolve, "", "partial", 403)

	if err := server.ResolveDueGames(ctx); err != nil { // before cutoff: nothing to score
		t.Fatal(err)
	}
	now = time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC)
	if err := server.ResolveDueGames(ctx); err != nil { // the per-minute resolver scores it at cutoff
		t.Fatal(err)
	}
	first := serve("POST", resolve, "", "admin", 200)
	if second := serve("POST", resolve, "", "admin", 200); second != first {
		t.Fatalf("repeat resolution changed: %s vs %s", first, second)
	}
	totals := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT p.public_id::text, COALESCE(SUM(l.points),0) FROM participants p LEFT JOIN bonus_point_ledger_entries l ON l.participant_id = p.id JOIN instances i ON i.id = p.instance_id WHERE i.public_id = $1 GROUP BY 1`, instance.ID)
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
	// admin: best individual +2 and Savu best average +1; partial/idle didn't finish so earn 0 despite Savu winning.
	want := map[string]int{"admin": 3, "partial": 0, "idle": 0, "failer": 0}
	for user, points := range want {
		if totals[ids[user]] != points {
			t.Errorf("%s earned %d, want %d (all: %v)", user, totals[ids[user]], points, totals)
		}
	}
}
