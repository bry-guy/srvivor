package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	fpath "path/filepath"

	"github.com/spf13/cobra"

	"github.com/bry-guy/srvivor/shared/config"
	"github.com/bry-guy/srvivor/shared/roster"
	"github.com/bry-guy/srvivor/shared/scorer"
)

func newScoreCmd() *cobra.Command {
	scoreCmd := &cobra.Command{
		Use:   "score [-f --file [filepath] | -d --drafters [drafters]] -s --season [season]",
		Short: "Calculate the score for a Survivor drafts",
		Long:  `Calculate and display the total score for Survivor drafts for a particular season.`,
		Run:   runScore,
	}

	scoreCmd.Flags().StringP("file", "f", "", "Input file containing the draft")
	scoreCmd.Flags().StringSliceP("drafters", "d", []string{}, "Drafter name(s) to lookup the draft")
	scoreCmd.Flags().IntP("season", "s", 0, "Season number of the Survivor game")
	scoreCmd.Flags().Bool("validate", false, "Validate all contestant names against roster before scoring")
	scoreCmd.Flags().BoolP("points-available", "p", false, "Show points available")
	scoreCmd.Flags().Bool("publish", false, "Publish scores to Discord bot")
	scoreCmd.Flags().StringSlice("voted-out", []string{}, "Names of contestants voted out this week")
	if err := scoreCmd.MarkFlagRequired("season"); err != nil {
		slog.Error("creating score command", "error", err)
		os.Exit(1)
	}

	return scoreCmd
}

