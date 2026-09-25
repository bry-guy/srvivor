package main

import (
	"testing"
	"time"
)

func TestParseWordle(t *testing.T) {
	for content, want := range map[string]int{
		"Wordle 1,561 3/6\n\n⬛🟨⬛⬛⬛": 3,
		"wordle 1561 X/6*":            7,
		"got it! Wordle 1.561 6/6":    6,
		"no score here":               0,
		"Wordle 1561 0/6":             0,
	} {
		got, ok := parseWordle(content)
		if got != want || ok != (want != 0) {
			t.Errorf("parseWordle(%q) = %d, %v; want %d", content, got, ok, want)
		}
	}
}

func TestReviewWordleTakesFirstShareInsideWindow(t *testing.T) {
	opens := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	cutoff := opens.Add(7 * 24 * time.Hour)
	players := []participant{{ID: "p1", Name: "Adam", DiscordUserID: "d1"}, {ID: "p2", Name: "Kate", DiscordUserID: "d2"}, {ID: "p3", Name: "Keith"}}
	tribes := []tribe{{ID: "t1", Name: "Savu"}}
	tribes[0].Members = append(tribes[0].Members, struct {
		ParticipantID string `json:"participant_id"`
		Name          string `json:"name"`
	}{ParticipantID: "p1", Name: "Adam"})
	message := func(author string, at time.Time, content string) discordMessage {
		m := discordMessage{Content: content, Timestamp: at}
		m.Author.ID, m.Author.Username = author, author
		return m
	}
	rows := reviewWordle([]discordMessage{
		message("d1", opens.Add(-time.Minute), "Wordle 1 1/6"), // before the window
		message("d1", opens.Add(time.Hour), "Wordle 2 4/6"),
		message("d1", opens.Add(2*time.Hour), "Wordle 2 2/6"), // second share ignored
		message("d2", opens.Add(time.Hour), "Wordle 2 3/6"),   // linked but not on a tribe
		message("d9", opens.Add(time.Hour), "Wordle 2 3/6"),   // not a player
		message("d1", cutoff, "Wordle 3 1/6"),                 // at the cutoff
	}, players, tribes, opens, cutoff)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Player != "Adam" || rows[0].Guesses != 4 || rows[0].Status != "READY" || rows[0].tribeID != "t1" {
		t.Fatalf("Adam row = %+v", rows[0])
	}
	if rows[1].Status == "READY" || rows[2].Status == "READY" {
		t.Fatalf("unexpected READY rows: %+v", rows[1:])
	}
}

func TestParseTribesFile(t *testing.T) {
	players := []participant{{ID: "p1", Name: "Adam"}, {ID: "p2", Name: "Kate"}, {ID: "p3", Name: "Keith"}}
	tribes, err := parseTribesFile("# start\nSavu: Adam, kate\nToka: Keith\n", players)
	if err != nil || len(tribes) != 2 || len(tribes[0]["participant_ids"].([]string)) != 2 {
		t.Fatalf("tribes = %v, err = %v", tribes, err)
	}
	if _, err := parseTribesFile("Savu: Nobody", players); err == nil {
		t.Fatal("unknown player should fail")
	}
}
