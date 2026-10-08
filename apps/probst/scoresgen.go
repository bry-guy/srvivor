package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// scoresFacts are the numbers the flavor lines must state; buildScoresPost fills them.
type scoresFacts struct {
	LeaderNames []string `json:"leader_names"`
	LeaderTotal int      `json:"leader_total"`
	GainerNames []string `json:"gainer_names"`
	Gain        int      `json:"gain"`
	SliderNames []string `json:"slider_names"`
	Slide       int      `json:"slide"` // points lost; negative means the slider still gained
	NobodyMoved bool     `json:"nobody_moved"`
}

type scoresFlavor struct {
	Intro  string `json:"intro"`
	Leader string `json:"leader"`
	Gainer string `json:"gainer"`
	Slider string `json:"slider"`
}

// flavorWriter returns the model's raw reply to a brief; runPi is the real one.
type flavorWriter func(ctx context.Context, brief []byte) ([]byte, error)

// runPi asks pi, with the castaway-scores-flavor skill and no tools, for the flavor lines.
func runPi(skill string) flavorWriter {
	return func(ctx context.Context, brief []byte) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		prompt := "Write this week's flavor lines following the castaway-scores-flavor skill. Reply with the JSON object only. " +
			"The intro must contain no digits (not even the week number). Each other line must include its facts' names and number.\n\n" + string(brief)
		cmd := exec.CommandContext(ctx, "pi", "-p", "--no-session", "--no-tools", "--skill", skill, prompt)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("pi: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}
}

// applyFlavor swaps in the model's lines where they keep every required name and number; any other line
// keeps the template wording. It returns the lines that fell back.
func applyFlavor(p *scoresPost, reply []byte) []string {
	raw := string(reply)
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	var f scoresFlavor
	if start < 0 || end < start || json.Unmarshal([]byte(raw[start:end+1]), &f) != nil {
		return []string{"all (unreadable reply)"}
	}
	keeps := func(line string, names []string, n int) bool {
		if strings.TrimSpace(line) == "" || strings.ContainsAny(line, "<@\n") || strings.Contains(strings.ToLower(line), "pony") {
			return false
		}
		for _, name := range names {
			if !strings.Contains(line, name) {
				return false
			}
		}
		return strings.Contains(line, strconv.Itoa(n))
	}
	var fellBack []string
	set := func(name string, dst *string, line string, ok bool) {
		if ok {
			*dst = strings.TrimSpace(line)
		} else {
			fellBack = append(fellBack, name)
		}
	}
	x := p.Facts
	set("intro", &p.Intro, f.Intro, strings.TrimSpace(f.Intro) != "" && !strings.ContainsAny(f.Intro, "<@\n0123456789"))
	set("leader", &p.Leader, f.Leader, keeps(f.Leader, x.LeaderNames, x.LeaderTotal))
	if p.Gainer != "" { // no gainer/slider lines when everybody moved the same
		set("gainer", &p.Gainer, f.Gainer, keeps(f.Gainer, x.GainerNames, x.Gain))
		set("slider", &p.Slider, f.Slider, keeps(f.Slider, x.SliderNames, abs(x.Slide)))
	}
	return fellBack
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// addScoresGenerateCommand adds `probst scores generate`: the week's post with fresh numbers and, unless
// --no-ai, flavor lines from pi. It only writes a local draft; `season post --body` saves and schedules it.
func addScoresGenerateCommand(scores *cobra.Command, call apiCall) {
	var week int
	var file, out string
	var noAI bool
	cmd := &cobra.Command{
		Use:   "generate --week N",
		Short: "Draft a week's scores post (fresh numbers + AI flavor) to a local file for you to edit",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			f, err := loadSeasonFile(file)
			if err != nil {
				return err
			}
			tmpl, err := seasonFilePaths(file)
			if err != nil {
				return err
			}
			_, airs, err := expandSeason(f, time.Now())
			if err != nil {
				return err
			}
			if week == 0 {
				week = latestAiredWeek(airs, time.Now())
			}
			p, err := composeScoresData(c.Context(), call, f, airs, week)
			if err != nil {
				return err
			}
			stderr := c.ErrOrStderr()
			if !noAI {
				skill := filepath.Join(filepath.Dir(file), "..", ".agents", "skills", "castaway-scores-flavor", "SKILL.md")
				if err := addFlavor(c.Context(), &p, runPi(skill)); err != nil {
					fmt.Fprintf(stderr, "AI flavor: %v.\n", err)
				}
			}
			text, err := renderScoresPost(tmpl, p)
			if err != nil {
				return err
			}
			if out == "" {
				out = filepath.Join(filepath.Dir(file), "drafts", fmt.Sprintf("%d-week-%d-scores.md", f.Season, week))
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(out, []byte(text), 0o600); err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "%s\nDraft saved to %s. Edit it, then save it for approval with:\n  probst season post %s --week %d --body %s --yes\n", text, out, file, week, out)
			return err
		},
	}
	cmd.Flags().IntVar(&week, "week", 0, "Week N (default: the latest aired episode)")
	cmd.Flags().StringVar(&file, "file", "seasons/51.yaml", "Season file")
	cmd.Flags().StringVar(&out, "out", "", "Draft path (default seasons/drafts/SEASON-week-N-scores.md)")
	cmd.Flags().BoolVar(&noAI, "no-ai", false, "Use the template's rotating lines instead of asking pi")
	scores.AddCommand(cmd)
}

func addFlavor(ctx context.Context, p *scoresPost, write flavorWriter) error {
	brief, err := json.MarshalIndent(map[string]any{
		"season": p.Season, "week": p.Week, "facts": p.Facts,
		"fallback": scoresFlavor{Intro: p.Intro, Leader: p.Leader, Gainer: p.Gainer, Slider: p.Slider},
	}, "", "  ")
	if err != nil {
		return err
	}
	reply, err := write(ctx, brief)
	if err != nil {
		return err
	}
	if fellBack := applyFlavor(p, reply); len(fellBack) > 0 {
		return fmt.Errorf("kept template wording for %s", strings.Join(fellBack, ", "))
	}
	return nil
}

// latestAiredWeek is the last week whose episode has aired (at least 1).
func latestAiredWeek(airs []time.Time, now time.Time) int {
	week := 1
	for i, at := range airs {
		if !at.After(now) {
			week = i + 1
		}
	}
	return week
}

// newGameAdminCommands adds `castawordle private` and `button test`, thin wrappers over the admin API.
func newGameAdminCommands(call apiCall, yes *bool) []*cobra.Command {
	cw := &cobra.Command{Use: "castawordle", Short: "Castawordle admin"}
	var player, name, answerFile string
	private := &cobra.Command{
		Use:   "private GAME_ID --player PARTICIPANT_ID --answer-file FILE",
		Short: "Give one player their own puzzle that replaces their result in GAME_ID (answer read from a file, never printed)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, a []string) error {
			raw, err := os.ReadFile(answerFile) // #nosec G304 -- operator-chosen local file
			if err != nil {
				return err
			}
			if !*yes {
				_, err = fmt.Fprintf(c.OutOrStdout(), "Dry run — would give %s a private %d-letter puzzle %q replacing their result in %s. Re-run with --yes.\n", player, len(strings.TrimSpace(string(raw))), name, a[0])
				return err
			}
			var out struct {
				ID         string `json:"id"`
				WordLength int    `json:"word_length"`
			}
			if err := call(c.Context(), "POST", "/castawordle/"+a[0]+"/private", map[string]string{"participant_id": player, "name": name, "answer": strings.TrimSpace(string(raw))}, &out); err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Created %d-letter private puzzle: https://castaway.bry-guy.net/castawordle/%s\n", out.WordLength, out.ID)
			return err
		},
	}
	private.Flags().StringVar(&player, "player", "", "Participant ID")
	private.Flags().StringVar(&name, "name", "Your Castawordle", "Puzzle name")
	private.Flags().StringVar(&answerFile, "answer-file", "", "File holding the answer")
	cw.AddCommand(private)
	return []*cobra.Command{cw}
}
