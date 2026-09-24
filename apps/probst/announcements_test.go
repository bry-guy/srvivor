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

func TestAnnouncementPreviewAndConfirmedSubmission(t *testing.T) {
	t.Setenv("PROBST_TOKEN", "test-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	const text = "📣 **Season update** <@456>\n\nNext episode soon.\n"
	var posted []map[string]any
	boundInstance := "instance-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" || r.Header.Get("X-Discord-User-ID") != "123" {
			t.Error("missing trusted service credentials")
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/discord/guilds/101/channels/201":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"instance_id": boundInstance}})
		case r.Method == "POST" && r.URL.Path == "/instances/instance-1/announcements":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			posted = append(posted, body)
			_ = json.NewEncoder(w).Encode(map[string]any{"announcement": map[string]any{"id": "created"}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("PROBST_API_URL", server.URL)
	file := filepath.Join(t.TempDir(), "update.md")
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(confirmed bool, extra ...string) (string, error) {
		t.Helper()
		cmd := newCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		args := append([]string{"announcement", "send", "201", "--instance", "instance-1", "--guild", "101", "--file", file}, extra...)
		if confirmed {
			args = append(args, "--yes")
		}
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	preview, err := run(false)
	if err != nil || !strings.Contains(preview, text) || len(posted) != 0 {
		t.Fatalf("dry run: %q %v; sent=%d", preview, err, len(posted))
	}
	if _, err := run(true); err != nil {
		t.Fatal(err)
	}
	if _, err := run(true); err != nil {
		t.Fatal(err)
	}
	if len(posted) != 2 || posted[0]["body"] != text || posted[0]["request_key"] != posted[1]["request_key"] || posted[0]["scheduled_at"] != nil {
		t.Fatalf("submission was altered or not retry stable: %v", posted)
	}
	if _, err := run(true, "--at", "2026-10-01 20:00"); err != nil {
		t.Fatal(err)
	}
	if posted[2]["scheduled_at"] != "2026-10-02T00:00:00Z" || posted[2]["request_key"] == posted[0]["request_key"] {
		t.Fatalf("Eastern time was not scheduled as a distinct UTC request: %v", posted[2])
	}
	if _, err := run(true, "--at", "tomorrow"); err == nil || len(posted) != 3 {
		t.Fatalf("bad --at accepted: %v", err)
	}
	posted = posted[:2]
	boundInstance = "other-instance"
	if _, err := run(true); err == nil || !strings.Contains(err.Error(), "not bound") || len(posted) != 2 {
		t.Fatalf("unbound channel accepted: %v; sent=%d", err, len(posted))
	}
}
