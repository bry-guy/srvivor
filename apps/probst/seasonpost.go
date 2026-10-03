package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

// Weekly scores copy: the same shape every week, with lines picked by week number so no line repeats within a
// 13-week season. %s is the player(s); %d the points (gainer/slider) or total (leader). No pronouns needed.
var (
	introLines = []string{
		"Come on in, castaways. Let's see where everybody stands.",
		"Bring it in! Another week, another shake-up.",
		"Castaways, gather 'round. The numbers are in.",
		"Welcome back. Somebody's torch is burning bright, and somebody's is flickering.",
		"Come on in, guys! Time to see who's playing and who's just camping.",
		"The tribe has spoken, and so have the scores.",
		"Another vote, another reckoning. Let's get to it.",
		"Castaways, take a seat on your log. Scores are in.",
		"Bring it in. We're deep in the game now.",
		"Come on in! Every point counts from here.",
		"The jury's watching. Here's how you all look.",
		"Final stretch, castaways. This is where it gets real.",
		"This is it. One last look at the board.",
	}
	leaderLines = []string{
		"%s sits on top with %d. That's a target, and a crown.",
		"%s leads the pack at %d. Calm, cool, and dangerous.",
		"Nobody's catching %s yet: %d and climbing.",
		"%s is running this island with %d.",
		"Still the one to beat: %s, %d points.",
		"%s holds the top spot at %d. Somebody make a move.",
		"%s, at %d, is playing chess while the rest of you play checkers.",
		"The crown stays with %s: %d and not letting go.",
		"%s rules the board at %d. Jeff's impressed. Mostly.",
		"Top of the island: %s, %d. Respect the grind.",
		"%s at %d. Somebody find an idol, fast.",
		"%s leads with %d, and the finish line is in sight.",
		"%s finishes on top of the heap with %d.",
	}
	gainerLines = []string{
		"Biggest move of the week: %s, up %d. Big swing!",
		"%s came to play: +%d this week.",
		"Hot streak alert: %s jumped %d.",
		"%s found an advantage somewhere: +%d.",
		"Look out for %s, up %d and coming for the top.",
		"%s made the power move this week: +%d.",
		"%s turned it up: %d points gained.",
		"Nobody climbed faster than %s: +%d.",
		"%s is on a heater: +%d.",
		"%s, +%d. That's how you play the game.",
		"Mover of the week: %s, up %d.",
		"%s surged %d when it mattered.",
		"One last leap: %s, +%d.",
	}
	sliderLines = []string{
		"%s slipped %d. It happens to the best of us. Allegedly.",
		"%s lost %d this week. Maybe check the rice ration.",
		"Rough week for %s: down %d. The fire went out, but you can make another.",
		"%s dropped %d. Somebody get them a reward challenge.",
		"%s slid %d. Not a blindside, just a bruise.",
		"%s, down %d. Still in the game, still dangerous. Probably.",
		"%s took a %d-point tumble. Shake it off.",
		"%s lost %d. Jeff says: dig deep.",
		"%s went %d in the wrong direction. Plenty of game left.",
		"%s fell back %d. Every legend has a low point.",
		"%s, down %d. Time for a comeback arc.",
		"%s slid %d. The edge of extinction is lonely, come back.",
		"%s dropped %d at the very end. Brutal. Beautiful.",
	}
	gameLines = map[string]string{ // %s is the open time; games open with the episode
		"press_the_button": "🔴 This week's game: **Press the Button**, open %s at <%s/button>.",
		"castawordle":      "🔤 This week's game: **Castawordle**, open %s at <%s/castawordle>.",
	}
	gameNames = map[string]string{"press_the_button": "Press the Button", "castawordle": "Castawordle"}
)

type scoresPost struct {
	Season, Week                               int
	Intro, Boots, Leader, Gainer, Slider, Last string
	Top                                        []string
	Site, NextGame                             string
}

func weekLine(lines []string, week int) string { return lines[(week-1)%len(lines)] }

func joinNames(names []string) string {
	if len(names) > 3 {
		return fmt.Sprintf("%d players", len(names))
	}
	if n := len(names); n > 1 {
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	}
	return strings.Join(names, "")
}

