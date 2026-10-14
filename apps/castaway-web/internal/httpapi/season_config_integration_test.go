package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The synced season config is admin-only and updates only from the revision the caller last saw.
func TestSeasonConfigSync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Season config", 51)
	other := createInstanceForTest(t, ctx, queries, "Other season", 52)
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "2001"}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	path := "/instances/" + uuid.UUID(instance.ID.Bytes).String() + "/season-config"
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

	call("GET", path, "", "2001", http.StatusNotFound)
	call("PUT", path, `{"season_yaml":"season: 51","scores_template":"hi"}`, "9999", http.StatusForbidden) // not an admin
	first := fmt.Sprint(call("PUT", path, `{"season_yaml":"season: 51","scores_template":"hi"}`, "2001", http.StatusOK)["revision"])
	call("PUT", path, `{"season_yaml":"season: 51","scores_template":"hi"}`, "2001", http.StatusConflict) // must name the current revision
	second := fmt.Sprint(call("PUT", path, `{"season_yaml":"season: 51\nchannel: x","scores_template":"hi","base_revision":"`+first+`"}`, "2001", http.StatusOK)["revision"])
	call("PUT", path, `{"season_yaml":"season: 51","scores_template":"x","base_revision":"`+first+`"}`, "2001", http.StatusConflict) // stale
	got := call("GET", path, "", "2001", http.StatusOK)
	if got["revision"] != second || got["season_yaml"] != "season: 51\nchannel: x" || got["updated_by"] != "2001" {
		t.Fatalf("config = %v", got)
	}
	call("GET", "/instances/"+uuid.UUID(other.ID.Bytes).String()+"/season-config", "", "2001", http.StatusForbidden) // other instance

	// Episode schedule from a season file: creates and moves (even into each other's slots), admin only, idempotent.
	eps := "/instances/" + uuid.UUID(instance.ID.Bytes).String() + "/episodes"
	at := func(day int) string { return time.Date(2030, 9, day, 0, 0, 0, 0, time.UTC).Format(time.RFC3339) }
	body := func(a, b string) string {
		return `{"episodes":[{"episode_number":1,"airs_at":"` + a + `"},{"episode_number":2,"airs_at":"` + b + `"}]}`
	}
	call("PUT", eps, body(at(1), at(8)), "9999", http.StatusForbidden)
	if got := call("PUT", eps, body(at(1), at(8)), "2001", http.StatusOK)["changed"]; got != 2.0 {
		t.Fatalf("created %v", got)
	}
	if got := call("PUT", eps, body(at(8), at(15)), "2001", http.StatusOK)["changed"]; got != 2.0 { // ep 1 takes ep 2's old slot
		t.Fatalf("moved %v", got)
	}
	if got := call("PUT", eps, body(at(8), at(15)), "2001", http.StatusOK)["changed"]; got != 0.0 {
		t.Fatalf("replay changed %v", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM instance_episodes e JOIN instances i ON i.id = e.instance_id WHERE i.public_id = $1`, instance.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("episodes %d %v", n, err)
	}
}

// A season whose scheduled job stops succeeding gets one admin DM per stall.
func TestAutomationHeartbeatAlert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)
	instance := createInstanceForTest(t, ctx, queries, "Heartbeat", 51)
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "2001"}); err != nil {
		t.Fatal(err)
	}
	if err := queries.SetDiscordChannelBinding(ctx, db.SetDiscordChannelBindingParams{GuildID: "7001", ChannelID: "7002", InstanceID: instance.ID}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	router := httpapi.New(pool, httpapi.WithClock(func() time.Time { return now }), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	path := "/instances/" + uuid.UUID(instance.ID.Bytes).String()
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
	claim := func() map[string]any {
		if alert, ok := call("POST", "/admin-alerts/claim", `{"guild_ids":["7001"]}`, "", http.StatusOK)["alert"].(map[string]any); ok {
			return alert
		}
		return nil
	}

	call("PUT", path+"/automation-heartbeat", "", "2001", http.StatusNotFound) // nothing synced yet
	call("PUT", path+"/season-config", `{"season_yaml":"season: 51","scores_template":"hi"}`, "2001", http.StatusOK)
	call("PUT", path+"/automation-heartbeat", "", "9999", http.StatusForbidden)
	call("PUT", path+"/automation-heartbeat", "", "2001", http.StatusOK)
	now = now.Add(29 * time.Minute)
	if a := claim(); a != nil {
		t.Fatalf("alerted while fresh: %v", a)
	}
	now = now.Add(2 * time.Minute)
	a := claim()
	if a == nil || !strings.Contains(fmt.Sprint(a["body"]), "hasn't run successfully since Oct 14 8:00am ET") {
		t.Fatalf("stale alert = %v", a)
	}
	call("POST", "/admin-alerts/"+fmt.Sprint(a["id"])+"/delivered", "{}", "", http.StatusOK)
	now = now.Add(time.Hour)
	if again := claim(); again != nil {
		t.Fatalf("alerted twice for one stall: %v", again)
	}
	call("PUT", path+"/automation-heartbeat", "", "2001", http.StatusOK) // recovers, then stalls again
	now = now.Add(31 * time.Minute)
	if next := claim(); next == nil {
		t.Fatal("a second stall wasn't alerted")
	}
}