func runScore(cmd *cobra.Command, args []string) {
	// flags
	drafters, err := cmd.Flags().GetStringSlice("drafters")
	if err != nil {
		slog.Error("parsing drafters flag", "error", err)
		os.Exit(1)
	}

	filepath, err := cmd.Flags().GetString("file")
	if err != nil {
		slog.Error("parsing file flag", "error", err)
		os.Exit(1)
	}

	validate, err := cmd.Flags().GetBool("validate")
	if err != nil {
		slog.Error("parsing validate flag", "error", err)
		os.Exit(1)
	}

	pointsAvailable, err := cmd.Flags().GetBool("points-available")
	if err != nil {
		slog.Error("parsing points-available flag", "error", err)
		os.Exit(1)
	}

	publish, err := cmd.Flags().GetBool("publish")
	if err != nil {
		slog.Error("parsing publish flag", "error", err)
		os.Exit(1)
	}

	votedOut, err := cmd.Flags().GetStringSlice("voted-out")
	if err != nil {
		slog.Error("parsing voted-out flag", "error", err)
		os.Exit(1)
	}

	if publish && len(votedOut) == 0 {
		slog.Error("voted-out is required when publishing")
		os.Exit(1)
	}

	if (filepath != "" && len(drafters) > 0) || (filepath == "" && len(drafters) == 0) {
		slog.Error("You must specify either a file or drafters, but not both")
		os.Exit(1)
	}

	// command season
	season, err := cmd.Flags().GetInt("season")
	if err != nil {
		slog.Error("You must specify a valid season.")
		os.Exit(1)
	}

	slog.Debug("drafters: ", "drafters", drafters)

	if len(drafters) == 1 && drafters[0] == "*" {
		// Wildcard logic
		draftFilepaths, err := fpath.Glob(fmt.Sprintf("./drafts/%d/*.txt", season))
		slog.Debug("draftFilepaths: ", "draftFilepaths", draftFilepaths)
		if err != nil {
			slog.Error("Unable to list draft files.", "error", err)
			os.Exit(1)
		}

		drafters = []string{}
		for _, draftFilepath := range draftFilepaths {
			// drafter should be the name of the txt file, with no directory, stripped of the .txt extension
			drafter := strings.TrimSuffix(fpath.Base(draftFilepath), ".txt")
			slog.Debug("drafter: ", "drafter", drafter)
			drafters = append(drafters, drafter)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		slog.Error("Unable to get working directory.", "error", err)
		os.Exit(1)
	}

	slog.Debug("workdir: ", "workdir", wd)

	var drafts []*scorer.Draft
	failedDrafts := []string{}
	if filepath != "" {
		// Single file mode
		draft, err := scorer.ProcessFile(filepath)
		if err != nil {
			slog.Error("Failed to process file.", "error", err)
			os.Exit(1)
		}
		drafts = append(drafts, draft)
	} else {
		// Drafter mode
		for _, drafter := range drafters {
			filepath := fmt.Sprintf("./drafts/%d/%s.txt", season, drafter)
			draft, err := scorer.ProcessFile(filepath)
			if err != nil {
				slog.Error("Failed to process file.", "error", err)
				failedDrafts = append(failedDrafts, drafter)
				continue
			}
			drafts = append(drafts, draft)
		}
	}

	// Try new location first
	finalFilepath := fmt.Sprintf("./drafts/%d/final.txt", season)
	if _, err := os.Stat(finalFilepath); os.IsNotExist(err) {
		// Fallback to old location with warning
		oldPath := fmt.Sprintf("./finals/%d.txt", season)
		if _, err := os.Stat(oldPath); err == nil {
			slog.Warn("Using deprecated finals location", "old_path", oldPath, "new_path", finalFilepath)
			finalFilepath = oldPath
		} else {
			// Auto-create empty final.txt
			slog.Warn("No finals found, creating empty final.txt", "path", finalFilepath)
			if err := createEmptyFinal(finalFilepath, season); err != nil {
				slog.Error("Failed to create empty final", "error", err)
				os.Exit(1)
			}
		}
	}

	final, err := scorer.ProcessFile(finalFilepath)
	if err != nil {
		slog.Error("Failed to process final file.", "error", err)
		os.Exit(1)
	}

	if validate {
		if len(failedDrafts) > 0 {
			fmt.Printf("Validation failed: failed to process drafts: %v\n", failedDrafts)
			os.Exit(1)
		}
		if err := validateDrafts(drafts, final, season); err != nil {
			os.Exit(1)
		}
		fmt.Printf("Validation passed for season %d\n", season)
	}

	slog.Info("Calculating score for each draft.")
	scores, err := scorer.Scores(drafts, final)
	if err != nil {
		slog.Error("Failed to score drafts.", "error", err)
		os.Exit(1)
	}

	// Sort drafts by score (desc), then points available (desc), then name (asc)
	sort.Slice(drafts, func(i, j int) bool {
		si := scores[drafts[i]]
		sj := scores[drafts[j]]
		if si.Score != sj.Score {
			return si.Score > sj.Score
		}
		if si.PointsAvailable != sj.PointsAvailable {
			return si.PointsAvailable > sj.PointsAvailable
		}
		return drafts[i].Metadata.Drafter < drafts[j].Metadata.Drafter
	})

	if publish {
		cfg, err := config.Validate()
		if err != nil {
			slog.Error("config validation", "error", err)
			os.Exit(1)
		}
		message := buildMessage(season, votedOut, drafts, scores, pointsAvailable)
		err = publishToDiscord(cfg.DiscordBotURL, message, season, votedOut)
		if err != nil {
			slog.Error("failed to publish to Discord", "error", err)
			// continue to print
		}
	}

	// Find the maximum length of drafter names for alignment
	maxLen := 0
	for _, d := range drafts {
		if len(d.Metadata.Drafter) > maxLen {
			maxLen = len(d.Metadata.Drafter)
		}
	}

	for _, d := range drafts {
		result := scores[d]
		if pointsAvailable {
			fmt.Printf("%-*s:\t%d\t(points available: %d)\n", maxLen, d.Metadata.Drafter, result.Score, result.PointsAvailable)
		} else {
			fmt.Printf("%-*s:\t%d\n", maxLen, d.Metadata.Drafter, result.Score)
		}
	}
}

func createEmptyFinal(filepath string, season int) error {
	// Ensure directory exists
	dir := fpath.Dir(filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write metadata
	fmt.Fprintf(file, "Drafter: Current\n")
	fmt.Fprintf(file, "Date: %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(file, "Season: %d\n", season)
	fmt.Fprintf(file, "---\n")

	// Write 1-18 empty positions
	for i := 1; i <= 18; i++ {
		fmt.Fprintf(file, "%d. \n", i)
	}

	return nil
}

func validateDrafts(drafts []*scorer.Draft, final *scorer.Draft, season int) error {
	roster, err := roster.LoadRoster(season)
	if err != nil {
		return fmt.Errorf("failed to load roster: %w", err)
	}

	// Create map of canonical names
	canonicalNames := make(map[string]bool)
	for _, c := range roster.Contestants {
		canonicalNames[c.CanonicalName] = true
	}

	// Validate final
	if err := validateDraft(final, canonicalNames, "final"); err != nil {
		return err
	}

	// Validate drafts
	for _, draft := range drafts {
		if err := validateDraft(draft, canonicalNames, draft.Metadata.Drafter); err != nil {
			return err
		}
	}

	return nil
}

func validateDraft(draft *scorer.Draft, canonicalNames map[string]bool, name string) error {
	var errors []string
	for _, entry := range draft.Entries {
		if entry.PlayerName != "" && !canonicalNames[entry.PlayerName] {
			errors = append(errors, fmt.Sprintf("  %s: %q is not an exact match for any contestant", name, entry.PlayerName))
		}
	}

	if len(errors) > 0 {
		fmt.Printf("Validating drafts for season...\n")
		for _, e := range errors {
			fmt.Println(e)
		}
		fmt.Printf("Validation failed: %d names do not exactly match roster\n", len(errors))
		fmt.Printf("Suggestion: Run 'srvivor fix-drafts -s %d -d \"*\"' to automatically correct names\n", draft.Metadata.Season)
		return fmt.Errorf("validation failed")
	}

	return nil
}

func buildMessage(season int, votedOut []string, drafts []*scorer.Draft, scores map[*scorer.Draft]scorer.ScoreResult, showPoints bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Survivor Season %d Scores**\n", season))
	if len(votedOut) > 0 {
		sb.WriteString(fmt.Sprintf("Voted out this week: %s\n\n", strings.Join(votedOut, ", ")))
	}
	sb.WriteString("**Leaderboard:**\n")
	for i, d := range drafts {
		result := scores[d]
		if showPoints {
			sb.WriteString(fmt.Sprintf("%d. %s: %d (points available: %d)\n", i+1, d.Metadata.Drafter, result.Score, result.PointsAvailable))
		} else {
			sb.WriteString(fmt.Sprintf("%d. %s: %d\n", i+1, d.Metadata.Drafter, result.Score))
		}
	}
	sb.WriteString("\n*Scores calculated automatically.*")
	return sb.String()
}

func publishToDiscord(url, message string, season int, votedOut []string) error {
	payload := map[string]interface{}{
		"message":   message,
		"season":    season,
		"voted_out": votedOut,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		slog.Warn("Discord bot responded with non-200", "status", resp.StatusCode)
	}
	return nil
}
