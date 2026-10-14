package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "probst-test-home-")
	if err != nil {
		panic(err)
	}
	original, existed := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	status := m.Run()
	if existed {
		_ = os.Setenv("HOME", original)
	} else {
		_ = os.Unsetenv("HOME")
	}
	_ = os.RemoveAll(home)
	os.Exit(status)
}

func writeTestConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".config", "probst", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Dir(filepath.Dir(filepath.Dir(path))))
	return path
}

func runConfigCommand(args ...string) (string, error) {
	cmd := newCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestConfigFileAndPrecedence(t *testing.T) {
	called := 0
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.Header.Get("Authorization") != "Bearer file-secret" || r.Header.Get("X-Discord-User-ID") != "file-actor" {
			t.Errorf("incorrect file credentials")
		}
		_, _ = w.Write([]byte(`{"source":"file"}`))
	}))
	defer fileServer.Close()
	envServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.Header.Get("Authorization") != "Bearer env-secret" || r.Header.Get("X-Discord-User-ID") != "flag-actor" {
			t.Errorf("incorrect overridden credentials")
		}
		_, _ = w.Write([]byte(`{"source":"environment"}`))
	}))
	defer envServer.Close()
	body, _ := json.Marshal(config{APIURL: fileServer.URL, DiscordUserID: "file-actor", Token: "file-secret", DiscordBotToken: "file-bot"})
	writeTestConfig(t, string(body))
	for _, key := range []string{"PROBST_API_URL", "PROBST_DISCORD_USER_ID", "PROBST_TOKEN", "CASTAWAY_DISCORD_BOT_TOKEN"} {
		t.Setenv(key, "")
		_ = os.Unsetenv(key)
	}
	out, err := runConfigCommand("auth", "status")
	if err != nil || !strings.Contains(out, `"source": "file"`) {
		t.Fatalf("file-only request: %v %s", err, out)
	}
	t.Setenv("PROBST_API_URL", envServer.URL)
	t.Setenv("PROBST_DISCORD_USER_ID", "env-actor")
	t.Setenv("PROBST_TOKEN", "env-secret")
	out, err = runConfigCommand("auth", "status", "--actor", "flag-actor")
	if err != nil || !strings.Contains(out, `"source": "environment"`) {
		t.Fatalf("environment request: %v %s", err, out)
	}
	t.Setenv("PROBST_API_URL", "http://not-loopback.example")
	out, err = runConfigCommand("auth", "status", "--server", envServer.URL, "--actor", "flag-actor")
	if err != nil || !strings.Contains(out, `"source": "environment"`) {
		t.Fatalf("flag override: %v %s", err, out)
	}
	if called != 3 {
		t.Fatalf("request count: %d", called)
	}
	for _, check := range []struct{ key, errPart string }{{"PROBST_TOKEN", "PROBST_TOKEN"}, {"PROBST_API_URL", "PROBST_API_URL"}, {"PROBST_DISCORD_USER_ID", "PROBST_DISCORD_USER_ID"}} {
		t.Setenv("PROBST_API_URL", fileServer.URL)
		t.Setenv("PROBST_TOKEN", "env-secret")
		t.Setenv("PROBST_DISCORD_USER_ID", "env-actor")
		t.Setenv(check.key, "")
		if _, err := runConfigCommand("auth", "status"); err == nil || !strings.Contains(err.Error(), check.errPart) {
			t.Fatalf("empty %s should override file: %v", check.key, err)
		}
	}
}

func TestConfigFileDiscordToken(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer file-secret" || r.Header.Get("X-Discord-User-ID") != "123" {
			t.Error("invalid API credentials")
		}
		switch r.URL.Path {
		case "/instances/inst/contestants":
			_, _ = w.Write([]byte(`{"contestants":[{"id":"c1","name":"Ada Test"}]}`))
		case "/instances/inst/participants":
			_, _ = w.Write([]byte(`{"participants":[{"id":"p1","name":"Player","discord_user_id":"123"}]}`))
		case "/instances/inst/drafts/p1":
			_, _ = w.Write([]byte(`{"picks":[]}`))
		default:
			t.Errorf("unexpected API route: %s", r.URL.Path)
		}
	}))
	defer api.Close()
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot file-bot" {
			t.Error("invalid Discord credentials")
		}
		_, _ = w.Write([]byte(`[{"id":"1","content":"Ada Test","timestamp":"2026-09-24T00:00:00Z","author":{"id":"123","username":"Player"}}]`))
	}))
	defer discord.Close()
	body, _ := json.Marshal(config{APIURL: api.URL, DiscordUserID: "123", Token: "file-secret", DiscordBotToken: "file-bot"})
	writeTestConfig(t, string(body))
	for _, key := range []string{"PROBST_API_URL", "PROBST_DISCORD_USER_ID", "PROBST_TOKEN", "CASTAWAY_DISCORD_BOT_TOKEN"} {
		t.Setenv(key, "")
		_ = os.Unsetenv(key)
	}
	t.Setenv("PROBST_DISCORD_API_URL", discord.URL)
	out, err := runConfigCommand("draft", "import", "987654321", "--instance", "inst")
	if err != nil || !strings.Contains(out, "READY") || !strings.Contains(out, "Dry run") {
		t.Fatalf("file-sourced Discord dry run failed: %v %s", err, out)
	}
}

func TestConfigFileValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mode       os.FileMode
	}{
		{"malformed", `{"token":`, 0o600},
		{"trailing", `{} {}`, 0o600},
		{"unknown", `{"token":"secret","extra":"private-value"}`, 0o600},
		{"world-readable", `{"token":"secret"}`, 0o644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTestConfig(t, tc.body)
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			_, err := runConfigCommand("auth", "status")
			if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private-value") {
				t.Fatalf("expected sanitized error: %v", err)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		cfg, err := loadConfig()
		if err != nil || !reflect.DeepEqual(cfg, config{}) {
			t.Fatalf("optional config: %v %+v", err, cfg)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		path := writeTestConfig(t, `{}`)
		if err := os.Rename(path, path+".target"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path+".target", path); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfig(); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("directory", func(t *testing.T) {
		path := writeTestConfig(t, `{}`)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfig(); err == nil {
			t.Fatal("directory accepted")
		}
	})
}

func TestConfigAliases(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"instance_id": "inst-uuid"}, "leaderboard": []any{}})
	}))
	defer server.Close()
	writeTestConfig(t, `{"api_url":"`+server.URL+`","discord_user_id":"1","token":"t",
		"aliases":{"podracing":{"instance":"inst-uuid","guild":"521","channel":"1330"}}}`)
	for _, args := range [][]string{
		{"scores", "--instance", "podracing"},
		{"channel", "show", "podracing", "--instance", "podracing"},
		{"channel", "show", "999", "--guild", "7"},
		{"scores", "--instance", "raw-uuid"},
	} {
		if out, err := runConfigCommand(args...); err != nil {
			t.Fatalf("%v: %q %v", args, out, err)
		}
	}
	want := "GET /instances/inst-uuid/leaderboard\nGET /discord/guilds/521/channels/1330\nGET /discord/guilds/7/channels/999\nGET /instances/raw-uuid/leaderboard"
	if got := strings.Join(calls, "\n"); got != want {
		t.Fatalf("calls:\n%s", got)
	}
}

func TestInstanceAliasSuppliesChannel(t *testing.T) {
	var calls []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"instance_id": "inst-uuid"}, "announcement": map[string]any{"id": "a"}})
	}))
	defer api.Close()
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "discord "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "9"})
	}))
	defer discord.Close()
	t.Setenv("PROBST_DISCORD_API_URL", discord.URL)
	writeTestConfig(t, `{"api_url":"`+api.URL+`","discord_user_id":"1","token":"t","discord_bot_token":"b",
		"aliases":{"podracing":{"instance":"inst-uuid","guild":"521","channel":"1330"}}}`)
	file := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"message", "hi", "--instance", "podracing", "--yes"},
		{"message", "777", "hi", "--instance", "podracing", "--yes"},
		{"message", "--file", file, "--instance", "podracing", "--yes"},
		{"announcement", "save", "week-2", "--file", file, "--instance", "podracing", "--yes"},
		{"announcement", "send", "--file", file, "--instance", "podracing", "--yes"},
	} {
		if out, err := runConfigCommand(args...); err != nil {
			t.Fatalf("%v: %q %v", args, out, err)
		}
	}
	want := "discord /channels/1330/messages\ndiscord /channels/777/messages\ndiscord /channels/1330/messages\n" +
		"GET /discord/guilds/521/channels/1330\nPOST /instances/inst-uuid/announcements\n" +
		"GET /discord/guilds/521/channels/1330\nPOST /instances/inst-uuid/announcements"
	if got := strings.Join(calls, "\n"); got != want {
		t.Fatalf("calls:\n%s", got)
	}
	if _, err := runConfigCommand("message", "hi", "--instance", "raw-uuid", "--yes"); err == nil || !strings.Contains(err.Error(), "CHANNEL") {
		t.Fatalf("no channel should be an error: %v", err)
	}
}
