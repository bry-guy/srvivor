package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestDraftThreadKeelingOriginalMessage(t *testing.T) {
	wantNames := []string{
		"Mike Pinsky", "Brady Booker", "Jenna Doore", `Angelica "Jelly" Loblack`, "Ana Sani",
		"Cristian Chavez", "Maggie Nestor", "Ori Jean-Charles", "Rob Antonson", "Devin Way",
		"Kristin Flickinger", "Alexis Levine", "Carter Krull", "Lewis Kelly", "Sharonda Cox",
		`Danny "Kilby" Kilby`, "Thien An Nguyen", "Linnea Capobianco", "Eric Macksoud", "Patt Cannaday", "Aaliyah Puglia",
	}
	testOriginalDraftThreadMessage(t, "Keeling", "keeling-original-draft.txt", "1554599095459651617", "2026-09-29T21:01:47.412Z", wantNames, false)
}

func TestDraftThreadMooneyOriginalMessage(t *testing.T) {
	wantNames := []string{
		"Linnea Capobianco", "Rob Antonson", "Thien An Nguyen", `Danny "Kilby" Kilby`, "Jenna Doore",
		`Angelica "Jelly" Loblack`, "Kristin Flickinger", "Brady Booker", "Carter Krull", "Lewis Kelly",
		"Alexis Levine", "Devin Way", "Ori Jean-Charles", "Sharonda Cox", "Cristian Chavez",
		"Patt Cannaday", "Maggie Nestor", "Ana Sani", "Mike Pinsky", "Eric Macksoud", "Aaliyah Puglia",
	}
	testOriginalDraftThreadMessage(t, "Mooney", "mooney-original-draft.txt", "1554628735611969549", "2026-09-29T22:59:34.175Z", wantNames, true)
}

