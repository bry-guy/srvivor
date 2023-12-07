package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"log/slog"

	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
)

func main() {
	w := os.Stderr

	log := slog.New(tint.NewHandler(w, &tint.Options{
		Level: slog.LevelDebug, // Minimum level to log
	}))

	var rootCmd = &cobra.Command{Use: "srvivor"}

	var cmdScore = &cobra.Command{
		Use:   "score [-f --file [filepath] | -d --draft [draft]] -s --season [season]",
		Short: "Calculate the score for a given Survivor game draft",
		Long:  `Calculate and display the total score for a given Survivor game draft based on the provided input file.`,
		Run: func(cmd *cobra.Command, args []string) {
			// commands filepath and drafter
			filepath, _ := cmd.Flags().GetString("file") //nolint:errcheck
			drafter, _ := cmd.Flags().GetString("draft") //nolint:errcheck

			// are mutually exclusive
			if (filepath != "" && drafter != "") || (filepath == "" && drafter == "") {
				log.Error("You must specify either a file or a drafter, but not both")
				os.Exit(1)
			}

			// command season
			season, err := cmd.Flags().GetInt("season")

			// must be passed
			if err != nil {
				log.Error("You must specify a valid season.")
				os.Exit(1)
			}

			// must be valid
			if season < 1 || season > 45 {
				log.Error("You must specify a valid season.")
				os.Exit(1)
			}

			// if filepath is not passed, look up based on drafter
			if filepath == "" {
				filepath = fmt.Sprintf("./drafts/%d/%s.txt", season, drafter)
			}

			log.Info("Using draft from filepath. ", "filepath", filepath)
			file, err := os.Open(filepath)
			if err != nil {
				log.Error("Failed to open file.", "error", err)
				os.Exit(1)
			}
			defer file.Close()

			draft, err := readDraft(file)
			if err != nil {
				log.Error("Failed to read draft.", "error", err)
				os.Exit(1)
			}

			finalFilepath := fmt.Sprintf("./finals/%d.txt", season)
			log.Info("Using finals from season.", "season", finalFilepath)
			file, err = os.Open(finalFilepath)
			if err != nil {
				log.Error("Failed to open file.", "error", err)
				os.Exit(1)
			}
			defer file.Close()

			final, err := readDraft(file)
			if err != nil {
				log.Error("Failed to read draft.", "error", err)
				os.Exit(1)
			}

			log.Info("Calculating score.", "draft", filepath, "season", finalFilepath)
			score, err := score(draft, final)
			if err != nil {
				log.Error("Failed to score draft.", "error", err)
				os.Exit(1)
			}
			log.Info("Total Score.", "score", score)

			fmt.Printf("%s: %d", draft.metadata.drafter, score)
		},
	}

	cmdScore.Flags().StringP("file", "f", "", "Input file containing the draft")
	cmdScore.Flags().StringP("draft", "d", "", "Drafter name to lookup the draft")
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

func score(draft, final *draft) (int, error) {
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
			return 0, fmt.Errorf("Player not found in final results: %v", draftEntry.playerName)
		}

		// Calculate the score for this entry
		score := totalPositions - draftEntry.position + 1 // Initial score based on draft position
		score -= abs(draftEntry.position - finalPosition) // Adjust score based on final position

		totalScore += score
	}

	return totalScore, nil
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
