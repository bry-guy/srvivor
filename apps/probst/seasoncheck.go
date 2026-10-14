package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

// scheduledGame is one week's game as the server has it (no answers).
type scheduledGame struct {
	Type    string `json:"type"`
	Episode int    `json:"episode_number"`
}

// gameReadiness says whether week's game is set: "" if ready (or deliberately none), else what's wrong.
func gameReadiness(t templateSet, f seasonFile, week int, live []scheduledGame) (string, error) {
	if f.Weeks[week].Game == "none" {
		return "", nil
	}
	want := weekGame(f, week)
	if want == "" {
		return t.render("messages/game-unset.md", map[string]any{"Week": week, "Season": f.Season})
	}
	for _, g := range live {
		if g.Episode == week && g.Type == want {
			return "", nil
		}
	}
	return t.render("messages/game-not-created.md", map[string]any{"Week": week, "Game": gameNames[want], "Season": f.Season})
}

// checkNextGame raises a one-time admin alert when the game for the week after the one that just opened
// isn't ready. It acts only once week's game has opened (and before the next opens). Returns what it did.
func checkNextGame(ctx context.Context, call apiCall, t templateSet, f seasonFile, airs []time.Time, now time.Time) (string, error) {
	week := 0
	for i, air := range airs {
		if opens, err := planTime(f.Weekly.Game.Opens, air); err == nil && !now.Before(opens) {
			week = i + 1
		}
	}
	next := week + 1
	if week == 0 || next > len(airs) {
		return "no next week to check", nil
	}
	var live struct {
		Games []scheduledGame `json:"games"`
	}
	path := "/instances/" + url.PathEscape(f.Instance)
	if err := call(ctx, "GET", path+"/scheduled-games", nil, &live); err != nil {
		return "", err
	}
	problem, err := gameReadiness(t, f, next, live.Games)
	if err != nil {
		return "", err
	}
	if problem == "" {
		return fmt.Sprintf("week %d's game is ready", next), nil
	}
	opens, err := planTime(f.Weekly.Game.Opens, airs[next-1])
	if err != nil {
		return "", err
	}
	body, err := t.render("messages/game-not-ready.md", map[string]any{"Problem": problem, "Opens": opens.Format("Mon Jan 2 at 3:04pm")})
	if err != nil {
		return "", err
	}
	var created struct {
		Created bool `json:"created"`
	}
	key := fmt.Sprintf("s%d-w%d-game-not-ready", f.Season, next)
	if err := call(ctx, "PUT", path+"/admin-alerts/"+url.PathEscape(key), map[string]string{"body": body}, &created); err != nil {
		return "", err
	}
	if created.Created {
		return "alerted admins: " + problem, nil
	}
	return "already alerted: " + problem, nil
}

func addSeasonCheckCommand(season *cobra.Command, call apiCall) {
	season.AddCommand(&cobra.Command{
		Use:   "check FILE",
		Short: "Scheduled check: once a week's game opens, DM admins (once) if the next week's game isn't ready",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			f, err := loadSeasonFile(a[0])
			if err != nil {
				return err
			}
			now := time.Now()
			_, airs, err := expandSeason(f, now)
			if err != nil {
				return err
			}
			tmpl, err := seasonTemplates(a[0], f)
			if err != nil {
				return err
			}
			did, err := checkNextGame(c.Context(), call, tmpl, f, airs, now)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(c.OutOrStdout(), did)
			return err
		},
	})
}
