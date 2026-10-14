package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runProbst(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // never read the operator's real config
	cmd := newCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestAnnouncementDraftWorkflowByName(t *testing.T) {
	t.Setenv("PROBST_TOKEN", "test-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	var calls []string
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		switch {
		case r.URL.Path == "/discord/guilds/101/channels/201":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"instance_id": "inst"}})
		case r.Method == "GET" && r.URL.Path == "/instances/inst/announcements":
			_ = json.NewEncoder(w).Encode(map[string]any{"announcements": []map[string]any{{"id": "a-1", "request_key": "kickoff", "status": "draft", "body": "old", "channel_id": "201", "due_at": "2026-09-30T00:00:00Z"}}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"announcement": map[string]any{"id": "a-1", "status": "pending", "due_at": "2026-09-30T23:00:00Z"}})
		}
	}))
	defer server.Close()
	t.Setenv("PROBST_API_URL", server.URL)
	file := filepath.Join(t.TempDir(), "kickoff.md")
	if err := os.WriteFile(file, []byte("🔥 Season 51"), 0600); err != nil {
		t.Fatal(err)
	}
	flags := []string{"--instance", "inst", "--guild", "101"}
	if out, err := runProbst(t, append([]string{"announcement", "save", "kickoff", "201", "--file", file}, flags...)...); err != nil || !strings.Contains(out, "Dry run") {
		t.Fatalf("save dry run: %q %v", out, err)
	}
	steps := [][]string{
		{"announcement", "save", "kickoff", "201", "--file", file, "--yes"},
		{"announcement", "edit", "kickoff", "--file", file, "--yes"},
		{"announcement", "schedule", "kickoff", "--at", "2026-09-30 19:00"},
		{"announcement", "unschedule", "kickoff"},
	}
	for _, step := range steps {
		if out, err := runProbst(t, append(step, flags...)...); err != nil {
			t.Fatalf("%v: %q %v", step, out, err)
		}
	}
	if _, err := runProbst(t, append([]string{"announcement", "schedule", "kickoff", "--yes"}, flags...)...); err == nil {
		t.Fatal("schedule without --at should refuse: nothing sends without approval")
	}
	if _, err := runProbst(t, append([]string{"announcement", "delete", "kickoff"}, flags...)...); err == nil {
		t.Fatal("delete without --yes should refuse")
	}
	want := []string{
		"GET /discord/guilds/101/channels/201",
		"GET /discord/guilds/101/channels/201", "POST /instances/inst/announcements",
		"GET /instances/inst/announcements", "PUT /instances/inst/announcements/a-1/body",
		"GET /instances/inst/announcements", "PUT /instances/inst/announcements/a-1/approval",
		"GET /instances/inst/announcements", "DELETE /instances/inst/announcements/a-1/schedule",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	if bodies[2]["draft"] != true || bodies[2]["request_key"] != "kickoff" || bodies[4]["body"] != "🔥 Season 51" || bodies[6]["send_at"] != "2026-09-30T23:00:00Z" {
		t.Fatalf("bodies: %v", bodies)
	}
}

func TestMessageWaitsForApproval(t *testing.T) {
	t.Setenv("PROBST_TOKEN", "test-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	var calls []string
	var bodies []map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"announcement": map[string]any{"id": "m-1"}})
	}))
	defer api.Close()
	t.Setenv("PROBST_API_URL", api.URL)
	flags := []string{"--instance", "inst", "--guild", "101"}
	if out, err := runProbst(t, append([]string{"message", "201", "The tribe has spoken, <@456>."}, flags...)...); err != nil || !strings.Contains(out, "Dry run") || len(calls) != 0 {
		t.Fatalf("dry run: %q %v calls=%v", out, err, calls)
	}
	out, err := runProbst(t, append([]string{"message", "201", "Buff for <@456>", "--notify", "--yes"}, flags...)...)
	if err != nil || !strings.Contains(out, "approval") {
		t.Fatalf("send: %q %v", out, err)
	}
	if strings.Join(calls, ",") != "POST /instances/inst/announcements,PUT /instances/inst/announcements/m-1/approval" ||
		bodies[0]["draft"] != true || bodies[0]["notify_users"] != true || bodies[0]["channel_id"] != "201" || bodies[1]["send_at"] == nil {
		t.Fatalf("message must be saved as a draft and held for approval: %v %v", calls, bodies)
	}
}
