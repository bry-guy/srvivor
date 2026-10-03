package main

import (
	"strings"
	"testing"
	"time"
)

func TestSeasonPlanTimeline(t *testing.T) {
	f, err := loadSeasonFile("../../seasons/51.yaml")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, eastern())
	items, airs, err := expandSeason(f, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(airs) != 13 || airs[12].Format("2006-01-02 15:04") != "2026-12-16 20:00" {
		t.Fatalf("episodes: %v", airs)
	}
	var b strings.Builder
	printTimeline(&b, items)
	out := b.String()
	for _, want := range []string{
		"Wed Oct 07 11:59am     Game: Castawordle closes + awards",
		"Wed Oct 07 8:00pm      Game: Press the Button opens",
		"Wed Oct 14 11:59am     Game: Press the Button closes + awards",
		"Wed Oct 07 12:00pm     Week 2 scores post",
		"Episode 13 airs — FINALE",
		"Wed Dec 23 12:00pm     FINAL scores post",
		"Post: merge", // TBD placeholder listed, never scheduled
	} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "\nWeek 1\n") != 1 || strings.Count(out, "\nWeek 2\n") != 1 {
		t.Errorf("weeks out of order:\n%s", out)
	}
	if !strings.Contains(out, "Week 3 scores post                                           ⚠ needs approval") {
		t.Errorf("unapproved posts must be flagged:\n%s", out)
	}
}

func TestPlanTime(t *testing.T) {
	base := time.Date(2026, 11, 25, 20, 0, 0, 0, eastern())
	got, err := planTime("+7d 11:59", base)
	if err != nil || got.Format("2006-01-02 15:04 MST") != "2026-12-02 11:59 EST" {
		t.Fatalf("relative: %v %v", got, err)
	}
	if _, err := planTime("TBD", base); err == nil {
		t.Fatal("TBD must not parse as a time")
	}
}
