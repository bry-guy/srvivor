package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommandHTTPBoundary(t *testing.T) {
	t.Setenv("PROBST_TOKEN", "test-secret")
	t.Setenv("PROBST_DISCORD_USER_ID", "123")
	calls := 0
	status := http.StatusOK
	location := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-secret" || r.Header.Get("X-Discord-User-ID") != "123" {
			t.Error("missing delegated credentials")
		}
		if location != "" {
			w.Header().Set("Location", location)
		}
		w.WriteHeader(status)
		if _, err := fmt.Fprint(w, `{"error":"denied","principal":"test-service"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	t.Setenv("PROBST_API_URL", server.URL)
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := newCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	out, err := run("auth", "status", "--json")
	if err != nil || !strings.Contains(out, "test-service") {
		t.Fatalf("status: %s %v", out, err)
	}
	before := calls
	for _, args := range [][]string{{"player", "unlink", "player", "--instance", "instance"}, {"channel", "unbind", "channel", "--guild", "guild"}} {
		if _, err := run(args...); err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("confirmation missing: %v", err)
		}
	}
	if calls != before {
		t.Fatal("unconfirmed destructive request sent")
	}
	status = http.StatusForbidden
	if _, err := run("scores", "--instance", "instance"); err == nil || !strings.Contains(err.Error(), "403: denied") {
		t.Fatalf("HTTP failure: %v", err)
	}
	destinationCalls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { destinationCalls++; w.WriteHeader(200) }))
	defer destination.Close()
	status = http.StatusTemporaryRedirect
	location = destination.URL
	if _, err := run("auth", "status"); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("redirect: %v", err)
	}
	if destinationCalls != 0 {
		t.Fatal("credentials followed redirect")
	}
	t.Setenv("PROBST_API_URL", "http://example.com")
	if _, err := run("auth", "status"); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("insecure URL: %v", err)
	}
	t.Setenv("PROBST_TOKEN", "")
	if _, err := run("auth", "status"); err == nil || !strings.Contains(err.Error(), "PROBST_TOKEN") {
		t.Fatalf("missing credentials: %v", err)
	}
}
