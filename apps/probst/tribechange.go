package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// tribeEmojis maps the tribe colors the tribes API reports to the emoji the post shows.
var tribeEmojis = map[string]string{"red": "🔴", "orange": "🟠", "yellow": "🟡", "green": "🟢", "blue": "🔵",
	"purple": "🟣", "pink": "🩷", "brown": "🟤", "black": "⚫", "white": "⚪"}

// colorTribe is a tribe as the tribes API returns it, with its color.
type colorTribe struct {
	tribe
	Color string `json:"color"`
}

// tribeChallenge is a recorded tribe challenge: what it awarded and the winning tribes' IDs.
type tribeChallenge struct {
	Kind        string    `json:"occurrence_type"`
	EffectiveAt time.Time `json:"effective_at"`
	Metadata    struct {
		Winners []string `json:"winning_tribe_ids"`
	} `json:"metadata"`
}

// tribeChangeData fills messages/tribe-change.md. Kind is swap, merge or assignment.
type tribeChangeData struct {
	Kind   string
	Tribes []swapTribe
	Awards []tribeAward
}

type swapTribe struct {
	Name, Emoji string
	Members     []string // <@discord id> mentions, or the name when the player has no Discord link
}

// tribeAward is one winning tribe's retroactive points: +2 per immunity, +1 per reward, each member.
type tribeAward struct {
	Name, Emoji      string
	Immunity, Reward bool
	Points           int
}

// tribeChangeFor builds the post from the tribes after the change and the episode's tribe challenges.
func tribeChangeFor(kind string, tribes []colorTribe, discordIDs map[string]string, challenges []tribeChallenge) (tribeChangeData, error) {
	d := tribeChangeData{Kind: kind}
	type won struct{ immunity, reward int }
	awards := map[string]*won{}
	for _, c := range challenges {
		for _, id := range c.Metadata.Winners {
			if awards[id] == nil {
				awards[id] = &won{}
			}
			switch c.Kind {
			case "immunity":
				awards[id].immunity++
			case "reward":
				awards[id].reward++
			}
		}
	}
	for _, t := range tribes {
		emoji, ok := tribeEmojis[strings.ToLower(t.Color)]
		if !ok {
			return d, fmt.Errorf("tribe %s has color %q, which has no emoji", t.Name, t.Color)
		}
		var members []string
		for _, m := range t.Members {
			if id := discordIDs[m.ParticipantID]; id != "" {
				members = append(members, "<@"+id+">")
			} else {
				members = append(members, m.Name)
			}
		}
		d.Tribes = append(d.Tribes, swapTribe{Name: t.Name, Emoji: emoji, Members: members})
		if w := awards[t.ID]; w != nil {
			d.Awards = append(d.Awards, tribeAward{Name: t.Name, Emoji: emoji, Immunity: w.immunity > 0, Reward: w.reward > 0,
				Points: 2*w.immunity + w.reward})
			delete(awards, t.ID)
		}
	}
	if len(awards) > 0 {
		return d, fmt.Errorf("a challenge's winning tribe isn't on a tribe at the swap time")
	}
	return d, nil
}

// loadTribeChallenges is the episode's tribe challenges recorded at effective time at.
func loadTribeChallenges(ctx context.Context, call apiCall, instancePath string, at time.Time) ([]tribeChallenge, error) {
	var acts struct {
		Activities []struct {
			ID           string `json:"id"`
			ActivityType string `json:"activity_type"`
		} `json:"activities"`
	}
	if err := call(ctx, "GET", instancePath+"/activities", nil, &acts); err != nil {
		return nil, err
	}
	var out []tribeChallenge
	for _, a := range acts.Activities {
		if a.ActivityType != "tribe_challenge" {
			continue
		}
		var occ struct {
			Occurrences []tribeChallenge `json:"occurrences"`
		}
		if err := call(ctx, "GET", "/activities/"+url.PathEscape(a.ID)+"/occurrences", nil, &occ); err != nil {
			return nil, err
		}
		for _, c := range occ.Occurrences {
			if c.EffectiveAt.Equal(at) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

func addTribeAnnounceCommand(tribes *cobra.Command, call apiCall, guild *string, yes *bool) {
	var seasonPath, kind, name, at string
	var episode int
	cmd := &cobra.Command{
		Use:   "announce --season FILE --episode N",
		Short: "Render the tribe swap post; --yes saves it as an unsent draft announcement (never scheduled or sent)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if seasonPath == "" || episode <= 0 {
				return fmt.Errorf("usage: probst tribes announce --season FILE --episode N")
			}
			if kind != "swap" && kind != "merge" && kind != "assignment" {
				return fmt.Errorf("--kind must be swap, merge, or assignment")
			}
			f, err := loadSeasonFile(seasonPath)
			if err != nil {
				return err
			}
			tmpl, err := seasonTemplates(seasonPath, f)
			if err != nil {
				return err
			}
			_, airs, err := expandSeason(f, time.Now())
			if err != nil {
				return err
			}
			if episode > len(airs) {
				return fmt.Errorf("episode %d is outside the season", episode)
			}
			// Tribes are read at the swap (air + tribeChangeOffset) unless --at says otherwise, as `tribes show --at`.
			swapAt := airs[episode-1].Add(tribeChangeOffset)
			if at != "" {
				if swapAt, err = parseAnnouncementTime(at); err != nil {
					return err
				}
			}
			ctx := c.Context()
			p := "/instances/" + url.PathEscape(f.Instance)
			var res struct {
				Tribes []colorTribe `json:"tribes"`
			}
			if err := call(ctx, "GET", p+"/tribes?at="+url.QueryEscape(swapAt.UTC().Format(time.RFC3339Nano)), nil, &res); err != nil {
				return err
			}
			players, err := loadParticipants(ctx, call, p)
			if err != nil {
				return err
			}
			discordIDs := map[string]string{}
			for _, pl := range players {
				discordIDs[pl.ID] = pl.DiscordUserID
			}
			challenges, err := loadTribeChallenges(ctx, call, p, airs[episode-1].Add(challengeOffset))
			if err != nil {
				return err
			}
			data, err := tribeChangeFor(kind, res.Tribes, discordIDs, challenges)
			if err != nil {
				return err
			}
			text, err := tmpl.render("messages/tribe-change.md", data)
			if err != nil {
				return err
			}
			if name == "" {
				name = fmt.Sprintf("s%d-ep%d-tribe-%s", f.Season, episode, kind)
			}
			out := c.OutOrStdout()
			if !*yes {
				_, err = fmt.Fprintf(out, "Dry run — %s, channel %s (pings the players in it):\n\n%s\n\nRe-run with --yes to save it as a draft. It isn't scheduled or sent.\n", name, f.Channel, text)
				return err
			}
			if err := saveDraftAnnouncement(ctx, call, p, *guild, f.Channel, name, text); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "Saved draft %s. Schedule it with `probst announcement schedule %s --at ...` when approved.\n", name, name)
			return err
		},
	}
	cmd.Flags().StringVar(&seasonPath, "season", "", "Season file (seasons/51.yaml): instance, channel, and templates")
	cmd.Flags().IntVar(&episode, "episode", 0, "Episode whose swap this is")
	cmd.Flags().StringVar(&kind, "kind", "swap", "swap, merge, or assignment (picks the first line)")
	cmd.Flags().StringVar(&name, "name", "", "Announcement key (default s<season>-ep<N>-tribe-<kind>)")
	cmd.Flags().StringVar(&at, "at", "", `Read tribes at "2006-01-02 15:04" (America/New_York) or RFC3339; default the episode's swap time`)
	tribes.AddCommand(cmd)
}
