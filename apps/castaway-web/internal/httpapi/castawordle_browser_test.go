package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/gameplay"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCastawordleBrowser(t *testing.T) {
	if os.Getenv("CASTAWAY_BROWSER_TEST") != "1" {
		t.Skip("set CASTAWAY_BROWSER_TEST=1 with Node/Playwright available to run browser checks")
	}
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Browser preview", 51)
	if err := gameplay.NewService(q).CopyInstanceSchedule(ctx, instance.ID, 51); err != nil {
		t.Fatal(err)
	}
	player := createParticipantForTest(t, ctx, q, instance.ID, "Browser Player")
	if _, err := q.SetParticipantDiscordUserID(ctx, db.SetParticipantDiscordUserIDParams{ID: player.ID, DiscordUserID: pgtype.Text{String: "cw-browser", Valid: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instance.ID, DiscordUserID: "cw-browser"}); err != nil {
		t.Fatal(err)
	}
	site := httptest.NewUnstartedServer(nil)
	base := "http://" + site.Listener.Addr().String()
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authorize" {
			http.Redirect(w, r, base+"/auth/callback?code=fixture&state="+url.QueryEscape(r.URL.Query().Get("state")), http.StatusFound)
			return
		}
		body := map[string]string{"access_token": "fixture"}
		if r.URL.Path == "/users/@me" {
			body = map[string]string{"id": "cw-browser", "username": "Browser Player"}
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Error(err)
		}
	}))
	defer discord.Close()
	site.Config.Handler = httpapi.New(pool, httpapi.WithClock(func() time.Time { return time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC) }), httpapi.WithPublic(httpapi.PublicConfig{
		BaseURL: base, InstanceID: uuid.UUID(instance.ID.Bytes).String(), DiscordClientID: "fixture", DiscordClientSecret: "fixture",
		DiscordAPIBaseURL: discord.URL, DiscordAuthorizeURL: discord.URL + "/authorize",
	})).PublicRouter()
	site.Start()
	defer site.Close()
	cmd := exec.CommandContext(ctx, "node", "../../script/castawordle-browser-check.cjs", base)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("browser checks: %v\n%s", err, out)
	}
	t.Log(string(out))
}
