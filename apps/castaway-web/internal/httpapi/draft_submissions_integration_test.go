package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestDraftSubmissionEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)

	instance := createInstanceForTest(t, ctx, queries, "Draft events", 952)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	path := "/instances/" + instanceID
	contestants := []string{}
	for _, name := range []string{"Ann", "Ben", "Cy"} {
		contestants = append(contestants, uuid.UUID(createContestantForTest(t, ctx, queries, instance.ID, name).ID.Bytes).String())
	}
	players := []string{}
	for _, name := range []string{"P1", "P2", "P3", "P4", "P5"} {
		players = append(players, uuid.UUID(createParticipantForTest(t, ctx, queries, instance.ID, name).ID.Bytes).String())
	}
	if _, err := pool.Exec(ctx, `UPDATE participants SET discord_user_id = '1234567890' WHERE public_id = $1`, players[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := queries.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "7001", ChannelID: "7002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	type response struct {
		Submission *struct {
			Order  int `json:"order"`
			Points int `json:"bonus_points"`
		} `json:"submission"`
		Last        any `json:"last"`
		Leaderboard []struct {
			Name  string `json:"participant_name"`
			Bonus int    `json:"bonus_points"`
		} `json:"leaderboard"`
	}
	call := func(method, url, body, actor string) (int, response) {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedJSONRequest(method, url, body, "token", actor))
		var decoded response
		if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode %s: %v", recorder.Body.String(), err)
		}
		return recorder.Code, decoded
	}
	draft := func(i int) (int, response) {
		return call(http.MethodPut, path+"/drafts/"+players[i], fmt.Sprintf(`{"contestant_ids":["%s"]}`, strings.Join(contestants, `","`)), "admin")
	}

	// Without opening, drafts save as before with no event.
	open := `{"tribes":["Savu","Toka"],"guild_id":"7001","channel_id":"7002"}`
	if code, body := call(http.MethodPost, path+"/draft-submissions", open, "stranger"); code != http.StatusForbidden {
		t.Fatalf("non-admin open: %d %v", code, body)
	}
	if code, body := call(http.MethodPost, path+"/draft-submissions", `{"tribes":["Savu","Toka"],"guild_id":"7001","channel_id":"9999"}`, "admin"); code != http.StatusConflict {
		t.Fatalf("unbound channel: %d %v", code, body)
	}
	if code, body := call(http.MethodPost, path+"/draft-submissions", open, "admin"); code != http.StatusCreated {
		t.Fatalf("open: %d %v", code, body)
	}
	if code, _ := call(http.MethodPost, path+"/draft-submissions", open, "admin"); code != http.StatusConflict {
		t.Fatalf("second open: %d", code)
	}

	for i := 0; i < 4; i++ {
		code, body := draft(i)
		if code != http.StatusOK || body.Submission == nil || body.Submission.Order != i+1 {
			t.Fatalf("draft %d: %d %v", i, code, body)
		}
	}
	// Resubmitting changes picks only.
	if code, body := draft(0); code != http.StatusOK || body.Submission != nil {
		t.Fatalf("resubmit: %d %v", code, body)
	}
	if code, body := call(http.MethodPost, path+"/draft-submissions/close", "", "admin"); code != http.StatusOK || body.Last == nil {
		t.Fatalf("close: %d %v", code, body)
	}
	// A late draft still gets a tribe, but no bonus.
	if code, body := draft(4); code != http.StatusOK || body.Submission == nil || body.Submission.Points != 0 {
		t.Fatalf("late draft: %d %v", code, body)
	}

	var savu, toka int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE g.name = 'Savu'), count(*) FILTER (WHERE g.name = 'Toka')
		FROM participant_group_membership_periods m JOIN participant_groups g ON g.id = m.participant_group_id WHERE g.instance_id = (SELECT id FROM instances WHERE public_id = $1)`, instanceID).Scan(&savu, &toka); err != nil {
		t.Fatal(err)
	}
	if savu+toka != 5 || savu-toka > 1 || toka-savu > 1 {
		t.Fatalf("tribes unbalanced: savu=%d toka=%d", savu, toka)
	}
	_, board := call(http.MethodGet, path+"/leaderboard", "", "admin")
	bonus := map[string]int{}
	for _, row := range board.Leaderboard {
		bonus[row.Name] = row.Bonus
	}
	if bonus["P1"] != 2 || bonus["P2"] != 1 || bonus["P3"] != 0 || bonus["P5"] != 0 {
		t.Fatalf("bonus = %v", bonus)
	}
	rows, err := pool.Query(ctx, `SELECT request_key, body, notify_users FROM announcements ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var bodies []string
	for rows.Next() {
		var key, text string
		var notify bool
		if err := rows.Scan(&key, &text, &notify); err != nil || !notify {
			t.Fatalf("announcement %s notify=%v %v", key, notify, err)
		}
		bodies = append(bodies, text)
	}
	all := strings.Join(bodies, "\n")
	if len(bodies) != 6 || !strings.Contains(all, "🔥 <@1234567890> — first") || !strings.Contains(all, "**P2**, right behind") || !strings.Contains(all, "**P4**... last draft in") {
		t.Fatalf("announcements = %q", bodies)
	}
}
