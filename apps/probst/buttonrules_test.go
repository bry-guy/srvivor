package main

import "testing"

func TestPressTheButtonRules(t *testing.T) {
	tmpl := mustTemplates(t, "../../seasons/51-templates")
	p := func(name string, presses int, rule string, rank int) gameRow {
		return gameRow{Name: name, Presses: presses, RankRule: rule, RankPoints: rank}
	}
	players := []gameRow{
		p("Ann", 1200, "most", 2), p("Bo", 26, "second", -1), p("Cy", 5, "tie", 2), p("Di", 5, "tie", 2),
		p("Ed", 7, "", 0), p("Fay", 1, "least", 1),
	}
	players[0].VolumePoints = 2
	players[1].StreakDays, players[1].StreakPoints = 4, 2
	players[4].StreakDays, players[4].StreakPoints = 4, 2
	players[5].First = true
	players[4].Last = true
	got, err := tmpl.render("games/press_the_button.md", map[string]any{"Tribes": []tribeRow{{Name: "Savu", Emoji: "🟣", Value: 20}, {Name: "Toka", Emoji: "🟡", Value: 67}},
		"Players": players, "Rules": buttonRules(players)})
	if err != nil {
		t.Fatal(err)
	}
	want := `🔴 **Press the Button**: 🟣 Savu 20 presses · 🟡 Toka 67 presses
👑 Most presses: Ann, 1200 (+2)
🥈 Second most: Bo, 26 (-1). So close, so costly.
🤝 Tied at 5: Cy and Di (+2 each)
🐢 Fewest presses: Fay, 1 (+1)
💪 1200 presses: Ann (+2 volume bonus)
🔥 4-day streak: Bo and Ed (+2 each)
⚡ First to press: Fay (+1)
⏰ Last to press: Ed (-1)`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
