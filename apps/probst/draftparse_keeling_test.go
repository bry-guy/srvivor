package main

import (
	"os"
	"slices"
	"testing"
)

func TestKeelingOriginalDraft(t *testing.T) {
	message, err := os.ReadFile("testdata/keeling-original-draft.txt")
	if err != nil {
		t.Fatal(err)
	}
	d := parseDraft(string(message), season51Roster())
	if !d.Ready() {
		t.Fatalf("original draft: %v", d.Problems)
	}
	want := []string{
		"Mike Pinsky", "Brady Booker", "Jenna Doore", `Angelica "Jelly" Loblack`, "Ana Sani",
		"Cristian Chavez", "Maggie Nestor", "Ori Jean-Charles", "Rob Antonson", "Devin Way",
		"Kristin Flickinger", "Alexis Levine", "Carter Krull", "Lewis Kelly", "Sharonda Cox",
		`Danny "Kilby" Kilby`, "Thien An Nguyen", "Linnea Capobianco", "Eric Macksoud", "Patt Cannaday", "Aaliyah Puglia",
	}
	if got := orderNames(d); !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if d.Picks[1].Method != "fuzzy" || d.Picks[16].Method != "inferred" || d.Picks[16].Note == "" {
		t.Fatalf("match methods: Brady=%+v Thien An=%+v", d.Picks[1], d.Picks[16])
	}
}
