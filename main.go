package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"log/slog"

	"github.com/spf13/cobra"
)

// ANSI color codes
const (
	red       = "\033[31m"
	green     = "\033[32m"
	yellow    = "\033[33m"
	lightGray = "\033[37m"
	reset     = "\033[0m"
)

type ColoredHandler struct {
	innerHandler slog.Handler
}

func (h *ColoredHandler) Handle(ctx context.Context, r slog.Record) error {
	color := ""
	switch r.Level {
	case slog.LevelError:
		color = red
	case slog.LevelWarn:
		color = yellow
	case slog.LevelDebug:
		color = lightGray
	default:
		color = reset
	}

	// Prepend the color code to the message
	r.Message = fmt.Sprintf("%s%s%s", color, r.Message, reset)

	return h.innerHandler.Handle(ctx, r)
}

// Enabled delegates the level check to the inner handler
func (h *ColoredHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.innerHandler.Enabled(ctx, level)
}

// Enabled delegates the level check to the inner handler
func (h *ColoredHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.innerHandler.WithAttrs(attrs)
}

func (h *ColoredHandler) WithGroup(name string) slog.Handler {
	return h.innerHandler.WithGroup(name)
}

func main() {
	// Initialize the logger with colored output
	log := slog.New(&ColoredHandler{innerHandler: slog.NewTextHandler(os.Stdout, nil)})

	var rootCmd = &cobra.Command{Use: "srvivor"}

	var cmdScore = &cobra.Command{
		Use:   "score -f --file [filepath]",
		Short: "Calculate the score for a given Survivor game draft",
		Long:  `Calculate and display the total score for a given Survivor game draft based on the provided input file.`,
		Run: func(cmd *cobra.Command, args []string) {
			filepath, _ := cmd.Flags().GetString("file")

			log.Info("Calculating score for file: ", "filepath", filepath)
			file, err := os.Open(filepath)
			if err != nil {
				log.Error("Failed to open file: ", "error", err)
				os.Exit(1)
			}
			defer file.Close()

			draft, err := readDraft(file)
			if err != nil {
				log.Error("Failed to read draft: ", "error", err)
				os.Exit(1)
			}

			file, err = os.Open(filepath)
			if err != nil {
				log.Error("Failed to open file: ", "error", err)
				os.Exit(1)
			}
			defer file.Close()

			final, err := readDraft(file)
			if err != nil {
				log.Error("Failed to read draft: ", "error", err)
				os.Exit(1)
			}

			score := score(log, draft, final)
			log.Info("Total Score: ", "score", score)
		},
	}

	cmdScore.Flags().StringP("file", "f", "", "Input file containing the draft")
	cmdScore.MarkFlagRequired("file")

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
	position      int    // Position in the draft
	positionValue int    // The value assigned to the position
	playerName    string // Name of the Survivor player
}

func score(log *slog.Logger, draft, final draft) int {
	totalScore := 0

	// TODO: Setup luasnips, and decorate score with log.Debug using snippets
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
			// Handle the case where a player in the draft is not in the final results
			log.Error("Player not found in final results", "player_name", draftEntry.playerName)
			continue
		}

		// Calculate the score for this entry
		score := totalPositions - draftEntry.position     // Initial score based on draft position
		score -= abs(draftEntry.position - finalPosition) // Adjust score based on final position

		totalScore += score
	}

	return totalScore
}

// Helper function to calculate the absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func readDraft(reader io.Reader) (draft, error) {
	draft := draft{
		entries: []entry{
			{
				position:      0,
				positionValue: 1,
				playerName:    "test",
			},
		},
	}
	return draft, nil
}
