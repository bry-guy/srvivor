package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func numberedDraft(order []int) string {
	lines := make([]string, len(order))
	for i, idx := range order {
		lines[i] = fmt.Sprintf("%d. %s", i+1, season51Names[idx])
	}
	return strings.Join(lines, "\n")
}

func identity() []int {
	order := make([]int, len(season51Names))
	for i := range order {
		order[i] = i
	}
	return order
}

func reversed() []int {
	order := identity()
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order
}

func TestDraftImportFromThread(t *testing.T) {
	var mu sync.Mutex
	puts := map[string][]string{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "GET" && r.URL.Path == "/instances/inst/contestants":
			rows := []map[string]string{}
			for i, n := range season51Names {
				rows = append(rows, map[string]string{"id": fmt.Sprintf("c%02d", i), "name": n})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"contestants": rows})
		case r.Method == "GET" && r.URL.Path == "/instances/inst/participants":
			_ = json.NewEncoder(w).Encode(map[string]any{"participants": []map[string]string{
				{"id": "p-ada", "name": "Ada", "discord_user_id": "111"},
				{"id": "p-ben", "name": "Ben", "discord_user_id": "222"},
				{"id": "p-cora", "name": "Cora"},
				{"id": "p-dee", "name": "Dee", "discord_user_id": "444"},
			}})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/instances/inst/drafts/"):
			picks := []map[string]any{}
			if strings.HasSuffix(r.URL.Path, "p-ben") { // Ben's stored draft already matches his post
				for i := range season51Names {
					picks = append(picks, map[string]any{"position": i + 1, "contestant_id": fmt.Sprintf("c%02d", i)})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"picks": picks})
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/instances/inst/drafts/"):
			var body struct {
				ContestantIDs []string `json:"contestant_ids"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			puts[strings.TrimPrefix(r.URL.Path, "/instances/inst/drafts/")] = body.ContestantIDs
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			t.Errorf("unexpected API call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()

	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	type msg struct {
		author, content string
		at              time.Duration
	}
	posts := []msg{
		{"111", "who's excited!! Kilby is going far", time.Minute},
		{"111", numberedDraft(reversed()), 2 * time.Minute},
		{"222", "here's mine\n" + numberedDraft(identity()) + "\ngl all", 3 * time.Minute},
		{"333", numberedDraft(identity()), 4 * time.Minute},
		{"111", "actually changed my mind:\n" + numberedDraft(identity()), 5 * time.Minute},
		{"444", numberedDraft(identity()), 48 * time.Hour},
	}
	for i := 0; i < 120; i++ { // chatter, enough to force a second page
		posts = append(posts, msg{"222", "lol Jelly and Rob tho " + strconv.Itoa(i), 6*time.Minute + time.Duration(i)*time.Second})
	}
	// Discord returns newest first; IDs grow with time.
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot bot-secret" || r.URL.Path != "/channels/999/messages" {
			t.Errorf("bad Discord request %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		before := len(posts)
		if b := r.URL.Query().Get("before"); b != "" {
			before, _ = strconv.Atoi(b)
		}
		page := []map[string]any{}
		for i := before - 1; i >= 0 && len(page) < limit; i-- {
			page = append(page, map[string]any{
				"id": strconv.Itoa(i), "content": posts[i].content, "timestamp": start.Add(posts[i].at).Format(time.RFC3339Nano),
				"author": map[string]string{"id": posts[i].author, "username": "user" + posts[i].author},
			})
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer discord.Close()

	t.Setenv("PROBST_TOKEN", "api-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	t.Setenv("PROBST_API_URL", api.URL)
	t.Setenv("CASTAWAY_DISCORD_BOT_TOKEN", "bot-secret")
	t.Setenv("PROBST_DISCORD_API_URL", discord.URL)
	run := func(args ...string) string {
		t.Helper()
		cmd := newCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.String())
		}
		return out.String()
	}
	url := "https://discord.com/channels/521073437779689474/999"
	cutoff := start.Add(24 * time.Hour).Format(time.RFC3339)

	out := run("draft", "import", url, "--instance", "inst", "--before", cutoff)
	for _, want := range []string{"Ada", "READY", "latest of 2 drafts", "UNCHANGED", "UNLINKED", "user333 (333)", "Dee", "only drafted after the cutoff", "Cora", "player has no Discord link", "Dry run"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output missing %q:\n%s", want, out)
		}
	}
	if len(puts) != 0 {
		t.Fatalf("dry run wrote drafts: %v", puts)
	}

	out = run("draft", "import", url, "--instance", "inst", "--before", cutoff, "--yes")
	if !strings.Contains(out, "SUBMITTED") || len(puts) != 1 || len(puts["p-ada"]) != len(season51Names) || puts["p-ada"][0] != "c00" {
		t.Fatalf("expected only Ada's latest draft submitted, got %v\n%s", puts, out)
	}
}

func TestDraftImportFromFileNeverSubmitsBrokenDraft(t *testing.T) {
	writes := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/instances/inst/contestants":
			rows := []map[string]string{}
			for i, n := range season51Names {
				rows = append(rows, map[string]string{"id": fmt.Sprintf("c%02d", i), "name": n})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"contestants": rows})
		case r.URL.Path == "/instances/inst/participants":
			_ = json.NewEncoder(w).Encode(map[string]any{"participants": []map[string]string{{"id": "p-ada", "name": "Ada"}}})
		default:
			writes++
			w.WriteHeader(500)
		}
	}))
	defer api.Close()
	t.Setenv("PROBST_TOKEN", "api-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	t.Setenv("PROBST_API_URL", api.URL)
	file := t.TempDir() + "/ada.txt"
	lines := strings.Split(numberedDraft(identity()), "\n")
	lines[20] = "21. An"
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"draft", "import", "--instance", "inst", "--file", file, "--participant", "ada", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if writes != 0 || !strings.Contains(out.String(), "NEEDS REVIEW") || !strings.Contains(out.String(), "missing Thien An Nguyen") {
		t.Fatalf("broken draft handled wrongly (writes=%d):\n%s", writes, out.String())
	}
}
