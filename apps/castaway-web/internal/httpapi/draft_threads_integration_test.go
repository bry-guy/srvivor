package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestDraftThreadWatching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)

	instance := createInstanceForTest(t, ctx, queries, "Draft thread", 953)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	path := "/instances/" + instanceID
	contestants := []string{}
	for _, name := range []string{"Ann Alpha", `Benjamin "Coach" Wade`, "Cyd Cole"} {
		contestants = append(contestants, uuid.UUID(createContestantForTest(t, ctx, queries, instance.ID, name).ID.Bytes).String())
	}
	players := map[string]string{}
	for i, name := range []string{"P1", "P2", "P3"} {
		players[name] = uuid.UUID(createParticipantForTest(t, ctx, queries, instance.ID, name).ID.Bytes).String()
		if _, err := pool.Exec(ctx, `UPDATE participants SET discord_user_id = $2 WHERE public_id = $1`, players[name], fmt.Sprint(1000+i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := queries.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "7001", ChannelID: "7002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	type response struct {
		Status     string   `json:"status"`
		Player     string   `json:"player"`
		Problems   []string `json:"problems"`
		Order      int      `json:"order"`
		Submission *struct {
			Order  int `json:"order"`
			Points int `json:"bonus_points"`
		} `json:"submission"`
		Threads []struct {
			ThreadID string `json:"thread_id"`
		} `json:"threads"`
	}
	call := func(method, url, body, actor string, want int) response {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedJSONRequest(method, url, body, "token", actor))
		if recorder.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, recorder.Code, recorder.Body.String())
		}
		var decoded response
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	post := func(messageID, version, author, content string) response {
		t.Helper()
		body, err := json.Marshal(map[string]string{"message_id": messageID, "version": version, "author_discord_user_id": author, "content": content})
		if err != nil {
			t.Fatal(err)
		}
		return call(http.MethodPost, "/draft-threads/8001/messages", string(body), "", http.StatusOK)
	}

	call(http.MethodPut, path+"/draft-submissions/thread", `{"thread_id":"8001"}`, "admin", http.StatusConflict)
	call(http.MethodPost, path+"/draft-submissions", `{"tribes":["Savu","Toka"],"guild_id":"7001","channel_id":"7002"}`, "admin", http.StatusCreated)
	call(http.MethodPut, path+"/draft-submissions/thread", `{"thread_id":"8001"}`, "stranger", http.StatusForbidden)
	call(http.MethodPut, path+"/draft-submissions/thread", `{"thread_id":"8001"}`, "admin", http.StatusOK)
	if got := call(http.MethodGet, "/draft-threads", "", "", http.StatusOK); len(got.Threads) != 1 || got.Threads[0].ThreadID != "8001" {
		t.Fatalf("threads = %+v", got)
	}

	if got := post("9001", "v1", "1000", "good luck everyone!"); got.Status != "ignored" {
		t.Fatalf("chat: %+v", got)
	}
	// P1's first post has a problem: it claims order 1 but isn't saved.
	if got := post("9002", "v1", "1000", "1. Ann\n2. Coach\n3. Ann"); got.Status != "problem" || got.Player != "P1" || len(got.Problems) == 0 {
		t.Fatalf("problem draft: %+v", got)
	}
	if got := post("9002", "v1", "1000", "1. Ann\n2. Coach\n3. Ann"); got.Status != "duplicate" {
		t.Fatalf("replayed message: %+v", got)
	}
	// P2's complete draft (nickname and first names) is saved as order 2 with +1.
	if got := post("9003", "v1", "1001", "1. Coach\n2. Cyd\n3. Ann"); got.Status != "saved" || got.Submission == nil || got.Submission.Order != 2 || got.Submission.Points != 1 {
		t.Fatalf("ready draft: %+v", got)
	}
	if got := post("9003", "v2", "1001", "1. Coach\n2. Cyd\n3. Ann"); got.Status != "unchanged" {
		t.Fatalf("edited but same: %+v", got)
	}
	if got := post("9004", "v1", "4242", "Ann, Coach, Cyd"); got.Status != "problem" || got.Player != "<@4242>" {
		t.Fatalf("unlinked author: %+v", got)
	}
	// Admin fixes P1's draft: it completes the original claim, keeping order 1 and +2.
	fixed := fmt.Sprintf(`{"contestant_ids":[%q,%q,%q]}`, contestants[0], contestants[1], contestants[2])
	if got := call(http.MethodPut, path+"/drafts/"+players["P1"], fixed, "admin", http.StatusOK); got.Submission == nil || got.Submission.Order != 1 || got.Submission.Points != 2 {
		t.Fatalf("admin fix: %+v", got)
	}
	call(http.MethodPost, path+"/draft-submissions/"+players["P1"]+"/reject", "", "admin", http.StatusConflict)
	// P3's problem draft is rejected, so their later good draft makes a fresh claim at the back of the line.
	post("9005", "v1", "1002", "1. Ann\n2. Cyd\n3. Cyd")
	if got := call(http.MethodPost, path+"/draft-submissions/"+players["P3"]+"/reject", "", "admin", http.StatusOK); got.Order != 3 {
		t.Fatalf("reject: %+v", got)
	}
	if got := post("9006", "v1", "1002", "Ann\nCyd\nCoach"); got.Status != "saved" || got.Submission.Order != 3 || got.Submission.Points != 0 {
		t.Fatalf("after reject: %+v", got)
	}
}
