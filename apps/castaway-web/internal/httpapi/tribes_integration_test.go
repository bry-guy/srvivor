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

func TestTribesAndTribeChallenges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	queries := db.New(pool)

	instance := createInstanceForTest(t, ctx, queries, "Tribes", 951)
	id := func(p db.CreateParticipantRow) string { return uuid.UUID(p.ID.Bytes).String() }
	a := createParticipantForTest(t, ctx, queries, instance.ID, "Alice")
	b := createParticipantForTest(t, ctx, queries, instance.ID, "Bob")
	c := createParticipantForTest(t, ctx, queries, instance.ID, "Cara")
	if _, err := queries.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "admin"}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"token"}})).Router()
	instancePath := "/instances/" + uuid.UUID(instance.ID.Bytes).String()
	type response struct {
		AwardedCount int `json:"awarded_count"`
		Tribes       []struct {
			Name    string `json:"name"`
			Members []any  `json:"members"`
		} `json:"tribes"`
		Leaderboard []struct {
			ParticipantName string `json:"participant_name"`
			BonusPoints     int    `json:"bonus_points"`
		} `json:"leaderboard"`
	}
	call := func(method, path, body, actor string) (int, response) {
		t.Helper()
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedJSONRequest(method, path, body, "token", actor))
		var decoded response
		if recorder.Code < 300 {
			if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
				t.Fatalf("decode %s %s: %v", method, path, err)
			}
		}
		return recorder.Code, decoded
	}
	tribes := func(at time.Time, savu, toka []string) string {
		quote := func(ids []string) string { return `"` + strings.Join(ids, `","`) + `"` }
		return fmt.Sprintf(`{"effective_at":%q,"tribes":[{"name":"Savu","participant_ids":[%s]},{"name":"Toka","participant_ids":[%s]}]}`, at.Format(time.RFC3339), quote(savu), quote(toka))
	}
	start := time.Date(2030, time.October, 1, 0, 0, 0, 0, time.UTC)
	swap := start.Add(7 * 24 * time.Hour)

	if code, _ := call(http.MethodPut, instancePath+"/tribes", tribes(start, []string{id(a), id(b)}, []string{id(c)}), "stranger"); code != http.StatusForbidden {
		t.Fatalf("non-admin set tribes: %d", code)
	}
	if code, body := call(http.MethodPut, instancePath+"/tribes", tribes(start, []string{id(a), id(b)}, []string{id(c)}), "admin"); code != http.StatusOK {
		t.Fatalf("set tribes: %d %v", code, body)
	}
	if code, _ := call(http.MethodPut, instancePath+"/tribes", tribes(start, []string{id(b), id(a)}, []string{id(c)}), "admin"); code != http.StatusOK {
		t.Fatalf("identical retry: %d", code)
	}
	if code, body := call(http.MethodPut, instancePath+"/tribes", tribes(swap, []string{id(a)}, []string{id(b), id(c)}), "admin"); code != http.StatusOK {
		t.Fatalf("swap: %d %v", code, body)
	}
	if code, _ := call(http.MethodPut, instancePath+"/tribes", tribes(start.Add(time.Hour), []string{id(a), id(c)}, []string{id(b)}), "admin"); code != http.StatusConflict {
		t.Fatalf("backdated change before the swap should conflict: %d", code)
	}
	_, before := call(http.MethodGet, instancePath+"/tribes?at="+start.Add(time.Hour).Format(time.RFC3339), "", "admin")
	_, after := call(http.MethodGet, instancePath+"/tribes?at="+swap.Format(time.RFC3339), "", "admin")
	members := func(body response, tribe string) int {
		for _, entry := range body.Tribes {
			if entry.Name == tribe {
				return len(entry.Members)
			}
		}
		return 0
	}
	if members(before, "Savu") != 2 || members(after, "Savu") != 1 || members(after, "Toka") != 2 {
		t.Fatalf("tribe history wrong: before=%v after=%v", before, after)
	}

	immunity := fmt.Sprintf(`{"key":"ep1-immunity","kind":"immunity","winning_tribes":["savu"],"effective_at":%q}`, start.Add(time.Hour).Format(time.RFC3339))
	if code, body := call(http.MethodPost, instancePath+"/tribe-challenges", immunity, "admin"); code != http.StatusCreated || body.AwardedCount != 2 {
		t.Fatalf("immunity: %d %v", code, body)
	}
	if code, body := call(http.MethodPost, instancePath+"/tribe-challenges", immunity, "admin"); code != http.StatusOK || body.AwardedCount != 2 {
		t.Fatalf("immunity retry: %d %v", code, body)
	}
	changed := strings.Replace(immunity, `"savu"`, `"Toka"`, 1)
	if code, _ := call(http.MethodPost, instancePath+"/tribe-challenges", changed, "admin"); code != http.StatusConflict {
		t.Fatalf("changed retry should conflict: %d", code)
	}
	reward := fmt.Sprintf(`{"key":"ep2-reward","kind":"reward","winning_tribes":["Toka"],"effective_at":%q}`, swap.Add(time.Hour).Format(time.RFC3339))
	if code, body := call(http.MethodPost, instancePath+"/tribe-challenges", reward, "admin"); code != http.StatusCreated || body.AwardedCount != 2 {
		t.Fatalf("reward: %d %v", code, body)
	}

	_, leaderboard := call(http.MethodGet, instancePath+"/leaderboard", "", "admin")
	bonus := map[string]int{}
	for _, row := range leaderboard.Leaderboard {
		bonus[row.ParticipantName] = row.BonusPoints
	}
	// Alice: Savu immunity +2. Bob: Savu immunity +2, then Toka reward +1 after the swap. Cara: Toka reward +1.
	if bonus["Alice"] != 2 || bonus["Bob"] != 3 || bonus["Cara"] != 1 {
		t.Fatalf("bonus points = %v", bonus)
	}
}
