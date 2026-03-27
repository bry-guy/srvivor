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

	client, err := NewClient(server.URL, nil, Options{})
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

	client, err := NewClient(server.URL, nil, Options{})
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
		if _, err := w.Write([]byte(`{"leaderboard":[{"participant_id":"p1","participant_name":"Bryan","score":21,"draft_points":18,"bonus_points":3,"total_points":21,"points_available":46}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	rows, err := client.GetLeaderboard(context.Background(), "i1", "p1")
	if err != nil {
		t.Fatalf("get leaderboard: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	row := rows[0]
	if row.ParticipantID != "p1" || row.DraftPoints != 18 || row.BonusPoints != 3 || row.TotalPoints != 21 || row.PointsAvailable != 46 {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestClientAddsBearerAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer shared-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if _, err := w.Write([]byte(`{"instances":[]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{BearerToken: "shared-token"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.ListInstances(context.Background(), ListInstancesOptions{})
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
}

func TestListActivitiesParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/activities" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"activities":[{"id":"a1","instance_id":"i1","activity_type":"tribal_pony","name":"Tribal Pony","status":"active","starts_at":"2026-03-05T00:00:00Z","created_at":"2026-03-01T00:00:00Z","updated_at":"2026-03-01T00:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	activities, err := client.ListActivities(context.Background(), "i1")
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	a := activities[0]
	if a.ID != "a1" || a.ActivityType != "tribal_pony" || a.Name != "Tribal Pony" || a.Status != "active" {
		t.Fatalf("unexpected activity: %#v", a)
	}
}

func TestListOccurrencesParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activities/a1/occurrences" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"occurrences":[{"id":"o1","activity_id":"a1","occurrence_type":"immunity_result","name":"Episode 1 Immunity","effective_at":"2026-03-05T01:00:00Z","status":"resolved","created_at":"2026-03-05T02:00:00Z","updated_at":"2026-03-05T02:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	occurrences, err := client.ListOccurrences(context.Background(), "a1")
	if err != nil {
		t.Fatalf("list occurrences: %v", err)
	}
	if len(occurrences) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occurrences))
	}
	o := occurrences[0]
	if o.ID != "o1" || o.OccurrenceType != "immunity_result" || o.Name != "Episode 1 Immunity" || o.Status != "resolved" {
		t.Fatalf("unexpected occurrence: %#v", o)
	}
}

func TestGetActivityParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activities/a1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"activity":{"id":"a1","instance_id":"i1","activity_type":"journey","name":"Journey 1","status":"completed","starts_at":"2026-03-12T00:00:00Z","created_at":"2026-03-12T00:00:00Z","updated_at":"2026-03-12T00:00:00Z"},"group_assignments":[{"participant_group_id":"g1","participant_group_name":"Leaf","role":"tribe","starts_at":"2026-03-12T00:00:00Z","configuration":{"pony_survivor_tribe":"leaf"}}],"participant_assignments":[{"participant_id":"p1","participant_name":"Mooney","participant_group_id":"g1","participant_group_name":"Leaf","role":"delegate","starts_at":"2026-03-12T00:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	detail, err := client.GetActivity(context.Background(), "a1")
	if err != nil {
		t.Fatalf("get activity: %v", err)
	}
	if detail.Activity.Name != "Journey 1" || len(detail.GroupAssignments) != 1 || len(detail.ParticipantAssignments) != 1 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestGetOccurrenceParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/occurrences/o1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"occurrence":{"id":"o1","activity_id":"a1","occurrence_type":"journey_resolution","name":"Journey 1 Tribal Diplomacy","effective_at":"2026-03-14T01:00:00Z","status":"resolved","created_at":"2026-03-14T01:00:00Z","updated_at":"2026-03-14T01:00:00Z"},"participants":[{"participant_id":"p1","participant_name":"Adam","participant_group_id":"g1","participant_group_name":"Tangerine","role":"delegate","result":"STEAL"}],"groups":[],"ledger":[{"id":"l1","participant_id":"p2","participant_name":"Katie","entry_kind":"award","points":1,"visibility":"public","reason":"Lotus award","effective_at":"2026-03-14T01:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	detail, err := client.GetOccurrence(context.Background(), "o1")
	if err != nil {
		t.Fatalf("get occurrence: %v", err)
	}
	if detail.Occurrence.Name != "Journey 1 Tribal Diplomacy" || len(detail.Participants) != 1 || len(detail.Ledger) != 1 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestGetParticipantActivityHistoryParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/participants/p1/activity-history" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Mooney"},"instance":{"id":"i1","name":"Season 50","season":50,"created_at":"2026-03-01T00:00:00Z"},"history":[{"activity_id":"a1","activity_name":"Journey 1","activity_type":"journey","occurrence_id":"o1","occurrence_name":"Lost for Words — Mooney","occurrence_type":"secret_risk_result","effective_at":"2026-03-14T02:00:00Z","summary":"risk attempt","ledger":[{"id":"l1","participant_id":"p1","participant_name":"Mooney","entry_kind":"award","points":1,"visibility":"secret","reason":"secret bonus","effective_at":"2026-03-14T02:00:00Z"}]}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	history, err := client.GetParticipantActivityHistory(context.Background(), "i1", "p1")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if history.Participant.Name != "Mooney" || len(history.History) != 1 {
		t.Fatalf("unexpected history: %#v", history)
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

	client, err := NewClient(server.URL, nil, Options{})
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
