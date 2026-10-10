package main

import (
	"strings"
	"testing"
)

func ep2Data() survivorEpisode {
	results := []map[string]any{}
	for _, r := range []struct {
		id          float64
		kind, tribe string
		won         float64
	}{{2, "Reward", "Savu", 0}, {2, "Reward", "Toka", 1}, {3, "Immunity", "Savu", 0}, {3, "Immunity", "Toka", 1}} {
		results = append(results, map[string]any{"challenge_id": r.id, "challenge_type": r.kind, "outcome_type": "Tribal", "tribe": r.tribe, "won": r.won})
	}
	return survivorEpisode{Season: 51, Episode: 2, Tables: map[string][]map[string]any{
		"episodes":              {{"episode": 1.0}, {"episode": 2.0}},
		"castaways":             {{"castaway_id": "US0001", "full_name": "Ana Sani", "place": 20.0}, {"castaway_id": "US0002", "full_name": "Kyle", "place": 0.0}},
		"boot_order":            {{"castaway_id": "US0001", "castaway": "Ana", "result": "2nd voted out"}},
		"challenge_description": {{"challenge_id": 2.0, "name": "Five Blind Mice"}, {"challenge_id": 3.0, "name": "Hartford Whalers"}},
		"challenge_results":     results,
	}}
}

func TestPlanEpisode(t *testing.T) {
	roster := newRoster([]contestant{{ID: "ana-id", Name: "Ana Sani"}, {ID: "kyle-id", Name: "Kyle Ostwald"}})
	rules := importRules{TribeFrom: 2}

	p := planEpisode(ep2Data(), "rev", roster, rules)
	if p.Status != "ready" || len(p.Holds) != 0 || len(p.Boots) != 1 || p.Boots[0].Position != 20 || p.Boots[0].ContestantID != "ana-id" {
		t.Fatalf("ep2: %s", p.describe())
	}
	if len(p.Challenges) != 2 || p.Challenges[0].Key != "survivor-ep2-c2-reward" || p.Challenges[0].LegacyKeys[0] != "ep2-reward" ||
		p.Challenges[1].Kind != "immunity" || strings.Join(p.Challenges[1].Tribes, ",") != "Toka" {
		t.Fatalf("ep2 challenges: %s", p.describe())
	}

	mutate := func(f func(d *survivorEpisode)) episodePlan {
		d := ep2Data()
		f(&d)
		return planEpisode(d, "rev", roster, rules)
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["episodes"] = d.Tables["episodes"][:1] }); p.Status != "absent" {
		t.Fatalf("unlisted episode: %s", p.describe())
	}
	// A described challenge without results isn't "no challenge": wait.
	if p := mutate(func(d *survivorEpisode) { d.Tables["challenge_results"] = d.Tables["challenge_results"][:2] }); p.Status != "incomplete" {
		t.Fatalf("missing results: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["challenge_description"] = nil }); p.Status != "incomplete" {
		t.Fatalf("no challenges: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["boot_order"][0]["result"] = "Medically evacuated" }); len(p.Holds) != 0 || len(p.Review) != 1 || len(p.Boots) != 1 {
		t.Fatalf("medevac: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) {
		d.Tables["boot_order"] = append(d.Tables["boot_order"], d.Tables["boot_order"][0])
	}); len(p.Holds) != 0 || len(p.Review) != 1 || len(p.Boots) != 2 {
		t.Fatalf("double boot: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["boot_order"] = nil }); len(p.Holds) != 1 {
		t.Fatalf("no boot: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["castaways"][0]["full_name"] = "Anna Sanny" }); len(p.Holds) != 1 || len(p.Boots) != 0 {
		t.Fatalf("unmatched boot: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { d.Tables["castaways"][0]["place"] = 0.0 }); p.Status != "incomplete" {
		t.Fatalf("no place: %s", p.describe())
	}
	// Both tribes "won" (or nobody did): hold, don't guess.
	if p := mutate(func(d *survivorEpisode) { d.Tables["challenge_results"][0]["won"] = 1.0 }); len(p.Holds) != 1 || len(p.Challenges) != 1 {
		t.Fatalf("tie: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) {
		for _, r := range d.Tables["challenge_results"] {
			r["outcome_type"] = "Individual"
		}
	}); len(p.Holds) != 2 || len(p.Challenges) != 0 {
		t.Fatalf("individual: %s", p.describe())
	}
	// Two rewards in one episode: no legacy key (ambiguous).
	if p := mutate(func(d *survivorEpisode) {
		d.Tables["challenge_results"][2]["challenge_type"] = "Reward"
		d.Tables["challenge_results"][3]["challenge_type"] = "Reward"
	}); len(p.Challenges[0].LegacyKeys) != 0 {
		t.Fatalf("ambiguous legacy key: %s", p.describe())
	}
	// Combined immunity+reward scores both.
	if p := mutate(func(d *survivorEpisode) {
		d.Tables["challenge_results"][2]["challenge_type"] = "Immunity and Reward"
		d.Tables["challenge_results"][3]["challenge_type"] = "Immunity and Reward"
	}); len(p.Challenges) != 3 {
		t.Fatalf("combined: %s", p.describe())
	}
	// Before tribe scoring, and at the merge.
	if p := planEpisode(ep2Data(), "rev", roster, importRules{TribeFrom: 3}); len(p.Challenges) != 0 || len(p.Boots) != 1 {
		t.Fatalf("before tribe scoring: %s", p.describe())
	}
	if p := planEpisode(ep2Data(), "rev", roster, importRules{TribeFrom: 2, MergeEpisode: 2}); len(p.Challenges) != 0 || len(p.Holds) != 1 {
		t.Fatalf("merge: %s", p.describe())
	}
}
