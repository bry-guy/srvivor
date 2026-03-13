package seeddata

import "testing"

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
	if season50.Outcomes[21].ContestantName != "Savannah" || season50.Outcomes[22].ContestantName != "Kyle" || season50.Outcomes[23].ContestantName != "Jenna" {
		t.Fatalf("season 50 expected known eliminations for positions 22-24, got %+v", season50.Outcomes[21:24])
	}
}
