package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"

	fpath "path/filepath"

	"github.com/bry-guy/srvivor/internal/matcher"
	"github.com/bry-guy/srvivor/internal/roster"
	"github.com/spf13/cobra"
)

func newFixDraftsCmd() *cobra.Command {
	fixCmd := &cobra.Command{
		Use:   "fix-drafts [-s --season [season]] [-d --drafters [drafters]] [--dry-run] [--threshold float]",
		Short: "Fix draft files by normalizing contestant names against the canonical roster",
		Long:  `Fix draft files by normalizing contestant names against the canonical roster for the specified season.`,
		Run:   runFixDrafts,
	}

	fixCmd.Flags().IntP("season", "s", 0, "Season number of the Survivor game")
	fixCmd.Flags().StringSliceP("drafters", "d", []string{}, "Drafter name(s) to fix the draft for")
	fixCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")
	fixCmd.Flags().Float64("threshold", 0.70, "Minimum confidence threshold for fuzzy matching")
	if err := fixCmd.MarkFlagRequired("season"); err != nil {
		slog.Error("creating fix-drafts command", "error", err)
		os.Exit(1)
	}

	return fixCmd
}

func runFixDrafts(cmd *cobra.Command, args []string) {
	// flags
	drafters, err := cmd.Flags().GetStringSlice("drafters")
	if err != nil {
		slog.Error("parsing drafters flag", "error", err)
		os.Exit(1)
	}

	season, err := cmd.Flags().GetInt("season")
	if err != nil {
		slog.Error("parsing season flag", "error", err)
		os.Exit(1)
	}

	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		slog.Error("parsing dry-run flag", "error", err)
		os.Exit(1)
	}

	threshold, err := cmd.Flags().GetFloat64("threshold")
	if err != nil {
		slog.Error("parsing threshold flag", "error", err)
		os.Exit(1)
	}

	slog.Debug("fixing drafts", "season", season, "drafters", drafters, "dryRun", dryRun, "threshold", threshold)

	// Load roster
	seasonRoster, err := roster.LoadRoster(season)
	if err != nil {
		slog.Error("failed to load roster", "error", err)
		os.Exit(1)
	}

	// Handle wildcard
	if len(drafters) == 1 && drafters[0] == "*" {
		draftFilepaths, err := fpath.Glob(fmt.Sprintf("./drafts/%d/*.txt", season))
		if err != nil {
			slog.Error("unable to list draft files", "error", err)
			os.Exit(1)
		}

		drafters = []string{}
		for _, draftFilepath := range draftFilepaths {
			drafter := strings.TrimSuffix(fpath.Base(draftFilepath), ".txt")
			if drafter != "final" { // Skip final.txt
				drafters = append(drafters, drafter)
			}
		}
	}

	// Process each draft
	totalCorrections := 0
	totalErrors := 0

	for _, drafter := range drafters {
		filepath := fmt.Sprintf("./drafts/%d/%s.txt", season, drafter)
		corrections, errors := fixDraft(filepath, seasonRoster, threshold, dryRun)
		totalCorrections += corrections
		totalErrors += errors
	}

	if dryRun {
		fmt.Printf("[DRY RUN] Total: %d corrections would be made, %d errors\n", totalCorrections, totalErrors)
	} else {
		fmt.Printf("Total: %d corrections made, %d errors\n", totalCorrections, totalErrors)
	}
}

func fixDraft(filepath string, seasonRoster *roster.SeasonRoster, threshold float64, dryRun bool) (int, int) {
	// Read file lines
	lines, err := readLines(filepath)
	if err != nil {
		slog.Error("failed to read draft file", "filepath", filepath, "error", err)
		return 0, 1
	}

	corrections := 0
	errors := 0
	modified := false

	if dryRun {
		fmt.Printf("[DRY RUN] Previewing changes for: %s\n", filepath)
	} else {
		fmt.Printf("Fixing draft: %s\n", filepath)
	}

	re := regexp.MustCompile(`^(\d+)\s*(.+)$`)
	parsingMetadata := true
	for i, line := range lines {
		lineNum := i + 1

		// Check for separator
		if line == "---" {
			parsingMetadata = false
			continue
		}

		if parsingMetadata {
			continue // Skip metadata
		}

		// Parse entry with regex
		matches := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 3 {
			continue // Invalid line
		}

		position, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		originalName := strings.TrimSpace(matches[2])
		// Remove leading separators
		originalName = strings.TrimLeft(originalName, ". )")
		originalName = strings.TrimSpace(originalName)
		originalName = strings.Trim(originalName, "\"") // Remove surrounding quotes
		if originalName == "" {
			continue
		}

		// Reformat line
		newLine := fmt.Sprintf("%d. %s", position, originalName)
		if newLine != strings.TrimSpace(line) { // Compare trimmed to handle spacing
			lines[i] = newLine
			fmt.Printf("  FORMAT Line %d: %q -> %q\n", lineNum, strings.TrimSpace(line), newLine)
			modified = true
		}

		// Try to match
		result, err := matcher.MatchContestant(originalName, seasonRoster)
		if err != nil {
			fmt.Printf("  ERROR Line %d: %q - %v\n", lineNum, originalName, err)
			errors++
			continue
		}

		canonicalName := result.Contestant.CanonicalName
		if canonicalName != originalName {
			newLine := strings.Replace(line, originalName, canonicalName, 1)
			lines[i] = newLine
			fmt.Printf("  Line %d: %q -> %q (%s, confidence: %.2f)\n", lineNum, originalName, canonicalName, result.MatchType, result.Score)
			corrections++
			modified = true
		}
	}

	if !dryRun && modified {
		err := writeLines(filepath, lines)
		if err != nil {
			slog.Error("failed to write draft file", "filepath", filepath, "error", err)
			return corrections, errors + 1
		}
		fmt.Printf("Draft saved to: %s\n", filepath)
	} else if dryRun {
		fmt.Printf("[DRY RUN] No changes written to file\n")
	}

	return corrections, errors
}

func readLines(filepath string) ([]string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func writeLines(filepath string, lines []string) error {
	content := strings.Join(lines, "\n")
	return os.WriteFile(filepath, []byte(content), 0644)
}
