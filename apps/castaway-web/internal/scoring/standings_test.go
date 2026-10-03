package scoring

import "testing"

func TestStandingsTiebreakers(t *testing.T) {
	names := map[string]string{"a": "Zed", "b": "Amy", "c": "Bob", "d": "Cat", "e": "Abe"}
	drafts := map[string][]DraftPick{
		"a": {{Position: 1, ContestantID: "x"}}, // 3 draft points
		"b": {{Position: 2, ContestantID: "x"}}, // 2 draft points + 1 bonus
		"c": {{Position: 2, ContestantID: "x"}}, // 2 draft points + 1 bonus, drafted earlier than b
		"d": {{Position: 2, ContestantID: "x"}}, // 2 + 1, never recorded a submission order
		"e": {{Position: 2, ContestantID: "x"}}, // 2 + 1, no submission order, name sorts before d
	}
	bonus := map[string]int{"b": 1, "c": 1, "d": 1, "e": 1}
	got := CalculateStandings(3, names, drafts, map[string]int{"x": 1}, bonus, map[string]int{"a": 5, "b": 4, "c": 2})
	want := []string{"a", "c", "b", "e", "d"}
	for i, id := range want {
		if got[i].ParticipantID != id {
			t.Fatalf("position %d = %s (%+v), want order %v", i+1, got[i].ParticipantID, got, want)
		}
	}
	if got[0].TotalPoints != got[1].TotalPoints {
		t.Fatalf("fixture should tie on total: %+v", got)
	}
}
