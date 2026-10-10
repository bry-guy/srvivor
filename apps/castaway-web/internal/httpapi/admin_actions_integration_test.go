package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TestAdminActionApproval covers an episode import proposed to admins: approval runs exactly the stored
// import once, a re-proposal with different results voids the older "yes", and only that season's admins
// can approve.
func TestAdminActionApproval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Actions", 961)
	other := createInstanceForTest(t, ctx, q, "Elsewhere", 962)
	id := func(u [16]byte) string { return uuid.UUID(u).String() }
	createParticipantForTest(t, ctx, q, instance.ID, "Alice")
	x := createContestantForTest(t, ctx, q, instance.ID, "Xena")
	y := createContestantForTest(t, ctx, q, instance.ID, "Yuri")
	createContestantForTest(t, ctx, q, instance.ID, "Zed")
	for inst, admin := range map[db.CreateInstanceRow]string{instance: "admin", other: "outsider"} {
		if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: inst.ID, DiscordUserID: admin}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "8101", ChannelID: "8102", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 10, 8, 23, 0, 0, 0, time.UTC)
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	path := "/instances/" + id(instance.ID.Bytes)
	call := func(method, url, body, actor string, want int) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authorizedJSONRequest(method, url, body, "token", actor))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, rec.Code, rec.Body.String())
		}
		out := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	// Two people left: Xena (voted out, 3rd) and Yuri (quit, 2nd).
	propose := func(yuriPlace int) {
		t.Helper()
		imp := fmt.Sprintf(`{"episode_number":3,"source":"survivor","source_revision":"r1","boots":[{"position":3,"contestant_id":%q},{"position":%d,"contestant_id":%q}],"challenges":[]}`,
			id(x.ID.Bytes), yuriPlace, id(y.ID.Bytes))
		call("PUT", path+"/admin-alerts/s1-ep3-import", `{"body":"Episode 3: two people left","action":{"kind":"episode_import","payload":`+imp+`}}`, "admin", 200)
	}
	claim := func() (string, string) {
		t.Helper()
		got := call("POST", "/admin-alerts/claim", `{"guild_ids":["8101"]}`, "", 200)
		alert, ok := got["alert"].(map[string]any)
		if !ok {
			t.Fatal("no alert to claim")
		}
		alertID, okID := alert["id"].(string)
		rev, okRev := alert["revision"].(string)
		if !okID || !okRev {
			t.Fatalf("claim: %v", alert)
		}
		return alertID, rev
	}
	outcomes := func() int {
		outs := call("GET", path+"/outcomes", "", "admin", 200)
		list, ok := outs["outcomes"].([]any)
		if !ok {
			t.Fatalf("outcomes: %v", outs)
		}
		return len(list)
	}

	call("PUT", path+"/admin-alerts/bad", `{"body":"x","action":{"kind":"delete_everything","payload":{}}}`, "admin", 400)
	propose(1) // wrong place for Yuri
	alertID, oldRev := claim()
	call("POST", "/admin-alerts/"+alertID+"/delivered", `{}`, "", 200)
	propose(2) // survivoR corrected itself: same alert, new revision, DMed again
	again, newRev := claim()
	if again != alertID || newRev == oldRev || newRev == "" {
		t.Fatalf("re-proposal should re-offer the same alert with a new revision: %s %s %s", again, oldRev, newRev)
	}
	call("POST", "/admin-alerts/"+alertID+"/approve", `{"revision":"`+oldRev+`"}`, "admin", 409)
	if outcomes() != 0 {
		t.Fatal("a stale yes recorded something")
	}
	call("POST", "/admin-alerts/"+alertID+"/approve", `{"revision":"`+newRev+`"}`, "outsider", 403)
	call("POST", "/admin-alerts/"+alertID+"/approve", `{"revision":"`+newRev+`"}`, "nobody", 403)

	// Two admins say yes at once: exactly one records it.
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authorizedJSONRequest("POST", "/admin-alerts/"+alertID+"/approve", `{"revision":"`+newRev+`"}`, "token", "admin"))
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for c := range codes {
		got[c]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatalf("concurrent approvals: %v", got)
	}
	if outcomes() != 2 {
		t.Fatalf("approval should record both boots, got %d", outcomes())
	}
	// Once applied, re-proposing changes nothing and a late yes is refused.
	propose(1)
	call("POST", "/admin-alerts/"+alertID+"/approve", `{"revision":"`+oldRev+`"}`, "admin", 409)
	imports := call("GET", path+"/episode-imports", "", "admin", 200)
	if list, ok := imports["imports"].([]any); !ok || len(list) != 1 {
		t.Fatalf("imports: %v", imports)
	}
}
