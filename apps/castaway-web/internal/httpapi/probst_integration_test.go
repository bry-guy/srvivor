package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
)

func TestProbstChannelAndPlayerAdministration(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	first := createInstanceForTest(t, ctx, q, "Probst first", 43)
	second := createInstanceForTest(t, ctx, q, "Probst second", 44)
	alice := createParticipantForTest(t, ctx, q, first.ID, "Alice")
	bob := createParticipantForTest(t, ctx, q, first.ID, "Bob")
	other := createParticipantForTest(t, ctx, q, second.ID, "Other")
	ids := []string{uuid.UUID(first.ID.Bytes).String(), uuid.UUID(second.ID.Bytes).String()}
	const admin = "235246238382030849"
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: second.ID, DiscordUserID: "other"}); err != nil {
		t.Fatal(err)
	}
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"probst-test"}}), httpapi.WithBootstrapAdminDiscordUserID(admin)).Router()
	server := httptest.NewServer(router)
	defer server.Close()
	request := func(method, path, body, actor string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := wordleServe(router, method, path, body, "probst-test", actor)
		wordleRequireStatus(t, r, want)
		return r
	}
	bootstrap := "/instances/" + ids[0] + "/admins/bootstrap"
	request("POST", bootstrap, "", "other", 403)
	disabled := httpapi.New(pool, httpapi.WithBootstrapAdminDiscordUserID(admin)).Router()
	wordleRequireStatus(t, wordleServe(disabled, "POST", bootstrap, "", "forged", admin), 401)
	disabledBootstrap := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"probst-test"}})).Router()
	wordleRequireStatus(t, wordleServe(disabledBootstrap, "POST", bootstrap, "", "probst-test", admin), 403)
	binary := filepath.Join(t.TempDir(), "probst")
	build := exec.Command("go", "build")
	build.Args = append(build.Args, "-o", binary, ".")
	build.Dir = "../../../probst"
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build Probst: %v %s", err, output)
	}
	cli := func(success bool, args ...string) []byte {
		t.Helper()
		command := &exec.Cmd{Path: binary, Args: append([]string{binary, "--server", server.URL, "--actor", admin, "--json"}, args...), Env: append(os.Environ(), "PROBST_TOKEN=probst-test")}
		output, err := command.CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("probst %v: err=%v output=%s", args, err, output)
		}
		return output
	}
	cli(true, "auth", "status")
	cli(true, "instance", "list")
	cli(true, "instance", "show", ids[0])
	cli(true, "instance", "bootstrap-admin", ids[0])
	cli(true, "instance", "bootstrap-admin", ids[0])
	cli(true, "channel", "bind", "201", "--guild", "101", "--instance", ids[0])
	cli(true, "channel", "show", "201", "--guild", "101")
	cli(false, "channel", "bind", "201", "--guild", "101", "--instance", ids[1], "--yes")
	request("POST", "/instances/"+ids[1]+"/admins/bootstrap", "", admin, 409)
	count, err := q.CountInstanceAdmins(ctx, second.ID)
	if err != nil || count != 1 {
		t.Fatalf("bootstrap changed admin count: %d %v", count, err)
	}
	granted, err := q.IsInstanceAdmin(ctx, db.IsInstanceAdminParams{InstanceID: second.ID, DiscordUserID: admin})
	if err != nil || granted {
		t.Fatalf("rejected bootstrap granted access: %t %v", granted, err)
	}
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: second.ID, DiscordUserID: admin}); err != nil {
		t.Fatal(err)
	}
	cli(true, "instance", "bootstrap-admin", ids[1])
	cli(false, "channel", "bind", "201", "--guild", "101", "--instance", ids[1])
	cli(true, "channel", "bind", "202", "--guild", "101", "--instance", ids[1])
	request("PUT", "/discord/guilds/101/channels/201", fmt.Sprintf(`{"instance_id":%q,"replace":true}`, ids[1]), "other", 403)
	if result := cli(true, "channel", "show", "201", "--guild", "101"); !bytes.Contains(result, []byte(ids[0])) {
		t.Fatalf("rejected rebind changed binding: %s", result)
	}
	a := uuid.UUID(alice.ID.Bytes).String()
	b := uuid.UUID(bob.ID.Bytes).String()
	cli(true, "player", "link", a, "--instance", ids[0], "--discord-user", admin)
	cli(true, "player", "link", a, "--instance", ids[0], "--discord-user", admin)
	cli(false, "player", "link", b, "--instance", ids[0], "--discord-user", admin)
	cli(true, "player", "link", b, "--instance", ids[0], "--discord-user", "987654321")
	cli(false, "player", "link", a, "--instance", ids[0], "--discord-user", "987654321")
	cli(false, "player", "link", uuid.UUID(other.ID.Bytes).String(), "--instance", ids[0], "--discord-user", "987654321")
	cli(true, "player", "link", uuid.UUID(other.ID.Bytes).String(), "--instance", ids[1], "--discord-user", admin)
	cli(true, "player", "list", "--instance", ids[0])
	cli(true, "scores", "--instance", ids[0])
	cli(true, "draft", "show", a, "--instance", ids[0])
	storedAlice, err := q.GetParticipant(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	storedBob, err := q.GetParticipant(ctx, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedAlice.DiscordUserID.String != admin || storedBob.DiscordUserID.String != "987654321" {
		t.Fatal("conflicting link changed persisted identity mappings")
	}
	linkPath := "/instances/" + ids[0] + "/participants/" + a + "/discord-link"
	wordleRequireStatus(t, wordleServe(disabled, "PUT", linkPath, `{"discord_user_id":"111"}`, "forged", admin), 401)
	wordleRequireStatus(t, wordleServe(disabled, "PUT", "/discord/guilds/101/channels/201", fmt.Sprintf(`{"instance_id":%q,"replace":true}`, ids[1]), "forged", admin), 401)
	wordleRequireStatus(t, wordleServe(disabled, "DELETE", "/discord/guilds/101/channels/201", "", "forged", admin), 401)
	request("DELETE", linkPath, "", "other", 403)
	wordleRequireStatus(t, wordleServe(disabled, "DELETE", linkPath, "", "forged", admin), 401)
	if result := cli(true, "channel", "show", "201", "--guild", "101"); !bytes.Contains(result, []byte(ids[0])) {
		t.Fatalf("forged request changed binding: %s", result)
	}
	storedAlice, err = q.GetParticipant(ctx, alice.ID)
	if err != nil || storedAlice.DiscordUserID.String != admin {
		t.Fatalf("forged request changed link: %v %v", storedAlice.DiscordUserID, err)
	}
	cli(false, "player", "unlink", a, "--instance", ids[0])
	cli(true, "player", "unlink", a, "--instance", ids[0], "--yes")
	cli(true, "channel", "bind", "201", "--guild", "101", "--instance", ids[1], "--yes")
	result := cli(true, "channel", "show", "201", "--guild", "101")
	if !bytes.Contains(result, []byte(ids[1])) {
		t.Fatalf("rebind did not persist: %s", result)
	}
	cli(false, "channel", "unbind", "201", "--guild", "101")
	cli(true, "channel", "unbind", "201", "--guild", "101", "--yes")
	request("GET", "/discord/guilds/101/channels/201", "", admin, 404)
	request("GET", "/discord/guilds/101/channels/202", "", admin, 200)
	var linked struct {
		Participant struct {
			DiscordUserID string `json:"discord_user_id"`
		}
	}
	response := request("GET", "/instances/"+ids[1]+"/participants/me", "", admin, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &linked); err != nil {
		t.Fatal(err)
	}
	if linked.Participant.DiscordUserID != admin {
		t.Fatal("instance-scoped link was not preserved")
	}
	if !strings.Contains(string(cli(true, "auth", "status")), "trusted-service delegation") {
		t.Fatal("auth status must not claim verified human login")
	}
}
