package main

import (
	"context"
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

func TestScoresPostNoMovement(t *testing.T) {
	rows := []scoreRow{{ID: "a", Name: "Ann", Total: 3, HasDraft: true}, {ID: "b", Name: "Bo", Total: 1, HasDraft: true}}
	p, err := buildScoresPost(51, 2, rows, rows, nil, "https://x", "")
	if err != nil || p.Gainer != "" || p.Slider != "" || !strings.Contains(p.Leader, "Nobody else moved") {
		t.Fatalf("%+v %v", p, err)
	}
	text, err := renderScoresPost("../../seasons/51-scores.md", p)
	if err != nil || strings.Contains(text, "📈") || strings.Contains(text, "📉") {
		t.Fatalf("%q %v", text, err)
	}
}

func TestScoresFlavorGuardrails(t *testing.T) {
	prev := []scoreRow{{ID: "a", Total: 10}, {ID: "b", Total: 9}}
	now := []scoreRow{{ID: "a", Name: "Ann", DiscordID: "1", Total: 15, HasDraft: true}, {ID: "b", Name: "Bo", DiscordID: "2", Total: 7, HasDraft: true}}
	base, err := buildScoresPost(51, 3, now, prev, nil, "https://x", "")
	if err != nil {
		t.Fatal(err)
	}
	if f := base.Facts; f.LeaderTotal != 15 || f.Gain != 5 || f.Slide != 2 || f.LeaderNames[0] != "Ann" || f.SliderNames[0] != "Bo" {
		t.Fatalf("facts: %+v", f)
	}
	write := func(reply string) flavorWriter {
		return func(context.Context, []byte) ([]byte, error) { return []byte(reply), nil }
	}

	good := base
	reply := "Sure!\n" + `{"intro":"Torches up, castaways.","leader":"Ann rules with 15.","gainer":"Ann surged 5.","slider":"Bo slid 2, but the fire's still lit."}`
	if err := addFlavor(context.Background(), &good, write(reply)); err != nil || good.Leader != "Ann rules with 15." || good.Slider != "Bo slid 2, but the fire's still lit." {
		t.Fatalf("good reply: %v %+v", err, good)
	}

	bad := base
	reply = `{"intro":"Week 3!","leader":"Ann rules with 16.","gainer":"<@1> surged 5.","slider":"Bo, the worst pony, slid 2."}`
	if err := addFlavor(context.Background(), &bad, write(reply)); err == nil {
		t.Fatal("wrong numbers, mentions and banned words should fall back")
	}
	if bad.Intro != base.Intro || bad.Leader != base.Leader || bad.Gainer != base.Gainer || bad.Slider != base.Slider {
		t.Fatalf("rejected lines must keep the template wording: %+v", bad)
	}

	garbage := base
	if err := addFlavor(context.Background(), &garbage, write("no json here")); err == nil || garbage.Leader != base.Leader {
		t.Fatalf("garbage reply: %v %+v", err, garbage)
	}
}
