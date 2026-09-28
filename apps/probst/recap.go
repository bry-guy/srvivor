package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type scoreRow struct {
	ID        string `json:"participant_id"`
	Name      string `json:"participant_name"`
	DiscordID string `json:"participant_discord_user_id"`
	Tribe     string `json:"current_tribe_name"`
	Draft     int    `json:"draft_points"`
	Bonus     int    `json:"bonus_points"`
	Total     int    `json:"total_points"`
	HasDraft  bool   `json:"has_draft"`
}

// scoreSnapshot is what a recap saves locally so next week's recap can diff against it.
type scoreSnapshot struct {
	Episode    int            `json:"episode"`
	Totals     map[string]int `json:"totals"`     // participant ID → total
	Eliminated []string       `json:"eliminated"` // contestant IDs with an outcome
}

type outcome struct {
	ContestantID string `json:"contestant_id"`
	Name         string `json:"contestant_name"`
	Position     int    `json:"position"`
}

// Flair for the elimination line, picked by episode; %s is the spoiler-wrapped names.
var bootFlair = []string{
	"The tribe has spoken. %s, grab your torch. 🔥",
	"%s got got. We hardly knew ye. 🫡",
	"Pour one out for %s, who is officially off the island. 🏝️",
	"%s has been voted out. Jeff snuffs, the board shuffles. 🔥",
	"Another torch, another snuff: %s is headed to Ponderosa. 🧺",
}

// shortName prefers a quoted nickname, then the first name: `Danny "Kilby" Kilby` → Kilby.
func shortName(full string) string {
	if _, rest, ok := strings.Cut(full, `"`); ok {
		if nick, _, ok := strings.Cut(rest, `"`); ok && nick != "" {
			return nick
		}
	}
	return strings.Fields(full + " ?")[0]
}

func mention(r scoreRow) string {
	if r.DiscordID != "" {
		return "<@" + r.DiscordID + ">"
	}
	return r.Name
}

// renderRecap builds the weekly scores post; prev is nil for the first recap (gains are then full totals).
func renderRecap(season, episode int, rows []scoreRow, outcomes []outcome, prev *scoreSnapshot) (string, scoreSnapshot) {
	snap := scoreSnapshot{Episode: episode, Totals: map[string]int{}}
	seen := map[string]bool{}
	if prev != nil {
		for _, id := range prev.Eliminated {
			seen[id] = true
		}
	}
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].Position > outcomes[j].Position })
	var booted []string
	for _, o := range outcomes {
		if o.ContestantID == "" {
			continue
		}
		snap.Eliminated = append(snap.Eliminated, o.ContestantID)
		if !seen[o.ContestantID] {
			// Padding hides the name's length behind the spoiler.
			booted = append(booted, fmt.Sprintf("||%-14s||", "  "+shortName(o.Name)))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Castaway Season %d: Week %d Scores\n\n", season, episode)
	if len(booted) > 0 {
		names := strings.Join(booted, ", ")
		if n := len(booted); n > 1 {
			names = strings.Join(booted[:n-1], ", ") + " and " + booted[n-1]
		}
		fmt.Fprintf(&b, bootFlair[(episode-1+len(bootFlair))%len(bootFlair)]+"\n\n", names)
	}

	gains := map[string]int{}
	var drafted []scoreRow
	for _, r := range rows {
		if !r.HasDraft {
			continue
		}
		drafted = append(drafted, r)
		snap.Totals[r.ID] = r.Total
		gains[r.ID] = r.Total
		if prev != nil {
			gains[r.ID] -= prev.Totals[r.ID]
		}
	}
	if len(drafted) > 1 {
		pick := func(better func(a, b int) bool) (string, int) {
			best := gains[drafted[0].ID]
			for _, r := range drafted {
				if better(gains[r.ID], best) {
					best = gains[r.ID]
				}
			}
			var who []string
			for _, r := range drafted {
				if gains[r.ID] == best {
					who = append(who, mention(r))
				}
			}
			if len(who) > 3 {
				return fmt.Sprintf("%d players tied", len(who)), best
			}
			return strings.Join(who, ", "), best
		}
		top, topGain := pick(func(a, b int) bool { return a > b })
		low, lowGain := pick(func(a, b int) bool { return a < b })
		if topGain != lowGain {
			fmt.Fprintf(&b, "📈 Biggest gainer: %s (+%d)\n📉 Biggest loser: %s (+%d)\n\n", top, topGain, low, lowGain)
		}
	}

	sort.SliceStable(drafted, func(i, j int) bool { return drafted[i].Total > drafted[j].Total })
	b.WriteString("**Leaderboard**\n")
	rank := 0
	for i, r := range drafted {
		if i == 0 || r.Total != drafted[i-1].Total {
			rank = i + 1
		}
		tribe := ""
		if r.Tribe != "" {
			tribe = " (" + r.Tribe + ")"
		}
		fmt.Fprintf(&b, "%d. %s%s: %d (%d+%d)\n", rank, mention(r), tribe, r.Total, r.Draft, r.Bonus)
	}
	b.WriteString("\n> Total (Draft+Bonus)\n")
	return b.String(), snap
}

func probstDataDir() (string, error) {
	if dir := os.Getenv("PROBST_DATA_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "probst"), err
}

// latestSnapshot returns the newest saved snapshot from before episode, or nil.
func latestSnapshot(dir string, episode int) (*scoreSnapshot, error) {
	for n := episode - 1; n >= 0; n-- {
		data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("episode-%02d.json", n)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var s scoreSnapshot
		return &s, json.Unmarshal(data, &s)
	}
	return nil, nil
}

func addRecapCommand(root *cobra.Command, call apiCall, instancePath func() (string, error), instanceID *string) {
	var episode int
	cmd := &cobra.Command{
		Use:   "recap",
		Short: "Print the weekly scores post (boots, biggest gainer/loser, leaderboard) and save a local snapshot for next week's diff",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if episode <= 0 {
				return fmt.Errorf("--episode is required (the last episode the scores cover)")
			}
			p, err := instancePath()
			if err != nil {
				return err
			}
			ctx := c.Context()
			var inst struct {
				Instance struct {
					Season int `json:"season"`
				} `json:"instance"`
			}
			if err := call(ctx, "GET", p, nil, &inst); err != nil {
				return err
			}
			var board struct {
				Leaderboard []scoreRow `json:"leaderboard"`
			}
			if err := call(ctx, "GET", p+"/leaderboard", nil, &board); err != nil {
				return err
			}
			var outs struct {
				Outcomes []outcome `json:"outcomes"`
			}
			if err := call(ctx, "GET", p+"/outcomes", nil, &outs); err != nil {
				return err
			}
			base, err := probstDataDir()
			if err != nil {
				return err
			}
			dir := filepath.Join(base, "scores", *instanceID)
			prev, err := latestSnapshot(dir, episode)
			if err != nil {
				return err
			}
			text, snap := renderRecap(inst.Instance.Season, episode, board.Leaderboard, outs.Outcomes, prev)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			encoded, err := json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("episode-%02d.json", episode)), encoded, 0o644); err != nil {
				return err
			}
			_, err = fmt.Fprint(c.OutOrStdout(), text)
			return err
		},
	}
	cmd.Flags().IntVar(&episode, "episode", 0, "Last episode these scores cover (Week N)")
	root.AddCommand(cmd)
}
