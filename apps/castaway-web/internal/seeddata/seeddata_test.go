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
	if got := len(season50.Activities); got != 5 {
		t.Fatalf("season 50 expected 5 activities, got %d", got)
	}
	if season50.Activities[0].ActivityType != "tribal_pony" {
		t.Fatalf("expected first activity to be tribal_pony, got %q", season50.Activities[0].ActivityType)
	}
	if season50.Activities[1].ActivityType != "tribe_wordle" {
		t.Fatalf("expected second activity to be tribe_wordle, got %q", season50.Activities[1].ActivityType)
	}
	if season50.Activities[2].ActivityType != "journey" {
		t.Fatalf("expected third activity to be journey, got %q", season50.Activities[2].ActivityType)
	}
	if season50.Activities[3].ActivityType != "manual_adjustment" {
		t.Fatalf("expected fourth activity to be manual_adjustment (Monty Hall), got %q", season50.Activities[3].ActivityType)
	}
	if season50.Activities[4].ActivityType != "stir_the_pot" {
		t.Fatalf("expected fifth activity to be stir_the_pot, got %q", season50.Activities[4].ActivityType)
	}
	if got := len(season50.Activities[0].GroupAssignments); got != 3 {
		t.Fatalf("tribal pony activity expected 3 group assignments, got %d", got)
	}
	if got := len(season50.Activities[0].Occurrences); got != 3 {
		t.Fatalf("tribal pony activity expected 3 occurrences, got %d", got)
	}
	if got := len(season50.Activities[1].Occurrences); got != 1 {
		t.Fatalf("tribe wordle activity expected 1 occurrence, got %d", got)
	}
	if got := len(season50.Activities[2].ParticipantAssignments); got != 3 {
		t.Fatalf("journey activity expected 3 participant assignments, got %d", got)
	}
	if got := len(season50.Activities[2].Occurrences); got != 4 {
		t.Fatalf("journey activity expected 4 occurrences, got %d", got)
	}

	var mooneySecretRisk struct {
		GuessCount int `json:"guess_count"`
	}
	if err := json.Unmarshal(season50.Activities[2].Occurrences[2].Participants[0].Metadata, &mooneySecretRisk); err != nil {
		t.Fatalf("unmarshal Mooney secret risk metadata: %v", err)
	}
	if mooneySecretRisk.GuessCount != 3 {
		t.Fatalf("expected Mooney secret risk guess_count 3, got %d", mooneySecretRisk.GuessCount)
	}

	var adamSecretRisk struct {
		GuessCount int `json:"guess_count"`
	}
	if err := json.Unmarshal(season50.Activities[2].Occurrences[3].Participants[0].Metadata, &adamSecretRisk); err != nil {
		t.Fatalf("unmarshal Adam secret risk metadata: %v", err)
	}
	if adamSecretRisk.GuessCount != 2 {
		t.Fatalf("expected Adam secret risk guess_count 2, got %d", adamSecretRisk.GuessCount)
	}

	// Verify participant groups
	if got := len(season50.ParticipantGroups); got != 3 {
		t.Fatalf("season 50 expected 3 participant groups, got %d", got)
	}
	groupNames := make(map[string]int)
	for _, g := range season50.ParticipantGroups {
		groupNames[g.Name] = len(g.Memberships)
	}
	if groupNames["Tangerine"] != 5 {
		t.Fatalf("expected Tangerine to have 5 members, got %d", groupNames["Tangerine"])
	}
	if groupNames["Leaf"] != 6 {
		t.Fatalf("expected Leaf to have 6 members, got %d", groupNames["Leaf"])
	}
	if groupNames["Lotus"] != 5 {
		t.Fatalf("expected Lotus to have 5 members, got %d", groupNames["Lotus"])
	}

	// Verify advantages
	if got := len(season50.Advantages); got != 6 {
		t.Fatalf("season 50 expected 6 advantages, got %d", got)
	}
	for _, a := range season50.Advantages {
		if a.AdvantageType != "stir_the_pot_advantage" {
			t.Fatalf("expected all advantages to be stir_the_pot_advantage, got %q", a.AdvantageType)
		}
		if a.GroupName != "Leaf" {
			t.Fatalf("expected all advantages to be for Leaf, got %q", a.GroupName)
		}
	}

	// Verify the latest known local eliminations are reflected.
	if season50.Outcomes[18].ContestantName != "Angelina" {
		t.Fatalf("expected Angelina at position 19, got %q", season50.Outcomes[18].ContestantName)
	}
	if season50.Outcomes[19].ContestantName != "Mike" {
		t.Fatalf("expected Mike at position 20, got %q", season50.Outcomes[19].ContestantName)
	}
}

func TestLoadVerificationMergeGameplaySeed(t *testing.T) {
	seasons, err := LoadFromJSON("../../seeds/verification-merge-gameplay.json")
	if err != nil {
		t.Fatalf("load verification seed: %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("expected exactly 1 verification season, got %d", len(seasons))
	}
	season := seasons[0]
	if season.InstanceName != "Verification Merge Gameplay" {
		t.Fatalf("unexpected verification instance name %q", season.InstanceName)
	}
	if got := len(season.Participants); got != 3 {
		t.Fatalf("expected 3 verification participants, got %d", got)
	}
	if got := len(season.ParticipantGroups); got != 2 {
		t.Fatalf("expected 2 verification participant groups, got %d", got)
	}
	if got := len(season.Activities); got != 2 {
		t.Fatalf("expected 2 verification activities, got %d", got)
	}
	if season.Activities[0].ActivityType != "tribal_pony" {
		t.Fatalf("expected first verification activity to be tribal_pony, got %q", season.Activities[0].ActivityType)
	}
	if season.Activities[1].ActivityType != "manual_adjustment" {
		t.Fatalf("expected second verification activity to be manual_adjustment, got %q", season.Activities[1].ActivityType)
	}
}
