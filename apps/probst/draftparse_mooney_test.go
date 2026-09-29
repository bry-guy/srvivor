package main

import (
	"os"
	"strings"
	"testing"
)

func TestMooneyOriginalDraftNeedsConfirmation(t *testing.T) {
	message, err := os.ReadFile("testdata/mooney-original-draft.txt")
	if err != nil {
		t.Fatal(err)
	}
	d := parseDraft(string(message), season51Roster())
	if d.Ready() || len(d.Order) != 0 || d.Matched != 19 || len(d.Picks) != 21 {
		t.Fatalf("weak matches must not produce a saved order: %+v", d)
	}
	for position, name := range map[int]string{2: "Rob Antonson", 14: "Sharonda Cox"} {
		p := d.Picks[position-1]
		if p.Contestant != nil || p.Method != "unmatched" || !strings.Contains(p.Note, name) || !strings.Contains(p.Note, "requires confirmation") {
			t.Fatalf("rank %d must remain an unconfirmed suggestion: %+v", position, p)
		}
		if !strings.Contains(strings.Join(d.Problems, "\n"), p.Note) {
			t.Fatalf("suggestion omitted from diagnostic: %+v", d)
		}
	}
}