// buildScoresPost fills the weekly post from the board now and as it stood at the previous post. Only the top
// three and last place are mentioned (and pinged); callouts name players without pinging.
func buildScoresPost(season, week int, now, prev []scoreRow, booted []string, site, nextGame string) (scoresPost, error) {
	var rows []scoreRow
	before := map[string]int{}
	for _, r := range prev {
		before[r.ID] = r.Total
	}
	for _, r := range now {
		if r.HasDraft {
			rows = append(rows, r)
		}
	}
	if len(rows) < 2 {
		return scoresPost{}, fmt.Errorf("need at least two players with drafts")
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Total > rows[j].Total })
	gain := func(r scoreRow) int { return r.Total - before[r.ID] }
	who := func(match func(scoreRow) bool) []string {
		var names []string
		for _, r := range rows {
			if match(r) {
				names = append(names, r.Name)
			}
		}
		return names
	}
	best, worst := gain(rows[0]), gain(rows[0])
	for _, r := range rows {
		best, worst = max(best, gain(r)), min(worst, gain(r))
	}
	p := scoresPost{Season: season, Week: week, Intro: weekLine(introLines, week), Site: site, NextGame: nextGame}
	p.Leader = fmt.Sprintf(weekLine(leaderLines, week), joinNames(who(func(r scoreRow) bool { return r.Total == rows[0].Total })), rows[0].Total)
	p.Gainer = fmt.Sprintf(weekLine(gainerLines, week), joinNames(who(func(r scoreRow) bool { return gain(r) == best })), best)
	sliders := joinNames(who(func(r scoreRow) bool { return gain(r) == worst }))
	if best == worst { // nobody separated from anybody: one line instead of three ties
		p.Gainer, p.Slider = "", ""
		p.Leader += " Nobody else moved an inch this week."
	} else if worst < 0 {
		p.Slider = fmt.Sprintf(weekLine(sliderLines, week), sliders, -worst)
	} else { // nobody lost points: rib the slowest climber instead
		p.Slider = fmt.Sprintf("%s managed just +%d. Slow and steady… mostly slow.", sliders, worst)
	}
	for _, r := range rows[:min(3, len(rows)-1)] {
		p.Top = append(p.Top, fmt.Sprintf("%s: %d", mention(r), r.Total))
	}
	last := rows[len(rows)-1]
	p.Last = fmt.Sprintf("%s: %d", mention(last), last.Total)
	if len(booted) > 0 {
		p.Boots = fmt.Sprintf(bootFlair[(week-1)%len(bootFlair)], joinNames(booted))
	}
	return p, nil
}

func renderScoresPost(tmplPath string, p scoresPost) (string, error) {
	t, err := template.New(filepath.Base(tmplPath)).Option("missingkey=error").ParseFiles(tmplPath)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, p); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()) + "\n", nil
}

// weekPostTime is when week's scores post goes out according to the file.
func weekPostTime(f seasonFile, airs []time.Time, week int) (time.Time, error) {
	if week < 1 || week > len(airs) {
		return time.Time{}, fmt.Errorf("week %d is outside the season", week)
	}
	post := f.Weekly.ScoresPost
	if w := f.Weeks[week]; w.ScoresPost != nil && w.ScoresPost.At != nil {
		post.At = w.ScoresPost.At
	}
	at, _ := post.At.(string)
	return planTime(at, airs[week-1])
}

// weekGame is a week's game type, or "" when the file has none, TBD, or a placeholder.
func weekGame(f seasonFile, week int) string {
	m, ok := f.Weeks[week].Game.(map[string]any)
	if !ok || m["tbd"] == true {
		return ""
	}
	t, _ := m["type"].(string)
	return t
}

func scoresKey(season, week int) string { return fmt.Sprintf("s%d-week-%d-scores", season, week) }

func seasonFilePaths(file string) (string, error) {
	tmpl := strings.TrimSuffix(file, filepath.Ext(file)) + "-scores.md"
	if _, err := os.Stat(tmpl); err != nil {
		return "", fmt.Errorf("scores template %s: %w", tmpl, err)
	}
	return tmpl, nil
}

func addSeasonPostCommand(season *cobra.Command, call apiCall, guild *string, yes *bool) {
	var week int
	cmd := &cobra.Command{
		Use:   "post FILE --week N",
		Short: "Render a week's scores post from the template; --yes saves and schedules it at the file's time",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			f, err := loadSeasonFile(a[0])
			if err != nil {
				return err
			}
			tmpl, err := seasonFilePaths(a[0])
			if err != nil {
				return err
			}
			_, airs, err := expandSeason(f, time.Now())
			if err != nil {
				return err
			}
			at, err := weekPostTime(f, airs, week)
			if err != nil {
				return err
			}
			text, err := composeScoresPost(c.Context(), call, f, airs, week, tmpl)
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			key := scoresKey(f.Season, week)
			if !*yes {
				_, err = fmt.Fprintf(out, "Dry run — %s, sends %s ET (pings the top 3 and last place):\n\n%s\nRe-run with --yes to save and schedule it.\n", key, at.Format("Mon Jan 02 3:04pm"), text)
				return err
			}
			path := "/instances/" + url.PathEscape(f.Instance)
			existing, err := findAnnouncement(c.Context(), call, path, key)
			switch {
			case err == nil && existing.Status == "sent":
				return fmt.Errorf("%s was already sent", key)
			case err == nil:
				err = call(c.Context(), "PUT", path+"/announcements/"+url.PathEscape(existing.ID)+"/body", map[string]string{"body": text}, nil)
			default:
				err = call(c.Context(), "POST", path+"/announcements", map[string]any{
					"guild_id": *guild, "channel_id": f.Channel, "request_key": key, "body": text, "draft": true, "notify_users": true,
				}, nil)
			}
			if err != nil {
				return err
			}
			found, err := findAnnouncement(c.Context(), call, path, key)
			if err != nil {
				return err
			}
			if err := call(c.Context(), "PUT", path+"/announcements/"+url.PathEscape(found.ID)+"/schedule", map[string]any{"scheduled_at": at.UTC().Format(time.RFC3339)}, nil); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Scheduled %s for %s ET. Re-run any time before then to refresh it with the latest scores.\n", key, at.Format("Mon Jan 02 3:04pm"))
			return err
		},
	}
	cmd.Flags().IntVar(&week, "week", 0, "Week N (scores through Episode N)")
	season.AddCommand(cmd)
}

