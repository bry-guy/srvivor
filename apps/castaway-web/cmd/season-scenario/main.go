package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/scenario"
)

func main() {
	scenarioPath := flag.String("scenario", "", "YAML scenario to compile")
	outputPath := flag.String("output", "", "Hurl output path; stdout when omitted")
	flag.Parse()

	if err := run(*scenarioPath, *outputPath); err != nil {
		log.Fatal(err)
	}
}

func run(scenarioPath, outputPath string) error {
	if scenarioPath == "" {
		return fmt.Errorf("-scenario is required")
	}
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("read scenario %s: %w", scenarioPath, err)
	}
	rendered, err := scenario.RenderYAML(data)
	if err != nil {
		return fmt.Errorf("render scenario %s: %w", scenarioPath, err)
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(rendered)
		return err
	}
	if err := os.WriteFile(outputPath, rendered, 0o600); err != nil {
		return fmt.Errorf("write Hurl file %s: %w", outputPath, err)
	}
	return nil
}
