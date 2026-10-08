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
func gameReadiness(f seasonFile, week int, live []scheduledGame) string {
	if f.Weeks[week].Game == "none" {
		return ""
	}
	want := weekGame(f, week)
	if want == "" {
		return fmt.Sprintf("Week %d has no game picked in seasons/%d.yaml (weeks.%d.game is TBD).", week, f.Season, week)
	}
	for _, g := range live {
		if g.Episode == week && g.Type == want {
			return ""
		}
	}
	return fmt.Sprintf("Week %d's %s isn't set up on the server yet. Run `probst season apply seasons/%d.yaml --yes` (Castawordle: `probst castawordle week`).", week, gameNames[want], f.Season)
}

// checkNextGame raises a one-time admin alert when the game for the week after the one that just opened
// isn't ready. It acts only once week's game has opened (and before the next opens). Returns what it did.
func checkNextGame(ctx context.Context, call apiCall, f seasonFile, airs []time.Time, now time.Time) (string, error) {
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
	problem := gameReadiness(f, next, live.Games)
	if problem == "" {
		return fmt.Sprintf("week %d's game is ready", next), nil
	}
	opens, err := planTime(f.Weekly.Game.Opens, airs[next-1])
	if err != nil {
		return "", err
	}
	body := fmt.Sprintf("⚠️ **Next week's game isn't ready.** %s It opens %s ET.", problem, opens.Format("Mon Jan 2 at 3:04pm"))
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
			did, err := checkNextGame(c.Context(), call, f, airs, now)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(c.OutOrStdout(), did)
			return err
		},
	})
}
