package httpapi

import (
	"reflect"
	"strings"
	"testing"
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
	for place, pts := range map[int]int{1: 3, 2: 2, 3: 1, 4: 0, 5: 0} {
		if placePoints(place) != pts {
			t.Errorf("place %d: %d", place, placePoints(place))
		}
	}
	if scrambleDuration(221) != "3:41" || scrambleDuration(3725) != "1:02:05" {
		t.Error("duration format")
	}
}
