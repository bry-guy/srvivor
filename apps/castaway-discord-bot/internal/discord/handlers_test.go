package discord

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/state"
	"github.com/bwmarrin/discordgo"
)

type testCastawayAPI struct {
	instances              []castaway.Instance
	participantsByInstance map[string][]castaway.Participant
	leaderboardByInstance  map[string][]castaway.LeaderboardRow
	draftsByInstance       map[string]map[string]castaway.Draft
	activitiesByInstance   map[string][]castaway.Activity
	activityDetails        map[string]castaway.ActivityDetail
	occurrencesByActivity  map[string][]castaway.Occurrence
	occurrenceDetails      map[string]castaway.OccurrenceDetail
	historyByParticipant   map[string]castaway.ParticipantActivityHistory
}

func TestScoreCommandRegression_UsesUserDefault(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-49", Name: "Historical Season 49", Season: 49}},
		participantsByInstance: map[string][]castaway.Participant{"instance-49": {{ID: "participant-bryan", Name: "Bryan"}}},
		leaderboardByInstance:  map[string][]castaway.LeaderboardRow{"instance-49": {{ParticipantID: "participant-bryan", ParticipantName: "Bryan", Score: 81, DraftPoints: 76, BonusPoints: 5, TotalPoints: 81, PointsAvailable: -198}}},
	})

	if err := store.SetUserDefault("guild-1", "user-1", "instance-49"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "score", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := "**Season 49 — Historical Season 49**\nBryan — 81 points (76+5; points available: -198)"
	if message != expected {
		t.Fatalf("unexpected score message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestScoresCommandRegression_ResolvesSingleSeasonInstance(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{
		instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		leaderboardByInstance: map[string][]castaway.LeaderboardRow{"instance-50": {
			{ParticipantID: "participant-keeling", ParticipantName: "Keeling", Score: 6, DraftPoints: 5, BonusPoints: 1, TotalPoints: 6, PointsAvailable: 294},
			{ParticipantID: "participant-adam", ParticipantName: "Adam", Score: 5, DraftPoints: 5, BonusPoints: 0, TotalPoints: 5, PointsAvailable: 292},
			{ParticipantID: "participant-amanda", ParticipantName: "Amanda", Score: 3, DraftPoints: 2, BonusPoints: 1, TotalPoints: 3, PointsAvailable: 281},
		}},
	})

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "scores", options: []*discordgo.ApplicationCommandInteractionDataOption{intOption("season", 50)}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50 — Historical Season 50**", "1. Keeling — 6 (5+1)", "2. Adam — 5 (5+0)", "3. Amanda — 3 (2+1)"}, "\n")
	if message != expected {
		t.Fatalf("unexpected leaderboard message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestDraftCommandRegression_UsesGuildDefault(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-bryan", Name: "Bryan"}}},
		draftsByInstance:       map[string]map[string]castaway.Draft{"instance-50": {"participant-bryan": {Participant: castaway.Participant{ID: "participant-bryan", Name: "Bryan"}, Picks: []castaway.DraftPick{{Position: 1, ContestantID: "emily", ContestantName: "Emily"}, {Position: 2, ContestantID: "christian", ContestantName: "Christian"}, {Position: 3, ContestantID: "q", ContestantName: "Q"}}}}},
	})

	if err := store.SetGuildDefault("guild-1", "instance-50"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "draft", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Bryan")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Bryan Draft** — Season 50 — Historical Season 50", "1. Emily", "2. Christian", "3. Q"}, "\n")
	if message != expected {
		t.Fatalf("unexpected draft message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestInstanceCommandRegression_UserDefaultLifecycle(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}})
	interaction := testInteraction("guild-1", "user-1", 0)

	setMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "set", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("instance", "Historical Season 50"), intOption("season", 50)}})
	if err != nil {
		t.Fatalf("set default: %v", err)
	}
	if setMessage != "Saved your default instance: Season 50 — Historical Season 50" {
		t.Fatalf("unexpected set message: %q", setMessage)
	}

	showMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "show"})
	if err != nil {
		t.Fatalf("show defaults: %v", err)
	}
	showExpected := strings.Join([]string{"**Saved instance defaults**", "- You: Season 50 — Historical Season 50", "- Guild: not set"}, "\n")
	if showMessage != showExpected {
		t.Fatalf("unexpected show message:\nexpected: %q\nactual:   %q", showExpected, showMessage)
	}

	clearMessage, err := bot.executeCommand(context.Background(), interaction, commandSpec{group: "instance", name: "clear"})
	if err != nil {
		t.Fatalf("clear defaults: %v", err)
	}
	if clearMessage != "Cleared your default instance." {
		t.Fatalf("unexpected clear message: %q", clearMessage)
	}
}

