package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"log/slog"

	fpath "path/filepath"

	"github.com/bry-guy/srvivor/internal/config"
	"github.com/bry-guy/srvivor/internal/log"
	"github.com/spf13/cobra"
)

func main() {
	cfg, err := config.Validate()
	if err != nil {
		fmt.Printf("Invalid environment configuration. %v\n", err)
		os.Exit(1)
	}

	log := log.NewLogger(cfg)

	var rootCmd = &cobra.Command{Use: "srvivor"}

	var cmdScore = &cobra.Command{
		Use:   "score [-f --file [filepath] | -d --drafters [drafters]] -s --season [season]",
		Short: "Calculate the score for a Survivor drafts",
		Long:  `Calculate and display the total score for Survivor drafts for a particular season.`,
		Run: func(cmd *cobra.Command, args []string) {
			drafters, err := cmd.Flags().GetStringSlice("drafters") //nolint:errcheck
			if err != nil {
				log.Error("flag drafters: %v", err)
				os.Exit(1)
			}

			filepath, err := cmd.Flags().GetString("file") //nolint:errcheck
			if err != nil {
				log.Error("flag drafters: %v", err)
				os.Exit(1)
			}

			if (filepath != "" && len(drafters) > 0) || (filepath == "" && len(drafters) == 0) {
				log.Error("You must specify either a file or drafters, but not both")
				os.Exit(1)
			}

			// command season
			season, err := cmd.Flags().GetInt("season")
			if err != nil || season < 1 || season > 45 {
				log.Error("You must specify a valid season.")
				os.Exit(1)
			}

			log.Debug("drafters: ", "drafters", drafters)

			if len(drafters) == 1 && drafters[0] == "*" {
				// Wildcard logic
				draftFilepaths, err := fpath.Glob(fmt.Sprintf("./drafts/%d/*.txt", season))
				log.Debug("draftFilepaths: ", "draftFilepaths", draftFilepaths)
				if err != nil {
					log.Error("Unable to list draft files.", "error", err)
					os.Exit(1)
				}

				drafters = []string{}
				for _, draftFilepath := range draftFilepaths {
					// drafter should be the name of the txt file, with no directory, stripped of the .txt extension
					drafter := strings.TrimSuffix(fpath.Base(draftFilepath), ".txt")
					log.Debug("drafter: ", "drafter", drafter)
					drafters = append(drafters, drafter)
				}
			}

			wd, err := os.Getwd()
			if err != nil {
				log.Error("Unable to get working directory.", "error", err)
				os.Exit(1)
			}

			log.Debug("workdir: ", "workdir", wd)

			var drafts []*draft
			if filepath != "" {
				// Single file mode
				draft, err := processFile(filepath, log)
				if err != nil {
					log.Error("Failed to process file.", "error", err)
					os.Exit(1)
				}
				drafts = append(drafts, draft)
			} else {
				// Drafter mode
				for _, drafter := range drafters {
					filepath := fmt.Sprintf("./drafts/%d/%s.txt", season, drafter)
					draft, err := processFile(filepath, log)
					if err != nil {
						log.Error("Failed to process file.", "error", err)
						continue
					}
					drafts = append(drafts, draft)
				}
			}

			finalFilepath := fmt.Sprintf("./finals/%d.txt", season)
			final, err := processFile(finalFilepath, log)
			if err != nil {
				log.Error("Failed to process final file.", "error", err)
				os.Exit(1)
			}

			log.Info("Calculating score for each draft.")
			scores, err := scores(log, drafts, final)
			if err != nil {
				log.Error("Failed to score drafts.", "error", err)
				os.Exit(1)
			}

			for _, d := range drafts {
				score := scores[d]
				fmt.Printf("%s: %d\n", d.metadata.drafter, score)
			}
		},
	}

	cmdScore.Flags().StringP("file", "f", "", "Input file containing the draft")
	cmdScore.Flags().StringSliceP("drafters", "d", []string{}, "Drafter name(s) to lookup the draft")
	cmdScore.Flags().IntP("season", "s", 0, "Season number of the Survivor game")
	cmdScore.MarkFlagRequired("season")

	rootCmd.AddCommand(cmdScore)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type metadata struct {
	drafter string // Name of the person who created the draft
	date    string // Date of the draft
	season  string // Season or edition of the Survivor game
	hash    string // A unique hash to identify this particular draft
}

type draft struct {
	metadata metadata
	entries  []entry
}

type entry struct {
	position   int    // Position in the draft
	playerName string // Name of the Survivor player
}

func score(log *slog.Logger, draft, final *draft) (int, error) {
	totalScore := 0

	totalPositions := len(final.entries)

	// Map to store the final positions of the players for easy lookup
	finalPositions := make(map[string]int)
	for _, e := range final.entries {
		finalPositions[e.playerName] = e.position
	}

	for _, draftEntry := range draft.entries {
		// Get the final position of the player
		finalPosition, ok := finalPositions[draftEntry.playerName]
		if !ok {
			log.Warn("Player not found in final results", "player", draftEntry.playerName)
			if final.metadata.drafter == "Current" {
				log.Warn("Season is curent. Assuming player has not finished.", "final", final)
				continue
			} else {
				return 0, fmt.Errorf("Player not found in final results: %v", draftEntry.playerName)
			}
		}

		// Calculate the score for this entry
		score := totalPositions - draftEntry.position + 1 // Initial score based on draft position
		score -= abs(draftEntry.position - finalPosition) // Adjust score based on final position

		totalScore += score
	}

	return totalScore, nil
}

func scores(log *slog.Logger, drafts []*draft, final *draft) (map[*draft]int, error) {
	scores := map[*draft]int{}
	for _, draft := range drafts {
		score, err := score(log, draft, final)
		if err != nil {
			return nil, err
		}
		scores[draft] = score
	}
	return scores, nil
}

// Helper function to calculate the absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func readDraft(file *os.File) (*draft, error) {
	// Scan File
	var d draft
	scanner := bufio.NewScanner(file)
	parsingMetadata := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for separator
		if line == "---" {
			parsingMetadata = false
			continue
		}

		if parsingMetadata {
			// Parse metadata
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid metadata length: not 2 parts: %v", line)
			}

			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			switch key {
			case "Drafter":
				d.metadata.drafter = value
			case "Date":
				d.metadata.date = value
			case "Season":
				d.metadata.season = value
			}
		} else {
			// Parse entries
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid entry length: not 2 parts: %v", line)
			}

			position, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid position: not an integer: %v", parts[0])
			}

			if position <= 0 {
				return nil, fmt.Errorf("invalid position: less than zero: %v", parts[0])
			}

			d.entries = append(d.entries, entry{
				position:   position,
				playerName: strings.TrimSpace(parts[1]),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

// Helper function to process a file and read the draft
func processFile(filepath string, log *slog.Logger) (*draft, error) {
	log.Info("Processing file.", "filepath", filepath)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readDraft(file)
}