func testOriginalDraftThreadMessage(t *testing.T, playerName, fixture, messageID, version string, wantNames []string, needsConfirmation bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, playerName+" original draft", 51)
	instanceID := uuid.UUID(instance.ID.Bytes).String()
	path := "/instances/" + instanceID
	var firstDraft []string
	for i, name := range wantNames {
		createContestantForTest(t, ctx, queries, instance.ID, name)
		firstDraft = append(firstDraft, fmt.Sprintf("%d %s", i+1, name))
	}
	marv := createParticipantForTest(t, ctx, queries, instance.ID, "Marv")
	subject := createParticipantForTest(t, ctx, queries, instance.ID, playerName)
	for i, player := range []db.CreateParticipantRow{marv, subject} {
		if _, err := pool.Exec(ctx, `UPDATE participants SET discord_user_id = $2 WHERE public_id = $1`, player.ID, fmt.Sprint(1000+i)); err != nil {
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
		Problems   []string `json:"problems"`
		Submission *struct {
			Order  int `json:"order"`
			Points int `json:"bonus_points"`
		} `json:"submission"`
		Picks []struct {
			Position int    `json:"position"`
			Name     string `json:"contestant_name"`
		} `json:"picks"`
	}
	call := func(method, url, body, actor string, want int) response {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedJSONRequest(method, url, body, "token", actor))
		if recorder.Code != want {
			t.Fatalf("%s %s: %d %s", method, url, recorder.Code, recorder.Body.String())
		}
		var got response
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	post := func(id, version, author, content string) response {
		t.Helper()
		body, err := json.Marshal(map[string]string{"message_id": id, "version": version, "author_discord_user_id": author, "content": content})
		if err != nil {
			t.Fatal(err)
		}
		return call(http.MethodPost, "/draft-threads/8001/messages", string(body), "", http.StatusOK)
	}
	call(http.MethodPost, path+"/draft-submissions", `{"tribes":["Savu","Toka"],"guild_id":"7001","channel_id":"7002"}`, "admin", http.StatusCreated)
	call(http.MethodPut, path+"/draft-submissions/thread", `{"thread_id":"8001"}`, "admin", http.StatusOK)
	if got := post("9001", "v1", "1000", strings.Join(firstDraft, "\n")); got.Status != "saved" || got.Submission == nil || got.Submission.Order != 1 || got.Submission.Points != 2 {
		t.Fatalf("first submission: %+v", got)
	}
	original, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	pendingLines := strings.Split(string(original), "\n")
	pendingLines[len(pendingLines)-1] = "21 Unknown X"
	if got := post("9002", "v1", "1001", strings.Join(pendingLines, "\n")); got.Status != "problem" {
		t.Fatalf("pending second submission: %+v", got)
	}
	confirmed := string(original)
	if needsConfirmation {
		got := post(messageID, version, "1001", string(original))
		if got.Status != "problem" || got.Submission != nil {
			t.Fatalf("unconfirmed message must not save: %+v", got)
		}
		problems := strings.Join(got.Problems, "\n")
		for _, suggestion := range []string{"possible Rob Antonson; requires confirmation", "possible Sharonda Cox; requires confirmation"} {
			if !strings.Contains(problems, suggestion) {
				t.Fatalf("missing suggestion %q: %v", suggestion, got.Problems)
			}
		}
		if saved := call(http.MethodGet, path+"/drafts/"+uuid.UUID(subject.ID.Bytes).String(), "", "admin", http.StatusOK); len(saved.Picks) != 0 {
			t.Fatalf("unconfirmed picks were saved: %+v", saved)
		}
		var sideEffects int
		if err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM bonus_point_ledger_entries WHERE participant_id = (SELECT id FROM participants WHERE public_id = $1)) +
			(SELECT count(*) FROM participant_group_membership_periods WHERE participant_id = (SELECT id FROM participants WHERE public_id = $1)) +
			(SELECT count(*) FROM announcements WHERE request_key = $2)`, subject.ID, "draft-submission-"+uuid.UUID(subject.ID.Bytes).String()).Scan(&sideEffects); err != nil {
			t.Fatal(err)
		}
		if sideEffects != 0 {
			t.Fatalf("unconfirmed message caused %d reward side effects", sideEffects)
		}
		if replay := post(messageID, version, "1001", string(original)); replay.Status != "duplicate" {
			t.Fatalf("unconfirmed replay: %+v", replay)
		}
		confirmed = strings.NewReplacer("Rhodey Rob", "Rob Antonson", "Shondra", "Sharonda Cox").Replace(confirmed)
		version += "-confirmed"
	}
	if got := post(messageID, version, "1001", confirmed); got.Status != "saved" || got.Submission == nil || got.Submission.Order != 2 || got.Submission.Points != 1 {
		t.Fatalf("confirmed message: %+v", got)
	}
	assertSavedOrder := func() {
		t.Helper()
		got := call(http.MethodGet, path+"/drafts/"+uuid.UUID(subject.ID.Bytes).String(), "", "admin", http.StatusOK)
		if len(got.Picks) != len(wantNames) {
			t.Fatalf("saved %d picks, want %d", len(got.Picks), len(wantNames))
		}
		for i, pick := range got.Picks {
			if pick.Position != i+1 || pick.Name != wantNames[i] {
				t.Fatalf("position %d: %+v, want %q", i+1, pick, wantNames[i])
			}
		}
	}
	assertSavedOrder()
	if got := post(messageID, version, "1001", confirmed); got.Status != "duplicate" {
		t.Fatalf("replay: %+v", got)
	}
	if got := post(messageID, "v2", "1001", confirmed); got.Status != "unchanged" || got.Submission != nil {
		t.Fatalf("same picks in another version: %+v", got)
	}
	assertSavedOrder()
	var awards, points, memberships, announcements int
	if err := pool.QueryRow(ctx, `SELECT count(*), COALESCE(sum(points), 0) FROM bonus_point_ledger_entries WHERE participant_id = (SELECT id FROM participants WHERE public_id = $1)`, subject.ID).Scan(&awards, &points); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM participant_group_membership_periods WHERE participant_id = (SELECT id FROM participants WHERE public_id = $1)`, subject.ID).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM announcements WHERE request_key = $1`, "draft-submission-"+uuid.UUID(subject.ID.Bytes).String()).Scan(&announcements); err != nil {
		t.Fatal(err)
	}
	if awards != 1 || points != 1 || memberships != 1 || announcements != 1 {
		t.Fatalf("replay duplicated rewards: awards=%d points=%d memberships=%d announcements=%d", awards, points, memberships, announcements)
	}
}
