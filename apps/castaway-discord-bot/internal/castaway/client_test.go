package castaway

import (
	"context"
	"encoding/json"
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
		if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Mooney"},"instance":{"id":"i1","name":"Season 50","season":50,"created_at":"2026-03-01T00:00:00Z"},"activities":[{"activity":{"id":"a1","instance_id":"i1","activity_type":"journey","name":"Journey 1","status":"completed","starts_at":"2026-03-12T00:00:00Z","created_at":"2026-03-12T00:00:00Z","updated_at":"2026-03-12T00:00:00Z"},"occurrences":[{"occurrence":{"id":"o1","activity_id":"a1","occurrence_type":"secret_risk_result","name":"Lost for Words — Mooney","effective_at":"2026-03-14T02:00:00Z","status":"resolved","created_at":"2026-03-14T02:00:00Z","updated_at":"2026-03-14T02:00:00Z"},"involvement":{"participant_id":"p1","participant_group_name":"Leaf","role":"delegate","result":"risked"},"ledger":[{"id":"l1","participant_id":"p1","participant_name":"Mooney","entry_kind":"award","points":1,"visibility":"secret","reason":"secret bonus","effective_at":"2026-03-14T02:00:00Z"}]}]}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	history, err := client.GetParticipantActivityHistory(context.Background(), "i1", "p1", "")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if history.Participant.Name != "Mooney" || len(history.Activities) != 1 || len(history.Activities[0].Occurrences) != 1 {
		t.Fatalf("unexpected history: %#v", history)
	}
}

func TestGetBonusLedgerSendsDiscordUserHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/participants/p1/bonus-ledger" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Discord-User-ID"); got != "user-1" {
			t.Fatalf("unexpected discord user header: %q", got)
		}
		if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Bryan"},"bonus_points":5,"ledger":[{"id":"l1","participant_id":"p1","participant_name":"Bryan","entry_kind":"award","points":5,"visibility":"secret","reason":"secret award","effective_at":"2026-03-14T02:00:00Z"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ledger, err := client.GetBonusLedger(context.Background(), "i1", "p1", "user-1")
	if err != nil {
		t.Fatalf("get bonus ledger: %v", err)
	}
	if ledger.Participant.Name != "Bryan" || ledger.BonusPoints != 5 || len(ledger.Ledger) != 1 || ledger.Ledger[0].Visibility != "secret" {
		t.Fatalf("unexpected ledger: %#v", ledger)
	}
}

func TestLinkAndLookupDiscordUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/instances/i1/participants/me":
			if got := r.Header.Get("X-Discord-User-ID"); got != "user-1" {
				t.Fatalf("unexpected discord user header: %q", got)
			}
			if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Bryan"}}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		case r.Method == http.MethodPut && r.URL.Path == "/instances/i1/participants/p1/discord-link":
			if got := r.Header.Get("X-Discord-User-ID"); got != "user-1" {
				t.Fatalf("unexpected discord user header: %q", got)
			}
			if got := r.URL.Query().Get("discord_user_id"); got != "user-2" {
				t.Fatalf("unexpected target discord user query: %q", got)
			}
			if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Bryan"}}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/instances/i1/participants/p1/discord-link":
			if got := r.Header.Get("X-Discord-User-ID"); got != "user-1" {
				t.Fatalf("unexpected discord user header: %q", got)
			}
			if _, err := w.Write([]byte(`{"participant":{"id":"p1","name":"Bryan"}}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	participant, err := client.GetLinkedParticipant(context.Background(), "i1", "user-1")
	if err != nil || participant.ID != "p1" {
		t.Fatalf("get linked participant: participant=%#v err=%v", participant, err)
	}
	participant, err = client.LinkDiscordUser(context.Background(), "i1", "p1", "user-1", "user-2")
	if err != nil || participant.ID != "p1" {
		t.Fatalf("link discord user: participant=%#v err=%v", participant, err)
	}
	participant, err = client.UnlinkDiscordUser(context.Background(), "i1", "p1", "user-1")
	if err != nil || participant.ID != "p1" {
		t.Fatalf("unlink discord user: participant=%#v err=%v", participant, err)
	}
}

func TestListContestantsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instances/i1/contestants" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if _, err := w.Write([]byte(`{"contestants":[{"id":"c1","name":"Joe"}]}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	contestants, err := client.ListContestants(context.Background(), "i1")
	if err != nil {
		t.Fatalf("list contestants: %v", err)
	}
	if len(contestants) != 1 || contestants[0].Name != "Joe" {
		t.Fatalf("unexpected contestants: %#v", contestants)
	}
}

func TestSetAuctionBidSendsDiscordHeaderAndJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/instances/i1/auction/contestants/c1/bid/me" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Discord-User-ID"); got != "user-1" {
			t.Fatalf("unexpected discord header: %q", got)
		}
		var body struct {
			ParticipantID string `json:"participant_id"`
			Points        int    `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Points != 4 || body.ParticipantID != "participant-1" {
			t.Fatalf("unexpected body: %+v", body)
		}
		if _, err := w.Write([]byte(`{"contestant":{"id":"c1","name":"Joe"},"lot_id":"lot-1","my_bid_points":4,"previous_bid_points":1,"bonus_points_available":6}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.SetAuctionBid(context.Background(), "i1", "c1", "user-1", "participant-1", 4)
	if err != nil {
		t.Fatalf("set auction bid: %v", err)
	}
	if result.MyBidPoints != 4 || result.BonusPointsAvailable != 6 || result.Contestant.Name != "Joe" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestAddStirThePotContributionSendsOptionalParticipantID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/instances/i1/stir-the-pot/me/contributions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Discord-User-ID"); got != "admin-1" {
			t.Fatalf("unexpected discord header: %q", got)
		}
		var body struct {
			ParticipantID string `json:"participant_id"`
			Points        int    `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Points != 1 || body.ParticipantID != "participant-keith" {
			t.Fatalf("unexpected body: %+v", body)
		}
		if _, err := w.Write([]byte(`{"participant":{"id":"participant-keith","name":"Keith"},"added_points":1,"my_contribution_points":1,"bonus_points_available":3}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	result, err := client.AddStirThePotContribution(context.Background(), "i1", "admin-1", "participant-keith", 1)
	if err != nil {
		t.Fatalf("add stir the pot contribution: %v", err)
	}
	if result.Participant.Name != "Keith" || result.AddedPoints != 1 {
		t.Fatalf("unexpected result: %#v", result)
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
