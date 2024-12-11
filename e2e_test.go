package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/jefflinse/melatonin-ext/exec"
	"github.com/jefflinse/melatonin/mt"
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
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Unable to get working directory. error: %s", err)
		os.Exit(1)
	}

	fmt.Printf("workdir: %s\n", wd)

	tests := []mt.TestCase{

			// cmd: none (help)
			exec.Run("./srvivor").
			ExpectExitCode(0).
			ExpectStdout(usage).
			ExpectStderr(""),

			// cmd: score drafter+season
			exec.Run("./srvivor").
			WithArgs("score", "-d", "bryan", "-s", "44").
			WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "debug"}).
			ExpectExitCode(0).
			ExpectStdout("Bryan: 83\n"),
			// ExpectStderr(""),

			// cmd: score filepath+season  
			exec.Run("./srvivor").
			WithArgs("score", "-f", "drafts/44/bryan.txt", "-s", "44").
			WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "debug"}).
			ExpectExitCode(0).
			ExpectStdout("Bryan: 83\n"),
			// ExpectStderr(""),

			// cmd: score drafters+season
			// exec.Run("./srvivor").
			// WithArgs("score", "-d", "bryan,riley", "-s", "44").
			// WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			// ExpectExitCode(0).
			// ExpectStdout("Bryan: 71\nRiley: 65\n").
			// ExpectStderr(""),

			// // cmd: score drafter+season
			// exec.Run("./srvivor").
			// 	WithArgs("score", "-d", "bryan", "-s", "45").
			// 	WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			// 	ExpectExitCode(0).
			// 	ExpectStdout("Bryan: 97\n").
			// 	ExpectStderr(""),

			// // cmd: score filepath+season
			// exec.Run("./srvivor").
			// 	WithArgs("score", "-f", "drafts/45/bryan.txt", "-s", "45").
			// 	WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			// 	ExpectExitCode(0).
			// 	ExpectStdout("Bryan: 97\n").
			// 	ExpectStderr(""),

			// // cmd: score drafters+season
			// exec.Run("./srvivor").
			// 	WithArgs("score", "-d", "bryan,riley", "-s", "45").
			// 	WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			// 	ExpectExitCode(0).
			// 	ExpectStdout("Bryan: 97\nRiley: 87\n").
			// 	ExpectStderr(""),

			// // cmd: score drafters (wildcard)+season
			// exec.Run("./srvivor").
			// 	WithArgs("score", "-d", "*", "-s", "45").
			// 	WithEnvVars(map[string]string{"SRVVR_LOG_LEVEL": "error"}).
			// 	ExpectExitCode(0).
			// 	ExpectStdout("Bryan: 97\nJosie: 67\nKatie: 69\nKyle: 63\nMooney: 89\nPeter: 75\nRiley: 87\n").
			// 	ExpectStderr(""),
	}

	results := mt.RunTestsT(t, tests...)

	if results.Failed > 0 {
		mt.PrintResults(results)
	}
}
