package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

type apiCall func(ctx context.Context, method, path string, body, out any) error

type participant struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DiscordUserID string `json:"discord_user_id"`
}

type discordMessage struct {
	ID              string     `json:"id"`
	Content         string     `json:"content"`
	Timestamp       time.Time  `json:"timestamp"`
	EditedTimestamp *time.Time `json:"edited_timestamp"`
	Author          struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
	} `json:"author"`
}

type importRow struct {
	Player string
	Author string
	Status string
	Detail string
	draft  parsedDraft
	target *participant
}

func loadRoster(ctx context.Context, call apiCall, instancePath string) ([]*contestant, error) {
	var res struct {
		Contestants []contestant `json:"contestants"`
	}
	if err := call(ctx, "GET", instancePath+"/contestants", nil, &res); err != nil {
		return nil, err
	}
	if len(res.Contestants) == 0 {
		return nil, fmt.Errorf("instance has no contestants")
	}
	return newRoster(res.Contestants), nil
}

func loadParticipants(ctx context.Context, call apiCall, instancePath string) ([]participant, error) {
	var res struct {
		Participants []participant `json:"participants"`
	}
	return res.Participants, call(ctx, "GET", instancePath+"/participants", nil, &res)
}

// findParticipant accepts a participant ID or an exact (case-insensitive) name.
func findParticipant(all []participant, ref string) (*participant, error) {
	var hits []*participant
	for i := range all {
		if all[i].ID == ref || strings.EqualFold(all[i].Name, ref) {
			hits = append(hits, &all[i])
		}
	}
	if len(hits) != 1 {
		return nil, fmt.Errorf("participant %q matched %d players; use the participant ID", ref, len(hits))
	}
	return hits[0], nil
}

var threadURL = regexp.MustCompile(`discord(?:app)?\.com/channels/\d+/(\d+)`)

func threadChannelID(ref string) (string, error) {
	if m := threadURL.FindStringSubmatch(ref); m != nil {
		return m[1], nil
	}
	if regexp.MustCompile(`^\d+$`).MatchString(ref) {
		return ref, nil
	}
	return "", fmt.Errorf("expected a Discord thread URL or channel ID, got %q", ref)
}

// fetchThread reads every message in a channel or thread, oldest first.
func fetchThread(ctx context.Context, channelID, token string) ([]discordMessage, error) {
	if token == "" {
		return nil, fmt.Errorf("set CASTAWAY_DISCORD_BOT_TOKEN through your credential provider to read Discord")
	}
	base := os.Getenv("PROBST_DISCORD_API_URL")
	if base == "" {
		base = "https://discord.com/api/v10"
	}
	client := &http.Client{Timeout: 15 * time.Second}
	var all []discordMessage
	before := ""
	for {
		endpoint := base + "/channels/" + url.PathEscape(channelID) + "/messages?limit=100"
		if before != "" {
			endpoint += "&before=" + before
		}
		var page []discordMessage
		for attempt := 0; ; attempt++ {
			req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bot "+token)
			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("Discord request failed: %w", err)
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			_ = resp.Body.Close()
			if err != nil {
				return nil, err
			}
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				var limit struct {
					RetryAfter float64 `json:"retry_after"`
				}
				_ = json.Unmarshal(body, &limit)
				time.Sleep(time.Duration((limit.RetryAfter + 0.1) * float64(time.Second)))
				continue
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("Discord HTTP %d reading channel %s: %s", resp.StatusCode, channelID, strings.TrimSpace(string(body)))
			}
			if err := json.Unmarshal(body, &page); err != nil {
				return nil, err
			}
			break
		}
		all = append(all, page...)
		if len(page) < 100 {
			break
		}
		before = page[len(page)-1].ID
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp.Before(all[j].Timestamp) })
	return all, nil
}

