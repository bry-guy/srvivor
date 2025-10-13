package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	fpath "path/filepath"

	"github.com/spf13/cobra"

	"github.com/bry-guy/srvivor/internal/scorer"
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

	slog.Info("Calculating score for each draft.")
	scores, err := scorer.Scores(drafts, final)
	if err != nil {
		slog.Error("Failed to score drafts.", "error", err)
		os.Exit(1)
	}

	for _, d := range drafts {
		result := scores[d]
		fmt.Printf("%s:\t%d\t(points available: %d)\n", d.Metadata.Drafter, result.Score, result.PointsAvailable)
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
