package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{Use: "srvivor"}

	var cmdScore = &cobra.Command{
		Use:   "score -f --file [filepath]",
		Short: "Calculate the score for a given Survivor game draft",
		Long:  `Calculate and display the total score for a given Survivor game draft based on the provided input file.`,
		Run: func(cmd *cobra.Command, args []string) {
			filepath, _ := cmd.Flags().GetString("file")

			fmt.Println("Calculating score for file:", filepath)
			file, err := os.Open(filepath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()

			draft, err := readDraft(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read draft: %v\n", err)
				os.Exit(1)
			}

			file, err = os.Open(filepath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()

			final, err := readDraft(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read draft: %v\n", err)
				os.Exit(1)
			}

			score := score(draft, final)
			fmt.Println("Total Score:", score)
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

func score(draft draft, final draft) int {
	return draft.entries[0].positionValue
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