// reviewThread picks each author's latest draft before the cutoff and maps authors to players.
func reviewThread(messages []discordMessage, roster []*contestant, players []participant, cutoff *time.Time) []importRow {
	type latest struct {
		msg   discordMessage
		draft parsedDraft
		count int
	}
	byAuthor := map[string]*latest{}
	var authorOrder []string
	late := map[string]int{}
	for _, m := range messages {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		d := parseDraft(m.Content, roster)
		if !isDraftCandidate(d, roster) {
			continue
		}
		if cutoff != nil && m.Timestamp.After(*cutoff) {
			late[m.Author.ID]++
			continue
		}
		l := byAuthor[m.Author.ID]
		if l == nil {
			l = &latest{}
			byAuthor[m.Author.ID] = l
			authorOrder = append(authorOrder, m.Author.ID)
		}
		l.msg, l.draft = m, d
		l.count++
	}
	playerByDiscord := map[string]*participant{}
	for i := range players {
		if players[i].DiscordUserID != "" {
			playerByDiscord[players[i].DiscordUserID] = &players[i]
		}
	}
	var rows []importRow
	drafted := map[string]bool{}
	for _, authorID := range authorOrder {
		l := byAuthor[authorID]
		author := l.msg.Author.GlobalName
		if author == "" {
			author = l.msg.Author.Username
		}
		author = fmt.Sprintf("%s (%s)", author, authorID)
		row := importRow{Author: author, draft: l.draft, target: playerByDiscord[authorID]}
		var notes []string
		if l.count > 1 {
			notes = append(notes, fmt.Sprintf("latest of %d drafts", l.count))
		}
		if late[authorID] > 0 {
			notes = append(notes, fmt.Sprintf("%d later draft(s) after cutoff ignored", late[authorID]))
		}
		problems := append([]string(nil), l.draft.Problems...)
		if cutoff != nil && l.msg.EditedTimestamp != nil && l.msg.EditedTimestamp.After(*cutoff) {
			problems = append(problems, "edited after the cutoff")
		}
		switch {
		case row.target == nil:
			row.Status = "UNLINKED"
			notes = append(notes, "link this Discord user to a player first")
		case len(problems) > 0:
			row.Player, row.Status = row.target.Name, "NEEDS REVIEW"
			drafted[row.target.ID] = true
		default:
			row.Player, row.Status = row.target.Name, "READY"
			drafted[row.target.ID] = true
		}
		row.draft.Problems = problems
		row.Detail = strings.Join(append(problems, notes...), "; ")
		rows = append(rows, row)
	}
	for i := range players {
		if !drafted[players[i].ID] {
			detail := "no draft found in thread"
			if players[i].DiscordUserID == "" {
				detail = "player has no Discord link"
			}
			if late[players[i].DiscordUserID] > 0 {
				detail = "only drafted after the cutoff"
			}
			rows = append(rows, importRow{Player: players[i].Name, Status: "NO DRAFT", Detail: detail})
		}
	}
	return rows
}

// submitReady compares READY drafts with the API and writes the ones that changed when confirmed.
func submitReady(ctx context.Context, call apiCall, instancePath string, rows []importRow, yes bool) error {
	for i := range rows {
		row := &rows[i]
		if row.Status != "READY" {
			continue
		}
		ids := make([]string, len(row.draft.Order))
		for j, c := range row.draft.Order {
			ids[j] = c.ID
		}
		var current struct {
			Picks []struct {
				Position     int    `json:"position"`
				ContestantID string `json:"contestant_id"`
			} `json:"picks"`
		}
		path := instancePath + "/drafts/" + url.PathEscape(row.target.ID)
		if err := call(ctx, "GET", path, nil, &current); err != nil {
			return fmt.Errorf("read %s's current draft: %w", row.target.Name, err)
		}
		sort.Slice(current.Picks, func(a, b int) bool { return current.Picks[a].Position < current.Picks[b].Position })
		same := len(current.Picks) == len(ids)
		for j := 0; same && j < len(ids); j++ {
			same = current.Picks[j].ContestantID == ids[j]
		}
		switch {
		case same:
			row.Status = "UNCHANGED"
		case !yes:
			row.Status = "READY"
			if len(current.Picks) > 0 {
				row.Detail = strings.TrimPrefix(row.Detail+"; replaces existing draft", "; ")
			}
		default:
			if err := call(ctx, "PUT", path, map[string]any{"contestant_ids": ids}, nil); err != nil {
				return fmt.Errorf("submit %s's draft: %w", row.target.Name, err)
			}
			row.Status = "SUBMITTED"
		}
	}
	return nil
}

func printImport(out io.Writer, rows []importRow, verbose bool) error {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "PLAYER\tDISCORD AUTHOR\tSTATUS\tDETAIL")
	for _, r := range rows {
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", dash(r.Player), dash(r.Author), r.Status, r.Detail)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if !verbose {
		return nil
	}
	for _, r := range rows {
		if len(r.draft.Picks) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(out, "\n%s %s\n", dash(r.Player), r.Author)
		table = tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		for i, p := range r.draft.Picks {
			match := "—"
			if p.Contestant != nil {
				match = p.Contestant.Name
			}
			_, _ = fmt.Fprintf(table, "  %d\t%s\t→ %s\t%s %s\n", i+1, strings.TrimSpace(p.Raw), match, p.Method, p.Note)
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
