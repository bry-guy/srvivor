package httpapi_test

import (
	"crypto/sha256"
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

// Profiles: league-only history joined by Discord account, secret points hidden, soft-closed draft reveal.
func TestProfiles(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	now := time.Date(2026, 10, 2, 16, 0, 0, 0, time.UTC)
	current := createInstanceForTest(t, ctx, q, "Season 51", 51)
	past := createInstanceForTest(t, ctx, q, "Season 50", 50)
	other := createInstanceForTest(t, ctx, q, "Season 51 (BrainLand)", 51)
	type person struct{ cur, past, other pgtype.UUID }
	people := map[string]*person{}
	link := func(inst pgtype.UUID, name, discord string) pgtype.UUID {
		p := createParticipantForTest(t, ctx, q, inst, name)
		if discord != "" {
			if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: p.ID, DiscordUserID: pgtype.Text{String: discord, Valid: true}}); err != nil {
				t.Fatal(err)
			}
		}
		return p.ID
	}
	for _, user := range []string{"alice", "bob", "carol"} {
		people[user] = &person{cur: link(current.ID, user, user)}
	}
	people["alice"].past = link(past.ID, "Alice (S50)", "alice")
	people["alice"].other = link(other.ID, "Alice BrainLand", "alice")
	outsider := link(past.ID, "Old Timer", "dave") // past league player, not in the current season

	torch := createContestantForTest(t, ctx, q, current.ID, "Torch Bearer")
	pastPick := createContestantForTest(t, ctx, q, past.ID, "Old Legend")
	createDraftPickForTest(t, ctx, q, current.ID, people["alice"].cur, torch.ID, 1)
	createDraftPickForTest(t, ctx, q, current.ID, people["bob"].cur, torch.ID, 1)
	createDraftPickForTest(t, ctx, q, past.ID, people["alice"].past, pastPick.ID, 1)
	createDraftPickForTest(t, ctx, q, past.ID, outsider, pastPick.ID, 1)

	activity := createActivityForTest(t, ctx, q, current.ID, now.Add(-48*time.Hour), nil, "draft_submission", "Draft submissions")
	occ := createOccurrenceForTest(t, ctx, q, activity.ID, "bonus", "early", now.Add(-24*time.Hour))
	createLedgerEntryForTest(t, ctx, q, current.ID, people["alice"].cur, occ.ID, pgtype.UUID{}, "award", 2, "public", "early", "early-alice")
	createLedgerEntryForTest(t, ctx, q, current.ID, people["alice"].cur, occ.ID, pgtype.UUID{}, "award", 40, "secret", "idol", "secret-alice")

	for _, user := range []string{"alice", "bob", "carol", "dave"} {
		hash := sha256.Sum256([]byte(user + "-profile"))
		if _, err := pool.Exec(ctx, `INSERT INTO web_sessions (token_hash, discord_user_id, discord_username, kind, expires_at) VALUES ($1,$2,$2,'browser',$3)`, hash[:], user, now.AddDate(0, 1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	id := func(u pgtype.UUID) string { return uuid.UUID(u.Bytes).String() }
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithPublic(httpapi.PublicConfig{
		BaseURL: "https://castaway.example", InstanceID: id(current.ID), LeagueName: "NowThisIsPodracing", LeagueInstanceIDs: []string{id(past.ID)},
	})).PublicRouter()
	get := func(path, user string, status int) string {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "castaway_session", Value: user + "-profile"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("GET %s as %s: %d, want %d: %s", path, user, rec.Code, status, rec.Body)
		}
		return rec.Body.String()
	}
	has := func(body string, want ...string) {
		t.Helper()
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Fatalf("missing %q in:\n%s", w, body)
			}
		}
	}
	lacks := func(body string, bad ...string) {
		t.Helper()
		for _, b := range bad {
			if strings.Contains(body, b) {
				t.Fatalf("unexpected %q in:\n%s", b, body)
			}
		}
	}

	me := get("/me", "alice", 200)
	has(me, "Season 51 · NowThisIsPodracing", "Me · alice", "Torch Bearer", "2 bonus", "Season 50", "Current", "/players/"+id(people["alice"].past))
	lacks(me, "BrainLand", "42<span", "40 bonus", "42 bonus")
	has(get("/", "alice", 200), `href="/players/`+id(people["bob"].cur)+`"`, `href="/me"`)

	// Drafts are soft-closed: while submissions are open, other players' current drafts stay hidden.
	bob := get("/players/"+id(people["alice"].cur), "bob", 200)
	has(bob, "Drafts are revealed after drafts close.", "alice")
	lacks(bob, "Torch Bearer", "Me ·")
	if _, err := pool.Exec(ctx, `UPDATE instance_activities SET status = 'completed' WHERE public_id = $1`, activity.ID); err != nil {
		t.Fatal(err)
	}
	has(get("/players/"+id(people["alice"].cur), "bob", 200), "Torch Bearer")
	has(get("/players/"+id(people["alice"].cur), "carol", 200), "Submit your own draft")
	has(get("/players/"+id(people["alice"].past), "carol", 200), "Old Legend", "Season 50")
	has(get("/players/"+id(people["carol"].cur), "alice", 200), "Unavailable", "No draft on record.")

	// Outside the league or the current season: not viewable, and not a player this season → no Me page.
	get("/players/"+id(people["alice"].other), "bob", 404)
	get("/players/"+id(outsider), "alice", 404)
	get("/players/not-a-uuid", "alice", 404)
	get("/me", "dave", 302)
}
