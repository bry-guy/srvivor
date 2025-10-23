package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE2E_Score(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("../bin/srvivor"); os.IsNotExist(err) {
		t.Fatalf("missing srvivor binary: %v", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unable to get working directory: %v", err)
	}
	t.Logf("Working directory: %s", wd)

	// Test cases
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		expected string
		exitCode int
	}{
		{
			name:     "help command",
			args:     []string{},
			env:      map[string]string{},
			expected: "Usage:",
			exitCode: 0,
		},
		{
			name:     "score with drafter and season",
			args:     []string{"score", "-d", "bryan", "-s", "44"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "DEBUG"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score with file and season",
			args:     []string{"score", "-f", "./drafts/44/bryan.txt", "-s", "44"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "DEBUG"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score with multiple drafters for season 44",
			args:     []string{"score", "-d", "bryan,riley", "-s", "44"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score for season 45",
			args:     []string{"score", "-d", "bryan", "-s", "45"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score with file for season 45",
			args:     []string{"score", "-f", "./drafts/45/bryan.txt", "-s", "45"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score with multiple drafters for season 45",
			args:     []string{"score", "-d", "bryan,riley", "-s", "45"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
		{
			name:     "score with wildcard for season 45",
			args:     []string{"score", "-d", "*", "-s", "45"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: "Bryan:\t",
			exitCode: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// #nosec G204 - Test-only code with hardcoded arguments
			cmd := exec.Command("../bin/srvivor", tc.args...)

			// Set environment variables
			cmd.Env = os.Environ() // Start with current environment
			for k, v := range tc.env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Failed to run command: %v", err)
				}
			}

			// Check exit code
			assert.Equal(t, tc.exitCode, exitCode, "Exit code mismatch")

			// Check output contains expected string
			assert.True(t, strings.Contains(stdout.String(), tc.expected),
				"Expected output to contain '%s', got: '%s'", tc.expected, stdout.String())

			// Log the outputs for debugging
			t.Logf("STDOUT: %s", stdout.String())
			t.Logf("STDERR: %s", stderr.String())
		})
	}
}

func TestE2E_ScoreWithValidation(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("../bin/srvivor"); os.IsNotExist(err) {
		t.Fatalf("missing srvivor binary: %v", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unable to get working directory: %v", err)
	}
	t.Logf("Working directory: %s", wd)

	// Test cases
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		expected []string
		exitCode int
	}{
		{
			name:     "score with validation passing",
			args:     []string{"score", "-d", "bryan", "-s", "48", "--validate"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"Validation passed", "Bryan:"},
			exitCode: 0,
		},
		{
			name:     "score with validation failing",
			args:     []string{"score", "-d", "amanda", "-s", "49", "--validate"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"Validation failed", "MC", "is not an exact match"},
			exitCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// #nosec G204 - Test-only code with hardcoded arguments
			cmd := exec.Command("../bin/srvivor", tc.args...)

			// Set environment variables
			cmd.Env = os.Environ() // Start with current environment
			for k, v := range tc.env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Failed to run command: %v", err)
				}
			}

			// Check exit code
			assert.Equal(t, tc.exitCode, exitCode, "Exit code mismatch")

			// Check output contains expected strings
			output := stdout.String() + stderr.String()
			for _, exp := range tc.expected {
				assert.True(t, strings.Contains(output, exp),
					"Expected output to contain '%s', got stdout: '%s', stderr: '%s'", exp, stdout.String(), stderr.String())
			}

			// Log the outputs for debugging
			t.Logf("STDOUT: %s", stdout.String())
			t.Logf("STDERR: %s", stderr.String())
		})
	}
}

func TestE2E_FixDrafts(t *testing.T) {
	// Skip if binary doesn't exist
	if _, err := os.Stat("../bin/srvivor"); os.IsNotExist(err) {
		t.Fatalf("missing srvivor binary: %v", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unable to get working directory: %v", err)
	}
	t.Logf("Working directory: %s", wd)

	// Test cases
	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		expected []string
		exitCode int
	}{
		{
			name:     "fix single draft with dry-run",
			args:     []string{"fix-drafts", "-s", "49", "-d", "amanda", "--dry-run"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"[DRY RUN]", "MC", "Michelle", "Sophie", "Sophie S", "Kristina", "Kristen", "No changes written"},
			exitCode: 0,
		},
		{
			name:     "fix with custom threshold",
			args:     []string{"fix-drafts", "-s", "49", "-d", "bryan", "--threshold", "0.80", "--dry-run"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"[DRY RUN]", "corrections would be made"},
			exitCode: 0,
		},
		{
			name:     "fix without roster file",
			args:     []string{"fix-drafts", "-s", "99", "-d", "test", "--dry-run"},
			env:      map[string]string{"SRVVR_LOG_LEVEL": "ERROR"},
			expected: []string{"failed to load roster"},
			exitCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// #nosec G204 - Test-only code with hardcoded arguments
			cmd := exec.Command("../bin/srvivor", tc.args...)

			// Set environment variables
			cmd.Env = os.Environ() // Start with current environment
			for k, v := range tc.env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Failed to run command: %v", err)
				}
			}

			// Check exit code
			assert.Equal(t, tc.exitCode, exitCode, "Exit code mismatch")

			// Check output contains expected strings
			output := stdout.String() + stderr.String()
			for _, exp := range tc.expected {
				assert.True(t, strings.Contains(output, exp),
					"Expected output to contain '%s', got stdout: '%s', stderr: '%s'", exp, stdout.String(), stderr.String())
			}

			// Log the outputs for debugging
			t.Logf("STDOUT: %s", output)
			t.Logf("STDERR: %s", stderr.String())
		})
	}
}
