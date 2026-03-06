package castaway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListInstancesSendsFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("season"); got != "49" {
			t.Fatalf("expected season filter, got %q", got)
		}
		if got := r.URL.Query().Get("name"); got != "office" {
			t.Fatalf("expected name filter, got %q", got)
		}
		if _, err := w.Write([]byte(`{"instances":[{"id":"i1","name":"Office","season":49,"created_at":"2026-01-01T00:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	season := int32(49)
	instances, err := client.ListInstances(context.Background(), ListInstancesOptions{Season: &season, Name: "office"})
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(instances) != 1 || instances[0].ID != "i1" {
		t.Fatalf("unexpected instances: %#v", instances)
	}
}

func TestListParticipantsSendsNameFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/participants" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("name"); got != "bry" {
			t.Fatalf("expected name filter, got %q", got)
		}
		if _, err := w.Write([]byte(`{"participants":[{"id":"p1","name":"Bryan"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	participants, err := client.ListParticipants(context.Background(), "i1", ListParticipantsOptions{Name: "bry"})
	if err != nil {
		t.Fatalf("list participants: %v", err)
	}
	if len(participants) != 1 || participants[0].ID != "p1" {
		t.Fatalf("unexpected participants: %#v", participants)
	}
}

func TestGetLeaderboardSendsParticipantFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/leaderboard" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("participant_id"); got != "p1" {
			t.Fatalf("expected participant filter, got %q", got)
		}
		if _, err := w.Write([]byte(`{"leaderboard":[{"participant_id":"p1","participant_name":"Bryan","score":21,"points_available":46}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	rows, err := client.GetLeaderboard(context.Background(), "i1", "p1")
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(rows) != 1 || rows[0].ParticipantID != "p1" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestGetJSONReturnsTypedAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error":"instance not found"}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.GetInstance(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound || apiErr.Message != "instance not found" {
		t.Fatalf("unexpected api error: %#v", apiErr)
	}
}
