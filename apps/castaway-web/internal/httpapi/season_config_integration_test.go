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
}
