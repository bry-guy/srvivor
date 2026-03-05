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
	for _, season := range seasons {
		if season.Season == 44 {
			found44 = true
		}
		if season.Season == 49 {
			found49 = true
		}
		if len(season.Participants) == 0 {
			t.Fatalf("season %d expected participants", season.Season)
		}
		if len(season.Contestants) == 0 {
			t.Fatalf("season %d expected contestants", season.Season)
		}
	}
	if !found44 || !found49 {
		t.Fatalf("expected seasons 44 and 49 to exist")
	}
}
