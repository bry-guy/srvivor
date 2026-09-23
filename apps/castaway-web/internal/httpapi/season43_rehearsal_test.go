package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type season43Fixture struct {
	Contestants []struct {
		ID, Name       string
		Episode, Place int
	}
	Players []struct {
		Name, Tribe string
		Picks       []string
	}
	Episodes []struct {
		Number   int32
		Date     string
		Guesses  []int
		Expected []struct {
			Player              string
			Draft, Bonus, Total int
		}
	}
}

func TestSeason43Rehearsal(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	if _, err := exec.LookPath("hurl"); err != nil {
		t.Fatal("rehearsal requires hurl on PATH")
	}
	raw, err := os.ReadFile("testdata/season43/fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture season43Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Contestants) != 18 || len(fixture.Players) != 6 || len(fixture.Episodes) != 13 {
		t.Fatal("invalid Season 43 fixture dimensions")
	}
	q := db.New(pool)
	start := time.Date(2022, 9, 20, 12, 0, 0, 0, time.UTC)
	clock := &wordleFixtureClock{now: start}
	server := httptest.NewServer(httpapi.New(pool, httpapi.WithClock(clock.Now), httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"rehearsal-token"}})).Router())
	defer server.Close()
	dir := t.TempDir()
	run := func(label, text string) []byte {
		t.Helper()
		input := filepath.Join(dir, label+".hurl")
		if err := os.WriteFile(input, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("hurl", "--retry", "0", "--secret", "service_token=rehearsal-token")
		command.Stdin = strings.NewReader(strings.ReplaceAll(text, "{{base_url}}", server.URL))
		var stderr bytes.Buffer
		command.Stderr = &stderr
		data, err := command.Output()
		if err != nil {
			t.Fatalf("%s: %v\n%s", label, err, stderr.String())
		}
		return data
	}
	names := make([]string, 0, 18)
	for _, c := range fixture.Contestants {
		names = append(names, c.Name)
	}
	raw = run("create", rehearsalHurl("POST", "/instances", map[string]any{"name": "Season 43 rehearsal " + uuid.NewString(), "season": 43, "contestants": names}, 201, ""))
	var created struct{ Instance struct{ ID string } }
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	instanceID := wordlePGUUID(uuid.MustParse(created.Instance.ID))
	path := "/instances/" + created.Instance.ID
	if _, err := q.CreateInstanceAdmin(ctx, db.CreateInstanceAdminParams{InstanceID: instanceID, DiscordUserID: "rehearsal-admin"}); err != nil {
		t.Fatal(err)
	}
	var setup strings.Builder
	for _, p := range fixture.Players {
		setup.WriteString(rehearsalHurl("POST", path+"/participants", map[string]any{"name": p.Name}, 201, ""))
	}
	setup.WriteString(rehearsalHurl("GET", path, nil, 200, ""))
	raw = run("roster", setup.String())
	var roster struct{ Contestants, Participants []struct{ ID, Name string } }
	if err := json.Unmarshal(raw, &roster); err != nil {
		t.Fatal(err)
	}
	contestantByName, contestantIDs, participantIDs := map[string]string{}, map[string]string{}, map[string]string{}
	for _, c := range roster.Contestants {
		contestantByName[c.Name] = c.ID
	}
	for _, c := range fixture.Contestants {
		contestantIDs[c.ID] = contestantByName[c.Name]
		if contestantIDs[c.ID] == "" {
			t.Fatalf("missing contestant %s", c.ID)
		}
	}
	for _, p := range roster.Participants {
		participantIDs[p.Name] = p.ID
	}
	groups := map[string]pgtype.UUID{}
	for _, name := range []string{"Ember", "Tide"} {
		groups[name] = createParticipantGroupForTest(t, ctx, q, instanceID, name, "tribe").ID
	}
	var drafts strings.Builder
	for _, p := range fixture.Players {
		createWordleMembership(t, ctx, q, groups[p.Tribe], wordlePGUUID(uuid.MustParse(participantIDs[p.Name])), start)
		picks := make([]string, 0, 18)
		for _, id := range p.Picks {
			picks = append(picks, contestantIDs[id])
		}
		drafts.WriteString(rehearsalHurl("PUT", path+"/drafts/"+participantIDs[p.Name], map[string]any{"contestant_ids": picks}, 200, ""))
	}
	drafts.WriteString(rehearsalHurl("POST", path+"/activities", map[string]any{"activity_type": "tribe_wordle", "name": "Weekly rehearsal Wordle", "status": "active", "starts_at": start}, 201, ""))
	raw = run("drafts", drafts.String())
	var activity struct{ Activity struct{ ID string } }
	if err := json.Unmarshal(raw, &activity); err != nil {
		t.Fatal(err)
	}
	activityID := wordlePGUUID(uuid.MustParse(activity.Activity.ID))
	for _, ep := range fixture.Episodes {
		date, err := time.Parse("2006-01-02", ep.Date)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.CreateInstanceEpisode(ctx, db.CreateInstanceEpisodeParams{InstanceID: instanceID, EpisodeNumber: ep.Number, Label: fmt.Sprintf("Episode %d", ep.Number), AirsAt: wordleTimestamp(date.Add(12 * time.Hour)), Metadata: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	post := rehearsalDiscord(t)
	wallStart := time.Now().Add(2 * time.Second)
	outcomeCount := 0
	var latest leaderboardResponse
	for index, ep := range fixture.Episodes {
		date, err := time.Parse("2006-01-02", ep.Date)
		if err != nil {
			t.Fatal(err)
		}
		opens := date.Add(12 * time.Hour)
		cutoff := opens.Add(time.Hour)
		clock.Set(opens)
		raw = run(fmt.Sprintf("round-%02d", ep.Number), rehearsalHurl("POST", "/activities/"+activity.Activity.ID+"/wordle-rounds", map[string]any{"round_key": fmt.Sprintf("week-%d", ep.Number), "name": fmt.Sprintf("Episode %d Wordle", ep.Number), "opens_at": opens, "cutoff_at": cutoff}, 201, ""))
		var round struct{ Round struct{ ID string } }
		if err := json.Unmarshal(raw, &round); err != nil {
			t.Fatal(err)
		}
		roundPath := "/wordle-rounds/" + round.Round.ID
		post(wallStart.Add(time.Duration(index)*6*time.Second), fmt.Sprintf("[TEST S43 E%02d — synthetic Wordle] Historical episode date: %s. Ember vs Tide: admin records three guess counts per tribe; lowest average earns +1 public bonus per member (ties both win). This accelerated round closes at the results post in ~3 seconds; no real submissions needed.", ep.Number, ep.Date))
		var entries strings.Builder
		for i, p := range fixture.Players {
			entries.WriteString(rehearsalHurl("PUT", roundPath+"/participants/"+participantIDs[p.Name], map[string]any{"participant_group_id": uuid.UUID(groups[p.Tribe].Bytes).String(), "guess_count": ep.Guesses[i]}, 200, ""))
		}
		run(fmt.Sprintf("submit-%02d", ep.Number), entries.String())
		clock.Set(cutoff)
		entries.Reset()
		entries.WriteString(rehearsalHurl("PUT", roundPath+"/participants/"+participantIDs[fixture.Players[0].Name], map[string]any{"participant_group_id": uuid.UUID(groups["Ember"].Bytes).String(), "guess_count": 1}, 409, ""))
		entries.WriteString(rehearsalHurl("POST", roundPath+"/close", nil, 200, ""))
		awards := 3
		if ep.Number == 13 {
			awards = 6
		}
		assertion := fmt.Sprintf("[Asserts]\njsonpath \"$.created_count\" == %d\njsonpath \"$.created_entries[*].visibility\" includes \"public\"\n", awards)
		entries.WriteString(rehearsalHurl("POST", roundPath+"/resolve", nil, 200, assertion))
		entries.WriteString(rehearsalHurl("POST", roundPath+"/resolve", nil, 200, assertion))
		for _, c := range fixture.Contestants {
			if c.Episode == int(ep.Number) {
				entries.WriteString(rehearsalHurl("PUT", fmt.Sprintf("%s/outcomes/%d", path, c.Place), map[string]any{"contestant_id": contestantIDs[c.ID]}, 200, ""))
				outcomeCount++
			}
		}
		assertions := fmt.Sprintf("[Asserts]\njsonpath \"$.outcomes\" count == %d\n", outcomeCount)
		for _, c := range fixture.Contestants {
			if c.Episode <= int(ep.Number) {
				assertions += fmt.Sprintf("jsonpath \"$.outcomes[?(@.position == %d)].contestant_id\" includes \"%s\"\n", c.Place, contestantIDs[c.ID])
			}
		}
		entries.WriteString(rehearsalHurl("GET", path+"/outcomes", nil, 200, assertions))
		assertions = "[Asserts]\njsonpath \"$.leaderboard\" count == 6\n"
		for _, e := range ep.Expected {
			for _, field := range []struct {
				name  string
				value int
			}{{"draft_points", e.Draft}, {"bonus_points", e.Bonus}, {"total_points", e.Total}} {
				assertions += fmt.Sprintf("jsonpath \"$.leaderboard[?(@.participant_name == '%s')].%s\" includes %d\n", e.Player, field.name, field.value)
			}
		}
		entries.WriteString(rehearsalHurl("GET", path+"/leaderboard", nil, 200, assertions))
		raw = run(fmt.Sprintf("score-%02d", ep.Number), entries.String())
		if err := json.Unmarshal(raw, &latest); err != nil {
			t.Fatal(err)
		}
		ledger, err := q.ListVisibleBonusPointLedgerEntriesByOccurrence(ctx, wordlePGUUID(uuid.MustParse(round.Round.ID)))
		if err != nil || len(ledger) != awards {
			t.Fatalf("episode %d ledger rows=%d expected=%d err=%v", ep.Number, len(ledger), awards, err)
		}
		for _, entry := range ledger {
			if entry.Points != 1 || entry.Visibility != "public" {
				t.Fatalf("unexpected award: %+v", entry)
			}
		}
		text := fmt.Sprintf("[TEST S43 E%02d — scores] Historical outcomes applied; synthetic Wordle guesses Ember %v / Tide %v. Public points only.\n", ep.Number, ep.Guesses[:3], ep.Guesses[3:])
		for rank, row := range latest.Leaderboard {
			text += fmt.Sprintf("%d. %s: %d (draft %d + Wordle %d)\n", rank+1, row.ParticipantName, row.TotalPoints, row.DraftPoints, row.BonusPoints)
		}
		post(wallStart.Add(time.Duration(index)*6*time.Second+3*time.Second), text)
		t.Logf("Episode %02d: %d published outcomes; %d public Wordle awards; exact scores and retry passed", ep.Number, outcomeCount, awards)
	}
	var invalid int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM bonus_point_ledger_entries WHERE visibility <> 'public' OR entry_kind <> 'award' OR points <> 1`).Scan(&invalid); err != nil || invalid != 0 {
		t.Fatalf("non-public/non-Wordle ledger rows=%d err=%v", invalid, err)
	}
	activities, err := q.ListInstanceActivitiesByInstance(ctx, instanceID)
	if err != nil || len(activities) != 1 || activities[0].ID != activityID {
		t.Fatalf("unexpected activities: %v err=%v", activities, err)
	}
	post(wallStart.Add(78*time.Second), "[TEST S43 COMPLETE] All 13 episodes and 18 placements replayed. Six fake drafts, weekly synthetic Wordle, 42 public +1 awards, zero secret-point activity. Every weekly score assertion and resolution retry passed. This was an accelerated disposable rehearsal, not a production scheduler.")
}

func rehearsalHurl(method, path string, body any, status int, checks string) string {
	var encoded string
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		encoded = "\n" + string(raw) + "\n"
	}
	return fmt.Sprintf("%s {{base_url}}%s\nAuthorization: Bearer {{service_token}}\nX-Discord-User-ID: rehearsal-admin\nContent-Type: application/json\n%s\nHTTP %d\n%s\n", method, path, encoded, status, checks)
}

func rehearsalDiscord(t *testing.T) func(time.Time, string) {
	t.Helper()
	if os.Getenv("CASTAWAY_REHEARSAL_LIVE") != "1" {
		return func(time.Time, string) {}
	}
	token := os.Getenv("CASTAWAY_DISCORD_BOT_TOKEN")
	reportPath := os.Getenv("CASTAWAY_REHEARSAL_REPORT")
	if token == "" || reportPath == "" {
		t.Fatal("live rehearsal requires bot token and CASTAWAY_REHEARSAL_REPORT")
	}
	const channel = "1078197143501819918"
	client := &http.Client{Timeout: 15 * time.Second}
	call := func(method, path string, body any) []byte {
		t.Helper()
		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(data)
		}
		request, err := http.NewRequest(method, "https://discord.com/api/v10"+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bot "+token)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "Castaway-Rehearsal/1.0")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("Discord transport failed; no automatic retry")
		}
		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Error("close Discord response")
			}
		}()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			t.Fatalf("Discord %s failed: HTTP %d; no automatic retry", method, response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 65536))
		if err != nil {
			t.Fatal("read Discord response failed")
		}
		return data
	}
	var channelInfo struct {
		GuildID string `json:"guild_id"`
	}
	if err := json.Unmarshal(call("GET", "/channels/"+channel, nil), &channelInfo); err != nil {
		t.Fatal(err)
	}
	if channelInfo.GuildID != "1078197143501819915" {
		t.Fatal("refusing Discord writes outside BrainLand")
	}
	report, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := report.Close(); err != nil {
			t.Error(err)
		}
	})
	sent := 0
	return func(planned time.Time, content string) {
		t.Helper()
		if sent >= 27 {
			t.Fatal("Discord message budget exhausted")
		}
		if delay := time.Until(planned); delay > 0 {
			time.Sleep(delay)
		}
		if time.Since(planned) > 2*time.Second {
			t.Fatal("rehearsal deadline missed by >2 seconds; stopping")
		}
		var message struct{ ID, Timestamp string }
		data := call("POST", "/channels/"+channel+"/messages", map[string]any{"content": content, "allowed_mentions": map[string]any{"parse": []string{}}})
		if err := json.Unmarshal(data, &message); err != nil || message.ID == "" {
			t.Fatal("Discord response missing message identity; no retry")
		}
		delivered, err := time.Parse(time.RFC3339Nano, message.Timestamp)
		if err != nil {
			t.Fatal(err)
		}
		sent++
		if err := json.NewEncoder(report).Encode(map[string]any{"sequence": sent, "channel_id": channel, "message_id": message.ID, "planned_at": planned.UTC(), "discord_at": delivered, "lateness_ms": delivered.Sub(planned).Milliseconds()}); err != nil {
			t.Fatal(err)
		}
		if err := report.Sync(); err != nil {
			t.Fatal(err)
		}
		t.Logf("Discord delivery %02d: message=%s lateness=%dms", sent, message.ID, delivered.Sub(planned).Milliseconds())
	}
}
