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
		{"slow", 300, 0}, {"fast", 60, 2}, {"fastClean", 60, 0}, {"tieA", 120, 1}, {"tieB", 120, 1},
	})
	want := map[string]int{"fastClean": 1, "fast": 2, "tieA": 3, "tieB": 3, "slow": 5}
	if !reflect.DeepEqual(places, want) {
		t.Fatalf("places %v", places)
	}
	scores := scrambleScores([]scrambleSolve{
		{"slow", 300, 0}, {"fast", 60, 2}, {"fastClean", 60, 0}, {"tieA", 120, 1}, {"tieB", 120, 1},
	})
	wantScores := map[string]scrambleScore{ // fast ties share first; fewest wrong: slow+fastClean tie, next is third
		"fastClean": {3, 3, 7}, "fast": {3, 0, 4}, "tieA": {0, 0, 1}, "tieB": {0, 0, 1}, "slow": {0, 3, 4},
	}
	if !reflect.DeepEqual(scores, wantScores) {
		t.Fatalf("scores %v", scores)
	}
	if got := scrambleScores([]scrambleSolve{{"a", 10, 2}, {"b", 20, 1}, {"c", 30, 0}}); got["b"] != (scrambleScore{2, 2, 5}) {
		t.Errorf("second places: %v", got)
	}
	if got := scrambleScores([]scrambleSolve{{"a", 10, 0}, {"b", 20, 1}, {"c", 20, 1}}); got["b"] != (scrambleScore{2, 2, 5}) || got["c"] != got["b"] {
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
