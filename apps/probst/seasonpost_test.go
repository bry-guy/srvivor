package main

import (
	"strings"
	"testing"
)

func TestScoresPost(t *testing.T) {
	prev := []scoreRow{{ID: "a", Total: 10}, {ID: "b", Total: 9}, {ID: "c", Total: 8}, {ID: "d", Total: 7}, {ID: "e", Total: 3}}
	now := []scoreRow{
		{ID: "a", Name: "Ann", DiscordID: "1", Total: 12, HasDraft: true},
		{ID: "b", Name: "Bo", DiscordID: "2", Total: 7, HasDraft: true},
		{ID: "c", Name: "Cy", DiscordID: "3", Total: 15, HasDraft: true},
		{ID: "d", Name: "Di", DiscordID: "4", Total: 8, HasDraft: true},
		{ID: "e", Name: "Ed", DiscordID: "5", Total: 4, HasDraft: true},
		{ID: "x", Name: "Late", DiscordID: "9", Total: 0},
	}
	p, err := buildScoresPost(51, 3, now, prev, []string{"||  Kristin     ||"}, "https://castaway.bry-guy.net", "🔴 This week's game: **Press the Button**.")
	if err != nil {
		t.Fatal(err)
	}
	text, err := renderScoresPost("../../seasons/51-scores.md", p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Castaway Season 51: Week 3 Scores",
		"Cy", "Kristin", // leader and boot
		"🥇 <@3>: 15", "🥈 <@1>: 12", "🥉 <@4>: 8", "🔦 <@5>: 4",
		"Full scores: <https://castaway.bry-guy.net>", "Press the Button",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(p.Gainer, "Cy") || !strings.Contains(p.Gainer, "7") || !strings.Contains(p.Slider, "Bo") || !strings.Contains(p.Slider, "2") {
		t.Errorf("gainer should be Cy (+7), slider Bo (-2): %q / %q", p.Gainer, p.Slider)
	}
	if strings.Contains(text, "<@2>") || strings.Contains(text, "<@9>") {
		t.Errorf("only the top 3 and last place are pinged:\n%s", text)
	}
	if len(text) > 2000 {
		t.Errorf("post is %d characters; Discord's limit is 2000", len(text))
	}
}

func TestWeeklyLinesDontRepeat(t *testing.T) {
	for name, lines := range map[string][]string{"intro": introLines, "leader": leaderLines, "gainer": gainerLines, "slider": sliderLines} {
		seen := map[string]bool{}
		for week := 1; week <= 13; week++ {
			line := weekLine(lines, week)
			if seen[line] {
				t.Errorf("%s line repeats by week %d", name, week)
			}
			seen[line] = true
		}
	}
}
