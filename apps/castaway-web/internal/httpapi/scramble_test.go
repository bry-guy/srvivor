package httpapi

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScramblePhrase(t *testing.T) {
	if got, err := scramblePhrase("  the tribe   has spoken ", 4); err != nil || got != "THE TRIBE HAS SPOKEN" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "let's go", "café", strings.Repeat("A", 31)} {
		if _, err := scramblePhrase(bad, 4); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if _, err := scramblePhrase(strings.Repeat("A", 30), 10); err != nil {
		t.Error("30 letters with 10 decoys should fit")
	}
	if _, err := scramblePhrase("SPOKEN", 11); err == nil {
		t.Error("11 decoys should be rejected")
	}
}

func TestScrambleTrayAndSlots(t *testing.T) {
	tray := scrambleTray("HAS SPOKEN", "XY")
	if len(tray) != 11 || !fromTray(tray, "HASSPOKEN") || !fromTray(tray, "XY") {
		t.Fatalf("tray %q", tray)
	}
	if fromTray("ABC", "AAB") || !fromTray("ABCA", "AAB") {
		t.Fatal("fromTray must use each tile once")
	}
	want := [][][]int{{{0, 1, 2}}, {{3, 4, 5, 6, 7, 8, 9, 10, 11}, {12, 13}}}
	if got := scrambleSlots("THE RESOURCEFUL"); !reflect.DeepEqual(got, want) {
		t.Fatalf("slots %v", got)
	}
}

func TestScramblePlaces(t *testing.T) {
	places := scramblePlaces([]scrambleSolve{
		{"slow", 300, 0, false}, {"fast", 60, 2, true}, {"fastClean", 60, 0, true}, {"tieA", 120, 1, true}, {"tieB", 120, 1, false},
	})
	want := map[string]int{"fastClean": 1, "fast": 2, "tieA": 3, "tieB": 3, "slow": 5}
	if !reflect.DeepEqual(places, want) {
		t.Fatalf("places %v", places)
	}
	scores := scrambleScores([]scrambleSolve{
		{"slow", 300, 0, false}, {"fast", 60, 2, true}, {"fastClean", 60, 0, true}, {"tieA", 120, 1, true}, {"tieB", 120, 1, false},
	})
	wantScores := map[string]scrambleScore{ // speed scores nothing; fewest wrong: slow+fastClean tie first (+2), next is third (0)
		"fastClean": {1, 2, 4}, "fast": {1, 0, 2}, "tieA": {1, 0, 2}, "tieB": {0, 0, 1}, "slow": {0, 2, 3},
	}
	if !reflect.DeepEqual(scores, wantScores) {
		t.Fatalf("scores %v", scores)
	}
	if got := scrambleScores([]scrambleSolve{{"a", 10, 2, false}, {"b", 20, 1, false}, {"c", 30, 0, false}}); got["b"] != (scrambleScore{0, 1, 2}) || got["a"] != (scrambleScore{0, 0, 1}) {
		t.Errorf("second places: %v", got)
	}
	if got := scrambleScores([]scrambleSolve{{"a", 10, 0, true}, {"b", 20, 1, true}, {"c", 20, 1, false}}); got["b"] != (scrambleScore{1, 1, 3}) || got["c"] != (scrambleScore{0, 1, 2}) {
		t.Errorf("tie for second: %v", got)
	}
	if scrambleDuration(221) != "3:41" || scrambleDuration(3725) != "1:02:05" {
		t.Error("duration format")
	}
}

func TestScrambleHint(t *testing.T) {
	hint := scrambleHint("SURVIVOR ISLAND", "6f1c2a9e-0000-4000-8000-000000000000")
	words := strings.Split(hint, " · ")
	if len(words) != 2 || hint != scrambleHint("SURVIVOR ISLAND", "6f1c2a9e-0000-4000-8000-000000000000") {
		t.Fatalf("hint %q", hint)
	}
	for i, w := range words {
		cells := strings.Fields(w)
		if len(cells) != len(strings.Fields("SURVIVOR ISLAND")[i]) || len(strings.Fields(strings.ReplaceAll(w, "_", ""))) != 1 {
			t.Errorf("word %d: %q", i, w)
		}
	}
	opens := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) // Wed Oct 14, 8pm EDT
	if got := scrambleHintAt(opens); !got.Equal(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("hint at %v", got)
	}
}
