package main

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoreCalculation(t *testing.T) {
	testCases := []struct {
		description       string
		haveDraftFilePath string // filePath to the draft data file
		haveFinalFilePath string // filePath to the final results data file
		want              int    // expected score
	}{
		{
			description:       "0_draft 0_final",
			haveDraftFilePath: "../test_fixtures/drafts/0.txt",
			haveFinalFilePath: "../test_fixtures/finals/0.txt",
			want:              3,
		},
		{
			description:       "1_draft 0_final scores to 6",
			haveDraftFilePath: "../test_fixtures/drafts/1.txt",
			haveFinalFilePath: "../test_fixtures/finals/0.txt",
			want:              6,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			log := slog.Default()
			// Open draft data file
			draftFile, err := os.Open(tc.haveDraftFilePath)
			assert.NoError(t, err, "Failed to open draft file")
			defer draftFile.Close()

			// Read draft
			draft, err := readDraft(draftFile)
			assert.NoError(t, err, "Failed to parse draft")

			// Open final results file
			finalFile, err := os.Open(tc.haveFinalFilePath)
			assert.NoError(t, err, "Failed to open final file")
			defer finalFile.Close()

			// Read final results
			final, err := readDraft(finalFile)
			assert.NoError(t, err, "Failed to parse final results")

			// Calculate and assert score
			got, _ := score(log, draft, final)
			assert.Equal(t, tc.want, got, "Score mismatch")
		})
	}
}
