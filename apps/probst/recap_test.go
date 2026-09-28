package main

import (
	"strings"
	"testing"
)

func TestRenderRecap(t *testing.T) {
	rows := []scoreRow{
		{ID: "a", Name: "Adam", DiscordID: "1", Tribe: "Savu", Draft: 20, Bonus: 2, Total: 22, HasDraft: true},
		{ID: "k", Name: "Keith", Tribe: "Toka", Draft: 21, Bonus: 1, Total: 22, HasDraft: true},
		{ID: "m", Name: "Mooney", Tribe: "Toka", Draft: 15, Bonus: 0, Total: 15, HasDraft: true},
		{ID: "x", Name: "Nodraft", Total: 0},
	}
	outs := []outcome{{ContestantID: "c21", Name: "Aaliyah Puglia", Position: 21}, {ContestantID: "c20", Name: `Danny "Kilby" Kilby`, Position: 20}}
	prev := &scoreSnapshot{Totals: map[string]int{"a": 10, "k": 18, "m": 12}, Eliminated: []string{"c21"}}
	text, snap := renderRecap(51, 2, rows, outs, prev)
	for _, want := range []string{
		"Week 2 Scores",
		"||  Kilby",
		"Biggest gainer: <@1> (+12)",
		"Biggest loser: Mooney (+3)",
		"1. <@1> (Savu): 22 (20+2)\n1. Keith (Toka): 22 (21+1)\n3. Mooney (Toka): 15 (15+0)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Aaliyah") || strings.Contains(text, "Nodraft") {
		t.Errorf("old boot or undrafted player shown:\n%s", text)
	}
	if len(snap.Eliminated) != 2 || snap.Totals["a"] != 22 || len(snap.Totals) != 3 {
		t.Errorf("snapshot = %+v", snap)
	}
}
