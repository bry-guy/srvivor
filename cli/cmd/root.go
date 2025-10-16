package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bry-guy/srvivor/shared/config"
	"github.com/bry-guy/srvivor/shared/log"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "srvivor"}

func init() {
	cfg, err := config.Validate()
	if err != nil {
		fmt.Printf("Invalid environment configuration. %v\n", err)
		os.Exit(1)
	}

	slog.SetDefault(log.NewLogger(cfg))

	scoreCmd := newScoreCmd()
	fixCmd := newFixDraftsCmd()
	rootCmd.AddCommand(scoreCmd)
	rootCmd.AddCommand(fixCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("root execute", "error", err)
		os.Exit(1)
	}
}
