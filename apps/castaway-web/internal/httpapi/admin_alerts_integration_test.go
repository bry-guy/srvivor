package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAdminAlerts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Alerts", 955)
	path := "/instances/" + uuid.UUID(instance.ID.Bytes).String() + "/admin-alerts/"
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "2001"}); err != nil {
		t.Fatal(err)
	}
	if err := queries.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "7001", ChannelID: "7002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 7, 4, 0, 0, 0, time.UTC)
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	type reply struct {
		Created bool `json:"created"`
		Alert   *struct {
			ID   string `json:"id"`
			Body string `json:"body"`
		} `json:"alert"`
		Admins []string `json:"admin_discord_user_ids"`
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
	call(http.MethodPut, path+"w4-game-missing", `{"body":"Week 4 has no game"}`, "1000", http.StatusForbidden) // not an admin
	call(http.MethodPut, path+"w4-game-missing", `{"body":" "}`, "2001", http.StatusBadRequest)
	if !call(http.MethodPut, path+"w4-game-missing", `{"body":"Week 4 has no game"}`, "2001", http.StatusOK).Created {
		t.Fatal("first raise should create the alert")
	}
	if call(http.MethodPut, path+"w4-game-missing", `{"body":"Week 4 has no game (again)"}`, "2001", http.StatusOK).Created {
		t.Fatal("raising the same key again must be a no-op")
	}
	claim := func(guild string) reply {
		return call(http.MethodPost, "/admin-alerts/claim", `{"guild_ids":["`+guild+`"]}`, "", http.StatusOK)
	}
	if other := claim("9999"); other.Alert != nil {
		t.Fatal("an alert was offered to a guild without the season")
	}
	got := claim("7001")
	if got.Alert == nil || got.Alert.Body != "Week 4 has no game" || len(got.Admins) != 1 || got.Admins[0] != "2001" {
		t.Fatalf("claim: %+v", got)
	}
	if again := claim("7001"); again.Alert != nil {
		t.Fatal("claimed twice within its lease")
	}
	now = now.Add(11 * time.Minute) // undelivered: offered again
	if retry := claim("7001"); retry.Alert == nil || retry.Alert.ID != got.Alert.ID {
		t.Fatalf("undelivered alert should be retried: %+v", retry)
	}
	call(http.MethodPost, "/admin-alerts/"+got.Alert.ID+"/delivered", `{}`, "", http.StatusOK)
	now = now.Add(11 * time.Minute)
	if done := claim("7001"); done.Alert != nil {
		t.Fatal("a delivered alert was offered again")
	}
	if call(http.MethodPut, path+"w4-game-missing", `{"body":"Week 4 has no game"}`, "2001", http.StatusOK).Created {
		t.Fatal("a delivered alert must not be raised again")
	}
}
