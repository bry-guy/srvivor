package roster

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadRoster loads a season roster from the JSON file at rosters/[season].json
func LoadRoster(season int) (*SeasonRoster, error) {
	filename := fmt.Sprintf("./rosters/%d.json", season)

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read roster file %s: %w", filename, err)
	}

	var roster SeasonRoster
	if err := json.Unmarshal(data, &roster); err != nil {
		return nil, fmt.Errorf("failed to parse roster JSON from %s: %w", filename, err)
	}

	// Validate the roster
	if err := validateRoster(&roster); err != nil {
		return nil, fmt.Errorf("invalid roster in %s: %w", filename, err)
	}

	return &roster, nil
}

// validateRoster performs basic validation on the loaded roster
func validateRoster(roster *SeasonRoster) error {
	if roster.Season <= 0 {
		return fmt.Errorf("season must be positive, got %d", roster.Season)
	}

	if len(roster.Contestants) == 0 {
		return fmt.Errorf("roster must contain at least one contestant")
	}

	// Check for duplicate canonical names
	canonicalNames := make(map[string]bool)
	for i, contestant := range roster.Contestants {
		if contestant.CanonicalName == "" {
			return fmt.Errorf("contestant %d has empty canonical_name", i)
		}
		if canonicalNames[contestant.CanonicalName] {
			return fmt.Errorf("duplicate canonical_name: %s", contestant.CanonicalName)
		}
		canonicalNames[contestant.CanonicalName] = true

		if contestant.FirstName == "" {
			return fmt.Errorf("contestant %s has empty first_name", contestant.CanonicalName)
		}
	}

	return nil
}
