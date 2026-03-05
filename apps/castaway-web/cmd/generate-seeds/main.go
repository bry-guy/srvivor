package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/seeddata"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("generate-seeds: %v", err)
	}
}

func run() error {
	legacyDir := getenv("LEGACY_CLI_DIR", "../cli")
	seedFile := getenv("SEED_FILE", "./seeds/historical-seasons.json")

	seasons, err := seeddata.LoadFromLegacy(legacyDir)
	if err != nil {
		return fmt.Errorf("load legacy seasons: %w", err)
	}
	if len(seasons) == 0 {
		return fmt.Errorf("no seasons found under %s", filepath.Join(legacyDir, "drafts"))
	}

	if err := seeddata.WriteJSON(seedFile, seasons); err != nil {
		return fmt.Errorf("write seed file: %w", err)
	}

	fmt.Printf("wrote %d seasons to %s\n", len(seasons), seedFile)
	return nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
