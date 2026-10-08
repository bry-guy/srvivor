package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCheckNextGame(t *testing.T) {
	f, err := loadSeasonFile("../../seasons/51.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, airs, err := expandSeason(f, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var alerts []string
	var games []scheduledGame
	call := func(_ context.Context, method, path string, body, out any) error {
		switch {
		case method == "GET" && strings.HasSuffix(path, "/scheduled-games"):
			out.(*struct {
				Games []scheduledGame `json:"games"`
			}).Games = games
		case method == "PUT" && strings.Contains(path, "/admin-alerts/"):
			alerts = append(alerts, path+" "+body.(map[string]string)["body"])
			out.(*struct {
				Created bool `json:"created"`
			}).Created = true
		default:
			t.Fatalf("unexpected %s %s", method, path)
		}
		return nil
	}
	at := func(s string) time.Time {
		t.Helper()
		v, err := time.ParseInLocation("2006-01-02 15:04", s, eastern())
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	ctx := context.Background()

	// Before Episode 3's game opens, week 4 isn't checked yet... the open week is 2, so week 3 is checked.
	games = []scheduledGame{{Type: "press_the_button", Episode: 3}}
	if did, err := checkNextGame(ctx, call, f, airs, at("2026-10-07 19:00")); err != nil || len(alerts) != 0 || !strings.Contains(did, "week 3") {
		t.Fatalf("before ep3 opens: %q %v %v", did, err, alerts)
	}
	// Once Episode 3's game opens, a missing week 4 game alerts the admins.
	did, err := checkNextGame(ctx, call, f, airs, at("2026-10-07 20:00"))
	if err != nil || len(alerts) != 1 || !strings.Contains(alerts[0], "/admin-alerts/s51-w4-game-not-ready") || !strings.Contains(alerts[0], "Week 4") || !strings.Contains(alerts[0], "Wed Oct 14 at 8:00pm") {
		t.Fatalf("missing week 4: %q %v %v", did, err, alerts)
	}

	// A week whose game is picked but not created gets a setup alert; once created, nothing.
	f.Weeks[4] = struct {
		Game       any       `yaml:"game"`
		ScoresPost *planPost `yaml:"scores_post"`
	}{Game: map[string]any{"type": "castawordle"}}
	alerts = nil
	if _, err := checkNextGame(ctx, call, f, airs, at("2026-10-08 09:00")); err != nil || len(alerts) != 1 || !strings.Contains(alerts[0], "isn't set up") {
		t.Fatalf("not created: %v %v", err, alerts)
	}
	games = append(games, scheduledGame{Type: "castawordle", Episode: 4})
	alerts = nil
	if did, err := checkNextGame(ctx, call, f, airs, at("2026-10-08 09:00")); err != nil || len(alerts) != 0 || !strings.Contains(did, "ready") {
		t.Fatalf("ready: %q %v %v", did, err, alerts)
	}
	// "none" is a deliberate no-game week.
	f.Weeks[4] = struct {
		Game       any       `yaml:"game"`
		ScoresPost *planPost `yaml:"scores_post"`
	}{Game: "none"}
	games = games[:1]
	if _, err := checkNextGame(ctx, call, f, airs, at("2026-10-08 09:00")); err != nil || len(alerts) != 0 {
		t.Fatalf("none: %v %v", err, alerts)
	}
	// After the finale opens there's no next week.
	if did, _ := checkNextGame(ctx, call, f, airs, at("2026-12-17 09:00")); did != "no next week to check" {
		t.Fatalf("after finale: %q", did)
	}
}
