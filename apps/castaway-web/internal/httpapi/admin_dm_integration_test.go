package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Admin DM actions: an approval-gated post waits for an admin's "yes"; an admin's corrected draft saves as
// the author's draft, and non-admins can do neither.
func TestAdminDMActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Admin DMs", 954)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	path := "/instances/" + instanceID
	for _, name := range []string{"Ann Alpha", "Thien \"An\" Nguyen", "Cyd Cole"} {
		createContestantForTest(t, ctx, queries, instance.ID, name)
	}
	player := createParticipantForTest(t, ctx, queries, instance.ID, "P1")
	if _, err := pool.Exec(ctx, `UPDATE participants SET discord_user_id = '1000' WHERE public_id = $1`, player.ID); err != nil {
		t.Fatal(err)
	}
	for _, admin := range []string{"2001", "2002"} {
		if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: admin}); err != nil {
			t.Fatal(err)
		}
	}
	if err := queries.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "7001", ChannelID: "7002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 4, 0, 0, 0, time.UTC)
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	type reply struct {
		Announcement *struct {
			ID string `json:"id"`
		} `json:"announcement"`
		Revision    string   `json:"revision"`
		Admins      []string `json:"admin_discord_user_ids"`
		ScheduledAt string   `json:"scheduled_at"`
		Status      string   `json:"status"`
	}
	call := func(method, url, body, actor string, want int) reply {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedJSONRequest(method, url, body, "token", actor))
		if recorder.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, recorder.Code, recorder.Body.String())
		}
		var out reply
		if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	// Approval gate.
	created := call(http.MethodPost, path+"/announcements", `{"guild_id":"7001","channel_id":"7002","request_key":"w2","body":"Week 2 scores","draft":true}`, "2001", http.StatusOK)
	id := created.Announcement.ID
	call(http.MethodPut, path+"/announcements/"+id+"/approval", `{"send_at":"2026-10-07T16:00:00Z"}`, "2001", http.StatusOK)
	claimPath, claimBody := "/announcements/approvals/claim", `{"guild_ids":["7001"]}`
	claim := call(http.MethodPost, claimPath, claimBody, "", http.StatusOK)
	if claim.Announcement == nil || fmt.Sprint(claim.Admins) != "[2001 2002]" {
		t.Fatalf("approval claim: %v", claim)
	}
	oldRevision := claim.Revision
	if again := call(http.MethodPost, claimPath, claimBody, "", http.StatusOK); again.Announcement != nil {
		t.Fatalf("approval DM went out twice: %v", again)
	}
	if got := call(http.MethodPost, "/announcements/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK); got.Announcement != nil {
		t.Fatal("an unapproved post was claimed for sending")
	}
	call(http.MethodPut, path+"/announcements/"+id+"/body", `{"body":"Week 2 scores, fixed"}`, "2001", http.StatusOK)
	again := call(http.MethodPost, claimPath, claimBody, "", http.StatusOK)
	if again.Announcement == nil || again.Revision == oldRevision {
		t.Fatalf("edited copy should be re-sent for approval as a new revision: %v", again)
	}
	revision := again.Revision
	approve := func(admin, rev string, want int) reply {
		return call(http.MethodPost, "/announcements/"+id+"/approve", fmt.Sprintf(`{"admin_discord_user_id":%q,"revision":%q}`, admin, rev), "", want)
	}
	approve("2002", oldRevision, http.StatusConflict) // a reply to the old DM can't approve the new text
	approve("1000", revision, http.StatusConflict)    // not an admin
	if approved := approve("2002", revision, http.StatusOK); approved.ScheduledAt != "2026-10-07T16:00:00Z" {
		t.Fatalf("approved: %v", approved)
	}
	approve("2001", revision, http.StatusConflict) // once

	// Editing after approval holds it again and re-asks.
	call(http.MethodPut, path+"/announcements/"+id+"/body", `{"body":"Week 2 scores, fixed again"}`, "2001", http.StatusOK)
	if got := call(http.MethodPost, "/announcements/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK); got.Announcement != nil {
		t.Fatal("an edited, unapproved post was claimed for sending")
	}
	revision = call(http.MethodPost, claimPath, claimBody, "", http.StatusOK).Revision
	approve("2001", revision, http.StatusOK)
	if got := call(http.MethodPost, "/announcements/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK); got.Announcement != nil {
		t.Fatal("approved post sent before its time")
	}
	now = time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC)
	if got := call(http.MethodPost, "/announcements/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK); got.Announcement == nil {
		t.Fatal("approved post didn't send at its time")
	}

	// Approval is only a gate: past its time it can't be approved and is never claimed.
	late := call(http.MethodPost, path+"/announcements", `{"guild_id":"7001","channel_id":"7002","request_key":"late","body":"Late post","draft":true}`, "2001", http.StatusOK)
	lateID := late.Announcement.ID
	call(http.MethodPut, path+"/announcements/"+lateID+"/approval", `{"send_at":"2026-10-07T17:00:00Z"}`, "2001", http.StatusOK)
	lateRevision := call(http.MethodPost, claimPath, claimBody, "", http.StatusOK).Revision
	now = time.Date(2026, 10, 7, 17, 1, 0, 0, time.UTC)
	call(http.MethodPost, "/announcements/"+lateID+"/approve", fmt.Sprintf(`{"admin_discord_user_id":"2001","revision":%q}`, lateRevision), "", http.StatusConflict)
	if got := call(http.MethodPost, "/announcements/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK); got.Announcement != nil {
		t.Fatal("a late, unapproved post was sent")
	}

	// Admin draft fixes.
	call(http.MethodPost, path+"/draft-submissions", `{"tribes":["Savu","Toka"],"guild_id":"7001","channel_id":"7002"}`, "2001", http.StatusCreated)
	call(http.MethodPut, path+"/draft-submissions/thread", `{"thread_id":"8001"}`, "2001", http.StatusOK)
	post := func(version, content, fixedBy string, want int) reply {
		body, err := json.Marshal(map[string]string{"message_id": "9001", "version": version, "author_discord_user_id": "1000", "content": content, "fixed_by_discord_user_id": fixedBy})
		if err != nil {
			t.Fatal(err)
		}
		return call(http.MethodPost, "/draft-threads/8001/messages", string(body), "", want)
	}
	problem := post("v1", "1. Ann\n2. Ann\n3. Cyd", "", http.StatusOK)
	if problem.Status != "problem" || fmt.Sprint(problem.Admins) != "[2001 2002]" {
		t.Fatalf("problem: %v", problem)
	}
	post("fix-1", "1. Ann\n2. Thien An\n3. Cyd", "1000", http.StatusForbidden) // the player can't "fix" as admin
	if saved := post("fix-2", "1. Ann\n2. Thien An\n3. Cyd", "2002", http.StatusOK); saved.Status != "saved" {
		t.Fatalf("admin fix: %v", saved)
	}
	post("fix-3", "1. Cyd\n2. Thien An\n3. Ann", "2001", http.StatusConflict) // a second admin's stale fix doesn't overwrite
	var picks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM draft_picks d JOIN participants p ON p.id = d.participant_id WHERE p.public_id = $1`, player.ID).Scan(&picks); err != nil || picks != 3 {
		t.Fatalf("saved picks = %d (%v), want 3", picks, err)
	}
}
