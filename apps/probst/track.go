package main

import (
	"fmt"
	"io"
	"math/bits"
	"net/url"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// trackBalls is the scored balls in an Inside Track run (the API's limit).
const trackBalls = 12

type trackPlay struct {
	ParticipantID string `json:"participant_id"`
	Name          string `json:"name"`
	Cups          int    `json:"cups"`
	BallsUsed     int    `json:"balls_used"`
	Finished      bool   `json:"finished"`
	Score         int    `json:"score"`
	Place         int    `json:"place"`
	Points        int    `json:"points"`
}

type trackPlays struct {
	GameID   string      `json:"game_id"`
	Open     bool        `json:"open"`
	Resolved bool        `json:"resolved"`
	Plays    []trackPlay `json:"plays"`
}

// trackBody is the PUT body for a correction; it refuses what the API would refuse, so a dry run fails the same way.
func trackBody(mask, balls int, finished bool) (map[string]any, error) {
	if mask < 0 || mask > 7 || balls < 0 || balls > trackBalls || bits.OnesCount(uint(mask)) > balls || (finished && mask != 7 && balls != trackBalls) {
		return nil, fmt.Errorf("cups mask %d, %d balls used, finished=%t is not a valid run", mask, balls, finished)
	}
	return map[string]any{"cups_mask": mask, "balls_used": balls, "finished": finished}, nil
}

// trackScoreOf is the in-game score: cups, plus balls left once all three are in.
func trackScoreOf(cups, balls int) int {
	if cups == 3 {
		return 3 + trackBalls - balls
	}
	return cups
}

func trackGamePath(instance string, episode int) (string, error) {
	if instance == "" {
		return "", fmt.Errorf("--instance is required")
	}
	if episode <= 0 {
		return "", fmt.Errorf("--episode is required")
	}
	return "/instances/" + url.PathEscape(instance) + "/track-games/" + strconv.Itoa(episode), nil
}

func newTrackCommand(call apiCall, instance *string, yes *bool) *cobra.Command {
	track := &cobra.Command{Use: "track", Short: "Inside Track admin"}

	var playsEpisode int
	plays := &cobra.Command{
		Use:   "plays --episode N",
		Short: "Show an episode's Inside Track runs, best place first",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			p, err := trackGamePath(*instance, playsEpisode)
			if err != nil {
				return err
			}
			var res trackPlays
			if err := call(c.Context(), "GET", p+"/plays", nil, &res); err != nil {
				return err
			}
			return printTrackPlays(c.OutOrStdout(), playsEpisode, res)
		},
	}
	plays.Flags().IntVar(&playsEpisode, "episode", 0, "Episode number")
	track.AddCommand(plays)

	var setEpisode, cups, mask, ballsUsed int
	set := &cobra.Command{
		Use:   "set PLAYER --episode N (--cups N | --mask M) [--balls-used K]",
		Short: "Correct a player's Inside Track run; dry run unless --yes",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			flags := c.Flags()
			if !flags.Changed("cups") && !flags.Changed("mask") {
				return fmt.Errorf("--cups or --mask is required")
			}
			if flags.Changed("cups") && (cups < 0 || cups > 3) {
				return fmt.Errorf("--cups must be 0-3")
			}
			p, err := trackGamePath(*instance, setEpisode)
			if err != nil {
				return err
			}
			ctx := c.Context()
			players, err := loadParticipants(ctx, call, "/instances/"+url.PathEscape(*instance))
			if err != nil {
				return err
			}
			player, err := findParticipant(players, a[0])
			if err != nil {
				return err
			}
			var res trackPlays
			if err := call(ctx, "GET", p+"/plays", nil, &res); err != nil {
				return err
			}
			var before trackPlay // no row yet: an empty run
			for _, play := range res.Plays {
				if play.ParticipantID == player.ID {
					before = play
				}
			}
			m := 1<<cups - 1 // the lowest N cups
			if flags.Changed("mask") {
				m = mask
			}
			balls := before.BallsUsed
			if flags.Changed("balls-used") {
				balls = ballsUsed
			}
			body, err := trackBody(m, balls, before.Finished)
			if err != nil {
				return err
			}
			out := c.OutOrStdout()
			nowCups := bits.OnesCount(uint(m))
			if !*yes {
				_, err := fmt.Fprintf(out, "Dry run — %s, episode %d: cups %d → %d, balls used %d → %d, score %d → %d. Re-run with --yes to save.\n",
					player.Name, setEpisode, before.Cups, nowCups, before.BallsUsed, balls, before.Score, trackScoreOf(nowCups, balls))
				return err
			}
			var saved struct {
				Score int `json:"score"`
			}
			if err := call(ctx, "PUT", p+"/plays/"+url.PathEscape(player.ID), body, &saved); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Saved %s, episode %d: cups %d, balls used %d, score %d.\n", player.Name, setEpisode, nowCups, balls, saved.Score)
			return err
		},
	}
	set.Flags().IntVar(&setEpisode, "episode", 0, "Episode number")
	set.Flags().IntVar(&cups, "cups", 0, "Cups filled (0-3); sets the lowest N cups unless --mask is given")
	set.Flags().IntVar(&mask, "mask", 0, "Cups as a bit mask (red 1, yellow 2, blue 4), 0-7")
	set.Flags().IntVar(&ballsUsed, "balls-used", 0, "Balls used (0-12); default keeps the current count")
	track.AddCommand(set)
	return track
}

func printTrackPlays(out io.Writer, episode int, res trackPlays) error {
	state := "closed, awaiting resolve"
	switch {
	case res.Resolved:
		state = "resolved"
	case res.Open:
		state = "open"
	}
	if _, err := fmt.Fprintf(out, "Episode %d Inside Track (%s)\n", episode, state); err != nil {
		return err
	}
	if len(res.Plays) == 0 {
		_, err := fmt.Fprintln(out, "No runs yet.")
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLACE\tNAME\tSCORE\tCUPS\tBALLS LEFT\tFINISHED\tPOINTS")
	for _, p := range res.Plays {
		finished := "no"
		if p.Finished {
			finished = "yes"
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%d\t%s\t%d\n", p.Place, p.Name, p.Score, p.Cups, trackBalls-p.BallsUsed, finished, p.Points)
	}
	return w.Flush()
}
