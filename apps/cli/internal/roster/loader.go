package roster

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed rosters/*.json
var rostersFS embed.FS

// LoadRoster loads a season roster from the embedded JSON file at rosters/[season].json
func LoadRoster(season int) (*SeasonRoster, error) {
	data, err := rostersFS.ReadFile(fmt.Sprintf("rosters/%d.json", season))
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded roster file rosters/%d.json: %w", season, err)
	}

	var roster SeasonRoster
	if err := json.Unmarshal(data, &roster); err != nil {
		return nil, fmt.Errorf("failed to parse roster JSON: %w", err)
	}

	// Validate the roster
	if err := validateRoster(&roster); err != nil {
		return nil, fmt.Errorf("invalid roster: %w", err)
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
