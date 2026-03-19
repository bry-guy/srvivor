package seeddata

import (
	"encoding/json"
	"testing"
)

func TestLoadFromLegacy(t *testing.T) {
	seasons, err := LoadFromLegacy("../../../cli")
	if err != nil {
		t.Fatalf("load from legacy: %v", err)
	}
	if len(seasons) == 0 {
		t.Fatalf("expected non-empty seasons")
	}

	found44 := false
	found49 := false
	found50 := false
	var season50 SeasonSeed
	for _, season := range seasons {
		if season.Season == 44 {
			found44 = true
		}
		if season.Season == 49 {
			found49 = true
		}
		if season.Season == 50 {
			found50 = true
			season50 = season
		}
		if len(season.Participants) == 0 {
			t.Fatalf("season %d expected participants", season.Season)
		}
		if len(season.Contestants) == 0 {
			t.Fatalf("season %d expected contestants", season.Season)
		}
	}
	if !found44 || !found49 || !found50 {
		t.Fatalf("expected seasons 44, 49, and 50 to exist")
	}
	if got := len(season50.Participants); got != 16 {
		t.Fatalf("season 50 expected 16 participants, got %d", got)
	}
	if got := len(season50.Contestants); got != 24 {
		t.Fatalf("season 50 expected 24 contestants, got %d", got)
	}
	if got := len(season50.Outcomes); got != 24 {
		t.Fatalf("season 50 expected 24 outcomes, got %d", got)
	}
	if season50.Outcomes[20].ContestantName != "Q" || season50.Outcomes[21].ContestantName != "Savannah" || season50.Outcomes[22].ContestantName != "Kyle" || season50.Outcomes[23].ContestantName != "Jenna" {
		t.Fatalf("season 50 expected known eliminations for positions 21-24, got %+v", season50.Outcomes[20:24])
	}
}

func TestLoadFromJSONSeason50Activities(t *testing.T) {
	seasons, err := LoadFromJSON("../../seeds/historical-seasons.json")
	if err != nil {
		t.Fatalf("load from json: %v", err)
	}

	var season50 *SeasonSeed
	for index := range seasons {
		if seasons[index].Season == 50 {
			season50 = &seasons[index]
			break
		}
	}
	if season50 == nil {
		t.Fatalf("expected season 50 in seed file")
	}
	if got := len(season50.Activities); got != 2 {
		t.Fatalf("season 50 expected 2 activities, got %d", got)
	}
	if season50.Activities[0].ActivityType != "manual_adjustment" {
		t.Fatalf("expected first activity to be manual_adjustment, got %q", season50.Activities[0].ActivityType)
	}
	if season50.Activities[1].ActivityType != "journey" {
		t.Fatalf("expected second activity to be journey, got %q", season50.Activities[1].ActivityType)
	}
	if got := len(season50.Activities[0].Occurrences); got != 6 {
		t.Fatalf("manual adjustment activity expected 6 occurrences, got %d", got)
	}
	if got := len(season50.Activities[1].Occurrences); got != 2 {
		t.Fatalf("journey activity expected 2 occurrences, got %d", got)
	}

	var mooneySecretRisk struct {
		GuessCount int `json:"guess_count"`
	}
	if err := json.Unmarshal(season50.Activities[1].Occurrences[0].Participants[0].Metadata, &mooneySecretRisk); err != nil {
		t.Fatalf("unmarshal Mooney secret risk metadata: %v", err)
	}
	if mooneySecretRisk.GuessCount != 3 {
		t.Fatalf("expected Mooney secret risk guess_count 3, got %d", mooneySecretRisk.GuessCount)
	}

	var adamSecretRisk struct {
		GuessCount int `json:"guess_count"`
	}
	if err := json.Unmarshal(season50.Activities[1].Occurrences[1].Participants[0].Metadata, &adamSecretRisk); err != nil {
		t.Fatalf("unmarshal Adam secret risk metadata: %v", err)
	}
	if adamSecretRisk.GuessCount != 2 {
		t.Fatalf("expected Adam secret risk guess_count 2, got %d", adamSecretRisk.GuessCount)
	}
}
