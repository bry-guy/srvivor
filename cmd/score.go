package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

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
	if err != nil || season < 1 {
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

	finalFilepath := fmt.Sprintf("./finals/%d.txt", season)
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
