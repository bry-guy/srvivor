package main

import (
	"fmt"
	"testing"
)

func TestContextualDraftMatching(t *testing.T) {
	roster := newRoster([]contestant{
		{ID: "brady", Name: "Brady Booker"},
		{ID: "ana", Name: "Ana Sani"},
		{ID: "thien", Name: "Thien An Nguyen"},
		{ID: "jane", Name: "Jane Smith"},
	})
	for _, tc := range []struct {
		name         string
		text         string
		wantReady    bool
		inferredRank int
	}{
		{"numbered with existing fuzzy match", "1 brody\n2 ana\n3 an\n4 jane", true, 3},
		{"unnumbered with chatter", "good luck everyone!\nbrody\nana\nAn\njane", true, 3},
		{"commas", "brody, ana, an, jane", true, 3},
		{"partial name first", "1 an\n2 ana\n3 brody\n4 jane", true, 1},
		{"full show name", "1 brody\n2 ana\n3 thien an\n4 jane", true, 0},
		{"standalone partial name", "An", false, 0},
		{"incomplete draft", "1 brady\n2 ana\n3 an", false, 0},
		{"unknown text", "1 brody\n2 ana\n3 mystery\n4 jane", false, 0},
		{"unknown unnumbered text", "brody\nana\nmystery\njane", false, 0},
		{"not an Ana prefix", "1 brody\n2 an\n3 thien\n4 jane", false, 0},
		{"one letter", "1 brody\n2 ana\n3 n\n4 jane", false, 0},
		{"duplicate contestant", "1 brady\n2 ana\n3 an\n4 ana", false, 0},
		{"duplicate unnumbered contestant", "brady\nana\nan\nana", false, 0},
		{"duplicate rank", "1 brady\n2 ana\n2 an\n4 jane", false, 0},
		{"out of range rank", "1 brady\n2 ana\n5 an\n4 jane", false, 0},
		{"zero rank", "1 brady\n0 ana\n3 an\n4 jane", false, 0},
		{"extra numbered pick", "1 brady\n2 ana\n3 an\n4 jane\n5 unknown", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := parseDraft(tc.text, roster)
			if d.Ready() != tc.wantReady {
				t.Fatalf("ready=%v: %v", d.Ready(), d.Problems)
			}
			inferred := 0
			for i, p := range d.Picks {
				if p.Method != "inferred" {
					continue
				}
				inferred++
				if i+1 != tc.inferredRank || p.Contestant.ID != "thien" || p.Note == "" {
					t.Fatalf("inferred pick %d: %+v", i+1, p)
				}
			}
			if (inferred == 1) != (tc.inferredRank > 0) {
				t.Fatalf("inferred %d picks, want rank %d", inferred, tc.inferredRank)
			}
			if tc.wantReady && (d.Matched != len(roster) || !isDraftCandidate(d, roster)) {
				t.Fatalf("complete inferred draft not recognised: %+v", d)
			}
		})
	}
}

func TestContextualDraftDoesNotResolveMultipleUncertainties(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		text  string
	}{
		{"two partial words", []string{"Brady Wade Booker", "Ana Sani", "Thien An Nguyen", "Jane Smith"}, "1 wade\n2 ana\n3 an\n4 jane"},
		{"fuzzy ambiguity plus partial word", []string{"Kristin Flickinger", "Kristen Frost", "Ana Sani", "Thien An Nguyen"}, "1 kriston\n2 kristen\n3 ana\n4 an"},
		{"no reduced-roster fuzzy matching", []string{"Kristin Flickinger", "Kristen Frost", "Ana Sani", "Thien An Nguyen"}, "1 kriston\n2 kristen\n3 ana\n4 thien"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows []contestant
			for i, name := range tc.names {
				rows = append(rows, contestant{ID: fmt.Sprint(i), Name: name})
			}
			d := parseDraft(tc.text, newRoster(rows))
			if d.Ready() {
				t.Fatalf("uncertain draft accepted: %+v", d)
			}
			for _, p := range d.Picks {
				if p.Method == "inferred" {
					t.Fatalf("uncertain draft inferred a pick: %+v", p)
				}
			}
		})
	}
}

func TestWeakSuggestionsRequireConfirmation(t *testing.T) {
	roster := newRoster([]contestant{
		{ID: "rob", Name: "Rob Antonson"},
		{ID: "sharonda", Name: "Sharonda Cox"},
		{ID: "thien", Name: "Thien An Nguyen"},
		{ID: "brady", Name: "Brady Booker"},
	})
	for _, text := range []string{
		"1 Rhodey Rob\n2 sharonda\n3 thien\n4 brady",
		"1 rob\n2 shondra\n3 thien\n4 brady",
		"1 not Rob\n2 sharonda\n3 thien\n4 brady",
		"1 Unknown Rob\n2 sharonda\n3 thien\n4 brady",
		"1 rob\n2 sharonda\n3 Rhodey Rob\n4 brady",
		"1 Rhodey Rob\n2 Shondra",
	} {
		d := parseDraft(text, roster)
		if d.Ready() || len(d.Order) != 0 {
			t.Fatalf("unconfirmed suggestion produced an order: %+v", d)
		}
		for _, p := range d.Picks {
			if p.Method == "inferred" {
				t.Fatalf("weak suggestion was inferred: %+v", p)
			}
		}
	}
}

func TestAmbiguousAliasIsNotInferred(t *testing.T) {
	roster := newRoster([]contestant{
		{ID: "taylor", Name: "Sam Taylor"},
		{ID: "tucker", Name: "Sam Tucker"},
		{ID: "ana", Name: "Ana Sani"},
	})
	d := parseDraft("1 sam\n2 tucker\n3 ana", roster)
	if d.Ready() || d.Picks[0].Method != "ambiguous" || d.Picks[0].Contestant != nil {
		t.Fatalf("ambiguous alias was inferred: %+v", d)
	}
}