func TestInstanceCommandRegression_GuildScopeRequiresManageServer(t *testing.T) {
	bot, _ := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}})
	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{group: "instance", name: "set", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("instance", "Historical Season 50"), intOption("season", 50), stringOption("scope", "guild")}})
	if err == nil {
		t.Fatal("expected permission error")
	}
	if err.Error() != "guild scope requires Manage Server permission" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveInstanceRegression_ClearsStaleUserDefaultBeforeGuildFallback(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-49", Name: "Historical Season 49", Season: 49}}})
	if err := store.SetUserDefault("guild-1", "user-1", "stale-instance"); err != nil {
		t.Fatalf("set stale user default: %v", err)
	}
	if err := store.SetGuildDefault("guild-1", "instance-49"); err != nil {
		t.Fatalf("set guild default: %v", err)
	}

	instance, err := bot.resolveInstance(context.Background(), testInteraction("guild-1", "user-1", 0), "", nil)
	if err != nil {
		t.Fatalf("resolve instance: %v", err)
	}
	if instance.ID != "instance-49" {
		t.Fatalf("unexpected instance: %#v", instance)
	}
}

func TestActivitiesCommandRegression_ListsActivitiesForInstance(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}, activitiesByInstance: map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}, {ID: "act-2", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed"}}}})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "activities"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}

	expected := strings.Join([]string{"**Season 50 — Historical Season 50 — Activities**", "- **Tribal Pony** (tribal_pony) — active", "- **Journey 1** (journey) — completed"}, "\n")
	if message != expected {
		t.Fatalf("unexpected activities message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestOccurrencesCommandRegression_ListsOccurrencesWithImpactSummary(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		activitiesByInstance:  map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}}},
		occurrencesByActivity: map[string][]castaway.Occurrence{"act-1": {{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "immunity_result", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z", Status: "resolved"}}},
		occurrenceDetails:     map[string]castaway.OccurrenceDetail{"occ-1": {Occurrence: castaway.Occurrence{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "immunity_result", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z", Status: "resolved"}, Ledger: []castaway.BonusLedgerEntry{{ParticipantName: "Amanda", Points: 1, Visibility: "public"}, {ParticipantName: "Bryan", Points: 1, Visibility: "public"}}}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrences", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Tribal Pony")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !strings.Contains(message, "impact: Amanda — +1 public; Bryan — +1 public") {
		t.Fatalf("expected richer occurrence impact in %q", message)
	}
}

func TestOccurrenceCommandRegression_ShowsDetailedOccurrence(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:             []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		activitiesByInstance:  map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "journey", Name: "Journey 1", Status: "completed"}}},
		occurrencesByActivity: map[string][]castaway.Occurrence{"act-1": {{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "journey_resolution", Name: "Journey 1 Tribal Diplomacy", EffectiveAt: "2026-03-14T01:00:00Z", Status: "resolved"}}},
		occurrenceDetails: map[string]castaway.OccurrenceDetail{"occ-1": {
			Occurrence:   castaway.Occurrence{ID: "occ-1", ActivityID: "act-1", OccurrenceType: "journey_resolution", Name: "Journey 1 Tribal Diplomacy", EffectiveAt: "2026-03-14T01:00:00Z", Status: "resolved"},
			Participants: []castaway.OccurrenceParticipant{{ParticipantName: "Adam", ParticipantGroupName: "Tangerine", Role: "delegate", Result: "STEAL"}, {ParticipantName: "Katie", ParticipantGroupName: "Lotus", Role: "delegate", Result: "SHARE"}},
			Ledger:       []castaway.BonusLedgerEntry{{ParticipantName: "Katie", Points: 1, Visibility: "public"}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrence", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Journey 1"), stringOption("occurrence", "Journey 1 Tribal Diplomacy")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Journey 1 Tribal Diplomacy**", "**Recorded**", "Adam — role=delegate, result=STEAL, group=Tangerine", "**Impact**"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestHistoryCommandRegression_ShowsParticipantActivityHistory(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{
		instances:              []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}},
		participantsByInstance: map[string][]castaway.Participant{"instance-50": {{ID: "participant-mooney", Name: "Mooney"}}},
		historyByParticipant: map[string]castaway.ParticipantActivityHistory{"participant-mooney": {
			Participant: castaway.Participant{ID: "participant-mooney", Name: "Mooney"},
			Instance:    castaway.Instance{ID: "instance-50", Name: "Historical Season 50", Season: 50},
			History:     []castaway.ParticipantActivityHistoryEntry{{ActivityName: "Journey 1", ActivityType: "journey", OccurrenceName: "Lost for Words — Mooney", EffectiveAt: "2026-03-14T02:00:00Z", Summary: "risk attempt", Ledger: []castaway.BonusLedgerEntry{{ParticipantName: "Mooney", Points: 1, Visibility: "secret"}}}},
		}},
	})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	message, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "history", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("participant", "Mooney")}})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	for _, fragment := range []string{"**Mooney — Activity History**", "**Journey 1** (journey)", "impact: Mooney — +1 secret"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestOccurrencesCommandRegression_ActivityNotFound(t *testing.T) {
	bot, store := newTestBot(t, testCastawayAPI{instances: []castaway.Instance{{ID: "instance-50", Name: "Historical Season 50", Season: 50}}, activitiesByInstance: map[string][]castaway.Activity{"instance-50": {{ID: "act-1", InstanceID: "instance-50", ActivityType: "tribal_pony", Name: "Tribal Pony", Status: "active"}}}})
	if err := store.SetUserDefault("guild-1", "user-1", "instance-50"); err != nil {
		t.Fatalf("set user default: %v", err)
	}

	_, err := bot.executeCommand(context.Background(), testInteraction("guild-1", "user-1", 0), commandSpec{name: "occurrences", options: []*discordgo.ApplicationCommandInteractionDataOption{stringOption("activity", "Nonexistent")}})
	if err == nil {
		t.Fatal("expected error for nonexistent activity")
	}
	if !strings.Contains(err.Error(), "no activities matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestBot(t *testing.T, api testCastawayAPI) (*Bot, *state.BoltStore) {
	t.Helper()
	server := httptest.NewServer(api.handler(t))
	t.Cleanup(server.Close)

	client, err := castaway.NewClient(server.URL, server.Client(), castaway.Options{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	store, err := state.OpenBolt(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close state store: %v", err)
		}
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Bot{castaway: client, state: store, log: logger}, store
}

func (api testCastawayAPI) handler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		writeJSON := func(status int, payload any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "activities" && r.Method == http.MethodGet {
			detail, ok := api.activityDetails[parts[1]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "activity not found"})
				return
			}
			writeJSON(http.StatusOK, detail)
			return
		}
		if len(parts) == 3 && parts[0] == "activities" && parts[2] == "occurrences" && r.Method == http.MethodGet {
			occurrences := api.occurrencesByActivity[parts[1]]
			if occurrences == nil {
				occurrences = []castaway.Occurrence{}
			}
			writeJSON(http.StatusOK, map[string]any{"occurrences": occurrences})
			return
		}
		if len(parts) == 2 && parts[0] == "occurrences" && r.Method == http.MethodGet {
			detail, ok := api.occurrenceDetails[parts[1]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "occurrence not found"})
				return
			}
			writeJSON(http.StatusOK, detail)
			return
		}
		if len(parts) == 1 && parts[0] == "instances" {
			instances := api.instances
			if seasonRaw := strings.TrimSpace(r.URL.Query().Get("season")); seasonRaw != "" {
				season, err := strconv.Atoi(seasonRaw)
				if err != nil {
					writeJSON(http.StatusBadRequest, map[string]any{"error": "invalid season"})
					return
				}
				filtered := make([]castaway.Instance, 0, len(instances))
				for _, instance := range instances {
					if int(instance.Season) == season {
						filtered = append(filtered, instance)
					}
				}
				instances = filtered
			}
			if nameFilter := strings.TrimSpace(r.URL.Query().Get("name")); nameFilter != "" {
				filtered := make([]castaway.Instance, 0, len(instances))
				for _, instance := range instances {
					if containsFold(instance.Name, nameFilter) {
						filtered = append(filtered, instance)
					}
				}
				instances = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"instances": instances})
			return
		}

		if len(parts) < 2 || parts[0] != "instances" {
			writeJSON(http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		instanceID := parts[1]
		instance, ok := api.instanceByID(instanceID)
		if !ok {
			writeJSON(http.StatusNotFound, map[string]any{"error": "instance not found"})
			return
		}

		switch {
		case len(parts) == 2 && r.Method == http.MethodGet:
			writeJSON(http.StatusOK, map[string]any{"instance": instance})
		case len(parts) == 3 && parts[2] == "participants" && r.Method == http.MethodGet:
			participants := api.participantsByInstance[instanceID]
			if nameFilter := strings.TrimSpace(r.URL.Query().Get("name")); nameFilter != "" {
				filtered := make([]castaway.Participant, 0, len(participants))
				for _, participant := range participants {
					if containsFold(participant.Name, nameFilter) {
						filtered = append(filtered, participant)
					}
				}
				participants = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"participants": participants})
		case len(parts) == 3 && parts[2] == "activities" && r.Method == http.MethodGet:
			activities := api.activitiesByInstance[instanceID]
			if activities == nil {
				activities = []castaway.Activity{}
			}
			writeJSON(http.StatusOK, map[string]any{"activities": activities})
		case len(parts) == 3 && parts[2] == "leaderboard" && r.Method == http.MethodGet:
			rows := api.leaderboardByInstance[instanceID]
			if participantID := strings.TrimSpace(r.URL.Query().Get("participant_id")); participantID != "" {
				filtered := make([]castaway.LeaderboardRow, 0, len(rows))
				for _, row := range rows {
					if row.ParticipantID == participantID {
						filtered = append(filtered, row)
					}
				}
				rows = filtered
			}
			writeJSON(http.StatusOK, map[string]any{"leaderboard": rows})
		case len(parts) == 4 && parts[2] == "drafts" && r.Method == http.MethodGet:
			draft, ok := api.draftsByInstance[instanceID][parts[3]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "draft not found"})
				return
			}
			writeJSON(http.StatusOK, draft)
		case len(parts) == 5 && parts[2] == "participants" && parts[4] == "activity-history" && r.Method == http.MethodGet:
			history, ok := api.historyByParticipant[parts[3]]
			if !ok {
				writeJSON(http.StatusNotFound, map[string]any{"error": "history not found"})
				return
			}
			writeJSON(http.StatusOK, history)
		default:
			writeJSON(http.StatusNotFound, map[string]any{"error": "not found"})
		}
	})
}

func (api testCastawayAPI) instanceByID(id string) (castaway.Instance, bool) {
	for _, instance := range api.instances {
		if instance.ID == id {
			return instance, true
		}
	}
	return castaway.Instance{}, false
}

func testInteraction(guildID, userID string, permissions int64) *discordgo.InteractionCreate {
	interaction := &discordgo.Interaction{GuildID: guildID}
	if guildID == "" {
		interaction.User = &discordgo.User{ID: userID}
	} else {
		interaction.Member = &discordgo.Member{Permissions: permissions, User: &discordgo.User{ID: userID}}
	}
	return &discordgo.InteractionCreate{Interaction: interaction}
}

func stringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value}
}

func intOption(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{Name: name, Type: discordgo.ApplicationCommandOptionInteger, Value: float64(value)}
}

func containsFold(candidate, filter string) bool {
	return strings.Contains(strings.ToLower(candidate), strings.ToLower(filter))
}
