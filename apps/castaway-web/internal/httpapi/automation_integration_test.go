package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestSeasonAutomation covers the all-or-nothing episode import (replay, conflicts, legacy keys, rollback),
// weekly standings snapshots, and automated scores posts that never send stale numbers.
func TestSeasonAutomation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Automation", 957)
	other := createInstanceForTest(t, ctx, q, "Other", 958)
	id := func(u [16]byte) string { return uuid.UUID(u).String() }
	a := createParticipantForTest(t, ctx, q, instance.ID, "Alice")
	b := createParticipantForTest(t, ctx, q, instance.ID, "Bob")
	x := createContestantForTest(t, ctx, q, instance.ID, "Xena")
	y := createContestantForTest(t, ctx, q, instance.ID, "Yuri")
	z := createContestantForTest(t, ctx, q, instance.ID, "Zed")
	createDraftPickForTest(t, ctx, q, instance.ID, a.ID, x.ID, 1)
	createDraftPickForTest(t, ctx, q, instance.ID, b.ID, y.ID, 1)
	for _, inst := range []db.CreateInstanceRow{instance, other} {
		if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: inst.ID, DiscordUserID: "admin"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "8001", ChannelID: "8002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 10, 8, 23, 0, 0, 0, time.UTC)
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	path := "/instances/" + id(instance.ID.Bytes)
	call := func(method, url, body string, want int) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authorizedJSONRequest(method, url, body, "token", "admin"))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
		}
		out := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	bonus := func() map[string]float64 {
		var res struct {
			Leaderboard []struct {
				Name  string  `json:"participant_name"`
				Bonus float64 `json:"bonus_points"`
			} `json:"leaderboard"`
		}
		raw, err := json.Marshal(call("GET", path+"/leaderboard", "", 200))
		if err != nil || json.Unmarshal(raw, &res) != nil {
			t.Fatal("leaderboard decode", err)
		}
		got := map[string]float64{}
		for _, r := range res.Leaderboard {
			got[r.Name] = r.Bonus
		}
		return got
	}
	tribeStart := time.Date(2030, 10, 1, 0, 0, 0, 0, time.UTC)
	call("PUT", path+"/tribes", fmt.Sprintf(`{"effective_at":%q,"tribes":[{"name":"Savu","participant_ids":[%q]},{"name":"Toka","participant_ids":[%q]}]}`,
		tribeStart.Format(time.RFC3339), id(a.ID.Bytes), id(b.ID.Bytes)), 200)

	ep2 := "2030-10-03T01:00:00Z"
	// A manual award under the old naming scheme, e.g. `probst challenge immunity Toka --episode 2`.
	call("POST", path+"/tribe-challenges", `{"key":"ep2-immunity","kind":"immunity","winning_tribes":["Toka"],"effective_at":"`+ep2+`"}`, 201)
	body := func(boot string, rewardTribe string) string {
		return fmt.Sprintf(`{"episode_number":2,"source":"survivor","source_revision":"rev1","boots":[{"position":3,"contestant_id":%q}],
			"challenges":[{"key":"survivor-ep2-c3-immunity","legacy_keys":["ep2-immunity"],"kind":"immunity","winning_tribes":["Toka"],"effective_at":%q},
				{"key":"survivor-ep2-c2-reward","legacy_keys":["ep2-reward"],"kind":"reward","winning_tribes":[%q],"effective_at":%q}]}`, boot, ep2, rewardTribe, ep2)
	}

	// A bad tribe in the second challenge rolls back the boot and the first challenge: nothing applied.
	call("POST", path+"/episode-imports", body(id(z.ID.Bytes), "Nobody"), 400)
	if outs := call("GET", path+"/outcomes", "", 200); jlen(outs, "outcomes") != 0 {
		t.Fatalf("partial import left outcomes: %v", outs)
	}
	if got := bonus(); got["Bob"] != 2 || got["Alice"] != 0 {
		t.Fatalf("after rollback: %v", got)
	}
	// A real import reuses the manual immunity (no double award) and adds the reward and the boot.
	if res := call("POST", path+"/episode-imports", body(id(z.ID.Bytes), "Savu"), 201); res["outcomes_recorded"] != 1.0 || res["challenge_awards"] != 2.0 {
		t.Fatalf("import: %v", res)
	}
	if got := bonus(); got["Bob"] != 2 || got["Alice"] != 1 {
		t.Fatalf("after import: %v", got)
	}
	// Same results from a newer upstream revision: nothing changes. Different results: conflict.
	call("POST", path+"/episode-imports", strings.Replace(body(id(z.ID.Bytes), "Savu"), "rev1", "rev2", 1), 200)
	call("POST", path+"/episode-imports", body(id(x.ID.Bytes), "Savu"), 409)
	if got := bonus(); got["Bob"] != 2 || got["Alice"] != 1 {
		t.Fatalf("after replays: %v", got)
	}
	// A manual award that disagrees with upstream is a conflict, not an overwrite.
	call("POST", path+"/tribe-challenges", `{"key":"ep3-reward","kind":"reward","winning_tribes":["Toka"],"effective_at":"2030-10-10T01:00:00Z"}`, 201)
	call("POST", path+"/episode-imports", `{"episode_number":3,"source":"survivor","boots":[],"challenges":[{"key":"survivor-ep3-c5-reward","legacy_keys":["ep3-reward"],"kind":"reward","winning_tribes":["Savu"],"effective_at":"2030-10-10T01:00:00Z"}]}`, 409)
	// Two rewards in one episode: the import can't tell which one a hand-entered `--key` reward was, so it
	// refuses; and a hand entry can't double an imported challenge.
	call("POST", path+"/tribe-challenges", `{"key":"ep4-reward-b","kind":"reward","winning_tribes":["Savu"],"effective_at":"2030-10-17T01:00:00Z"}`, 201)
	call("POST", path+"/episode-imports", `{"episode_number":4,"source":"survivor","boots":[],"challenges":[{"key":"survivor-ep4-c7-reward","kind":"reward","winning_tribes":["Savu"],"effective_at":"2030-10-17T01:00:00Z"},{"key":"survivor-ep4-c8-reward","kind":"reward","winning_tribes":["Toka"],"effective_at":"2030-10-17T01:00:00Z"}]}`, 409)
	call("POST", path+"/episode-imports", `{"episode_number":5,"source":"survivor","boots":[],"challenges":[{"key":"survivor-ep5-c9-reward","kind":"reward","winning_tribes":["Savu"],"effective_at":"2030-10-24T01:00:00Z"},{"key":"survivor-ep5-c10-reward","kind":"reward","winning_tribes":["Toka"],"effective_at":"2030-10-24T01:00:00Z"}]}`, 201)
	call("POST", path+"/tribe-challenges", `{"key":"ep5-reward","kind":"reward","winning_tribes":["Savu"],"effective_at":"2030-10-24T01:00:00Z"}`, 409)
	imports := call("GET", path+"/episode-imports", "", 200)
	if jlen(imports, "imports") != 2 || jstr(imports, "imports", 0, "source_revision") != "rev1" {
		t.Fatalf("imports: %v", imports)
	}
	// Another season sees none of it.
	if got := call("GET", "/instances/"+id(other.ID.Bytes)+"/episode-imports", "", 200); jlen(got, "imports") != 0 {
		t.Fatal("cross-instance import leak")
	}

	// Snapshots and automated scores posts.
	snap := call("PUT", path+"/score-snapshots/2", `{"request_key":"s957-week-2-scores"}`, 200)
	fp := jstr(snap, "fingerprint")
	post := func(text, fingerprint string, sendAt time.Time, want int) string {
		t.Helper()
		res := call("PUT", path+"/score-posts/s957-week-2-scores", fmt.Sprintf(`{"guild_id":"8001","channel_id":"8002","body":%q,"send_at":%q,"score_fingerprint":%q,"notify_users":true}`,
			text, sendAt.Format(time.RFC3339), fingerprint), want)
		return jstr(res, "action")
	}
	sendAt := now.Add(-30 * time.Minute) // drafted after its 8pm time: allowed within 3 hours
	post("Week 2", fp, now.Add(-4*time.Hour), 409)
	post("Week 2", "stale", sendAt, 409)
	if got := post("Week 2: Bob leads", fp, sendAt, 200); got != "created" {
		t.Fatal(got)
	}
	if got := post("Week 2: Bob leads", fp, sendAt, 200); got != "unchanged" {
		t.Fatal(got)
	}
	approval := call("POST", "/announcements/approvals/claim", `{"guild_ids":["8001"]}`, 200)
	annID, revision := jstr(approval, "announcement", "id"), jstr(approval, "revision")
	call("POST", "/announcements/"+annID+"/approval-delivered", fmt.Sprintf(`{"revision":%q}`, revision), 200)
	call("POST", "/announcements/"+annID+"/approve", fmt.Sprintf(`{"admin_discord_user_id":"admin","revision":%q}`, revision), 200)

	// Scores change after approval: the sender refuses it and holds it again.
	call("POST", path+"/tribe-challenges", `{"key":"ep2-extra","kind":"immunity","winning_tribes":["Savu"],"effective_at":"2030-10-03T02:00:00Z"}`, 201)
	if got := call("POST", "/announcements/claim", `{"guild_ids":["8001"]}`, 200); got["announcement"] != nil {
		t.Fatal("stale approved scores post was sent")
	}
	// Re-snapshot and redraft: the untouched draft is replaced and re-asked.
	fp2 := jstr(call("PUT", path+"/score-snapshots/2", `{"request_key":"s957-week-2-scores"}`, 200), "fingerprint")
	if fp2 == fp {
		t.Fatal("fingerprint did not change with scores")
	}
	if got := post("Week 2: tied!", fp2, sendAt, 200); got != "regenerated" {
		t.Fatal(got)
	}
	approval = call("POST", "/announcements/approvals/claim", `{"guild_ids":["8001"]}`, 200)
	if jstr(approval, "announcement", "body") != "Week 2: tied!" {
		t.Fatalf("re-ask: %v", approval)
	}
	call("POST", "/announcements/"+annID+"/approval-delivered", fmt.Sprintf(`{"revision":%q}`, approval["revision"]), 200)
	// An admin edits the copy by DM; later score changes keep the edit and alert the admins.
	call("POST", "/announcements/"+annID+"/revise", fmt.Sprintf(`{"admin_discord_user_id":"admin","revision":%q,"body":"Admin words, tied!"}`, approval["revision"]), 200)
	call("POST", path+"/tribe-challenges", `{"key":"ep2-extra2","kind":"immunity","winning_tribes":["Toka"],"effective_at":"2030-10-03T02:00:00Z"}`, 201)
	fp3 := jstr(call("PUT", path+"/score-snapshots/2", `{"request_key":"s957-week-2-scores"}`, 200), "fingerprint")
	editedRevision := jstr(call("POST", "/announcements/approvals/claim", `{"guild_ids":["8001"]}`, 200), "revision")
	if got := post("Week 2: regenerated", fp3, sendAt, 200); got != "rehold-edited" {
		t.Fatal(got)
	}
	// "yes" to the DM sent before the rescore no longer approves it.
	call("POST", "/announcements/"+annID+"/approve", fmt.Sprintf(`{"admin_discord_user_id":"admin","revision":%q}`, editedRevision), 409)
	approval = call("POST", "/announcements/approvals/claim", `{"guild_ids":["8001"]}`, 200)
	if jstr(approval, "announcement", "body") != "Admin words, tied!" {
		t.Fatalf("admin edit lost: %v", approval)
	}
	alert := call("POST", "/admin-alerts/claim", `{"guild_ids":["8001"]}`, 200)
	if !strings.Contains(jstr(alert, "alert", "body"), "Scores changed after you edited") {
		t.Fatalf("no rescored alert: %v", alert)
	}
	// Approve and send; afterwards the snapshot is frozen and the post is never touched again.
	call("POST", "/announcements/"+annID+"/approval-delivered", fmt.Sprintf(`{"revision":%q}`, approval["revision"]), 200)
	call("POST", "/announcements/"+annID+"/approve", fmt.Sprintf(`{"admin_discord_user_id":"admin","revision":%q}`, approval["revision"]), 200)
	sent := call("POST", "/announcements/claim", `{"guild_ids":["8001"]}`, 200)
	if sent["announcement"] == nil {
		t.Fatal("current approved post did not send")
	}
	call("POST", "/announcements/"+annID+"/finish", `{"message_id":"123456"}`, 200)
	call("POST", path+"/tribe-challenges", `{"key":"ep2-extra3","kind":"immunity","winning_tribes":["Toka"],"effective_at":"2030-10-03T02:00:00Z"}`, 201)
	frozen := call("PUT", path+"/score-snapshots/2", `{"request_key":"s957-week-2-scores"}`, 200)
	if frozen["fingerprint"] != fp3 || frozen["frozen"] != true {
		t.Fatalf("snapshot moved after send: %v", frozen)
	}
	if got := post("Week 2 again", fp3, sendAt, 409); got != "" {
		t.Fatal("stale fingerprint accepted")
	}
}

// jget walks decoded JSON by keys (string) and indexes (int); missing paths give nil.
func jget(v any, path ...any) any {
	for _, k := range path {
		switch k := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[k]
		case int:
			l, ok := v.([]any)
			if !ok || k >= len(l) {
				return nil
			}
			v = l[k]
		}
	}
	return v
}

func jstr(v any, path ...any) string {
	if s, ok := jget(v, path...).(string); ok {
		return s
	}
	return ""
}

func jlen(v any, path ...any) int {
	if l, ok := jget(v, path...).([]any); ok {
		return len(l)
	}
	return 0
}