func composeScoresPost(ctx context.Context, call apiCall, f seasonFile, airs []time.Time, week int, tmpl string) (string, error) {
	path := "/instances/" + url.PathEscape(f.Instance)
	var now, prev struct {
		Leaderboard []scoreRow `json:"leaderboard"`
	}
	if err := call(ctx, "GET", path+"/leaderboard", nil, &now); err != nil {
		return "", err
	}
	since := time.Time{}
	if week > 1 {
		t, err := weekPostTime(f, airs, week-1)
		if err != nil {
			return "", err
		}
		since = t
		if err := call(ctx, "GET", path+"/leaderboard?at="+url.QueryEscape(since.UTC().Format(time.RFC3339)), nil, &prev); err != nil {
			return "", err
		}
	}
	var outs struct {
		Outcomes []struct {
			Name      string    `json:"contestant_name"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"outcomes"`
	}
	if err := call(ctx, "GET", path+"/outcomes", nil, &outs); err != nil {
		return "", err
	}
	var booted []string
	for _, o := range outs.Outcomes {
		if o.Name != "" && o.UpdatedAt.After(since) {
			booted = append(booted, fmt.Sprintf("||%-14s||", "  "+shortName(o.Name)))
		}
	}
	next := ""
	if g := weekGame(f, week+1); g != "" && week < len(airs) {
		if line, ok := gameLines[g]; ok {
			opens, err := planTime(f.Weekly.Game.Opens, airs[week])
			if err != nil {
				return "", err
			}
			when := "tonight at " + strings.TrimSuffix(opens.Format("3:04pm"), ":00pm") + "pm ET"
			next = fmt.Sprintf(line, when, "https://castaway.bry-guy.net")
		}
	}
	p, err := buildScoresPost(f.Season, week, now.Leaderboard, prev.Leaderboard, booted, "https://castaway.bry-guy.net", next)
	if err != nil {
		return "", err
	}
	return renderScoresPost(tmpl, p)
}

func addSeasonApplyCommand(season *cobra.Command, call apiCall, yes *bool) {
	season.AddCommand(&cobra.Command{
		Use:   "apply FILE",
		Short: "Create or reschedule the file's games (dry run unless --yes); placeholders are skipped",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			f, err := loadSeasonFile(a[0])
			if err != nil {
				return err
			}
			_, airs, err := expandSeason(f, time.Now())
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			for week := 1; week <= len(airs); week++ {
				game := weekGame(f, week)
				if game == "" {
					continue
				}
				opens, err1 := planTime(f.Weekly.Game.Opens, airs[week-1])
				closes, err2 := planTime(f.Weekly.Game.Closes, airs[week-1])
				if err1 != nil || err2 != nil {
					return fmt.Errorf("weekly.game: bad opens/closes")
				}
				line := fmt.Sprintf("Week %d %s: %s → %s ET", week, gameNames[game], opens.Format("Mon Jan 02 3:04pm"), closes.Format("Mon Jan 02 3:04pm"))
				switch game {
				case "press_the_button":
					if !*yes {
						fmt.Fprintln(out, "would apply "+line)
						continue
					}
					body := map[string]any{"episode_number": week, "opens_at": opens.UTC().Format(time.RFC3339), "cutoff_at": closes.UTC().Format(time.RFC3339)}
					if err := call(c.Context(), "POST", "/instances/"+url.PathEscape(f.Instance)+"/button-games", body, nil); err != nil {
						return fmt.Errorf("%s: %w", line, err)
					}
					fmt.Fprintln(out, "applied "+line)
				case "castawordle":
					fmt.Fprintln(out, "skipped "+line+" (set up with `probst castawordle week`, which needs the answer)")
				default:
					return fmt.Errorf("week %d: unknown game type %q", week, game)
				}
			}
			if !*yes {
				fmt.Fprintln(out, "Dry run — re-run with --yes to apply.")
			}
			return nil
		},
	})
}
