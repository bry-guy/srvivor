package main

import (
	"log/slog"
	"os"
	"testing"

	"github.com/jefflinse/melatonin-ext/exec"
	"github.com/jefflinse/melatonin/mt"
	"github.com/stretchr/testify/assert"
)

const (
	usage = `Usage:
  srvivor [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  score       Calculate the score for a Survivor drafts

Flags:
  -h, --help   help for srvivor

Use "srvivor [command] --help" for more information about a command.
`
)

func TestE2EScore(t *testing.T) {
	tests := []mt.TestCase{

		// cmd: none (help)
		exec.Run("srvivor").
			ExpectExitCode(0).
			ExpectStdout(usage).
			ExpectStderr(""),

		// cmd: score drafter+season
		exec.Run("srvivor").
			WithArgs("score", "-d", "bryan", "-s", "45").
			WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			ExpectExitCode(0).
			ExpectStdout("Bryan: 97\n").
			ExpectStderr(""),

		// cmd: score filepath+season
		exec.Run("srvivor").
			WithArgs("score", "-f", "../drafts/45/bryan.txt", "-s", "45").
			WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			ExpectExitCode(0).
			ExpectStdout("Bryan: 97\n").
			ExpectStderr(""),

		// cmd: score drafters+season
		exec.Run("srvivor").
			WithArgs("score", "-d", "bryan,riley", "-s", "45").
			WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			ExpectExitCode(0).
			ExpectStdout("Bryan: 97\nRiley: 87\n").
			ExpectStderr(""),
	}

	results := mt.RunTestsT(t, tests...)

	if results.Failed > 0 {
		mt.PrintResults(results)
	}
}

func TestScoreCalculation(t *testing.T) {
	testCases := []struct {
		description       string
		haveDraftFilePath string // filePath to the draft data file
		haveFinalFilePath string // filePath to the final results data file
		want              int    // expected score
	}{
		{
			description:       "0_draft 0_final scores to 1",
			haveDraftFilePath: "../test_fixtures/drafts/0.txt",
			haveFinalFilePath: "../test_fixtures/finals/0.txt",
			want:              2,
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
