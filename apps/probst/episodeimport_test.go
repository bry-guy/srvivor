package main

import (
	"strings"
	"time"
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
	}); len(p.Holds) != 1 || len(p.Boots) != 1 {
		t.Fatalf("same exit twice: %s", p.describe())
	}
	if p := mutate(func(d *survivorEpisode) { // two people left: propose both
		d.Tables["castaways"][1]["place"] = 19.0
		d.Tables["castaways"][1]["full_name"] = "Kyle Ostwald"
		d.Tables["boot_order"] = append(d.Tables["boot_order"], map[string]any{"castaway_id": "US0002", "castaway": "Kyle", "result": "Quit"})
	}); len(p.Holds) != 0 || len(p.Review) != 2 || len(p.Boots) != 2 {
		t.Fatalf("two exits: %s", p.describe())
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

// After the merge, challenges are won by castaways: their Champion pickers score. Any winner that isn't an
// exact roster match holds the challenge; the merge episode itself holds (nobody's Champion is locked yet).
func TestPlanEpisodeChampion(t *testing.T) {
	roster := newRoster([]contestant{{ID: "ana-id", Name: "Ana Sani"}, {ID: "kyle-id", Name: "Kyle Ostwald"}, {ID: "jo-id", Name: "Jo Bloggs"}})
	data := func(winners ...string) survivorEpisode {
		d := ep2Data()
		d.Episode = 9
		d.Tables["episodes"] = append(d.Tables["episodes"], map[string]any{"episode": 9.0})
		d.Tables["castaways"] = append(d.Tables["castaways"], map[string]any{"castaway_id": "US0003", "full_name": "Kyle Ostwald", "place": 0.0},
			map[string]any{"castaway_id": "US0004", "full_name": "Jo Bloggs", "place": 0.0}, map[string]any{"castaway_id": "US0005", "full_name": "Mystery Person", "place": 0.0})
		d.Tables["challenge_description"] = []map[string]any{{"challenge_id": 7.0, "name": "Balance"}, {"challenge_id": 8.0, "name": "Feast"}}
		rows := []map[string]any{{"challenge_id": 7.0, "challenge_type": "Immunity", "outcome_type": "Individual", "castaway_id": "US0003", "won": 1.0},
			{"challenge_id": 7.0, "challenge_type": "Immunity", "outcome_type": "Individual", "castaway_id": "US0004", "won": 0.0}}
		for _, w := range winners { // team reward
			rows = append(rows, map[string]any{"challenge_id": 8.0, "challenge_type": "Reward", "outcome_type": "Team", "castaway_id": w, "won": 1.0})
		}
		rows = append(rows, map[string]any{"challenge_id": 8.0, "challenge_type": "Reward", "outcome_type": "Team", "castaway_id": "US0001", "won": 0.0})
		d.Tables["challenge_results"] = rows
		return d
	}
	rules := importRules{TribeFrom: 2, MergeEpisode: 8}

	p := planEpisode(data("US0003", "US0004"), "rev", roster, rules)
	if len(p.Holds) != 0 || len(p.Challenges) != 0 || len(p.Champions) != 2 ||
		p.Champions[0].Key != "survivor-ep9-c7-immunity" || p.Champions[0].Winners[0].ContestantID != "kyle-id" || len(p.Champions[0].Winners) != 1 ||
		p.Champions[1].Kind != "reward" || len(p.Champions[1].Winners) != 2 {
		t.Fatalf("post-merge: %s", p.describe())
	}
	body := p.importBody(time.Date(2030, 1, 1, 1, 0, 0, 0, time.UTC), false)
	if ch := body["champion_challenges"].([]map[string]any); len(ch) != 2 || strings.Join(ch[1]["winners"].([]string), ",") != "kyle-id,jo-id" {
		t.Fatalf("import body: %v", body)
	}

	if p := planEpisode(data("US0003", "US0005"), "rev", roster, rules); len(p.Champions) != 1 || len(p.Holds) != 1 || !strings.Contains(p.Holds[0], "Mystery Person") {
		t.Fatalf("unmatched winner: %s", p.describe())
	}
	merge := data("US0003")
	merge.Episode = 8
	merge.Tables["episodes"] = append(merge.Tables["episodes"], map[string]any{"episode": 8.0})
	if p := planEpisode(merge, "rev", roster, rules); len(p.Champions) != 0 || len(p.Holds) != 1 || !strings.Contains(p.Holds[0], "merge episode") {
		t.Fatalf("merge episode: %s", p.describe())
	}
}
