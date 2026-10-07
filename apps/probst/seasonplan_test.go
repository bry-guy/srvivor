package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
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

func TestSeasonApplySpellItOut(t *testing.T) {
	dir := t.TempDir()
	plan, err := os.ReadFile("../../seasons/51.yaml")
	if err != nil {
		t.Fatal(err)
	}
	plan = []byte(strings.Replace(string(plan), "  4: {game: TBD}", "  4: {game: {type: spell_it_out, phrase_file: ep4.txt, decoys: 4}}", 1))
	if err := os.WriteFile(filepath.Join(dir, "51.yaml"), plan, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ep4.txt"), []byte("the tribe has spoken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	call := func(_ context.Context, method, path string, body, _ any) error {
		if strings.HasSuffix(path, "/scramble-games") {
			sent = body.(map[string]any)
		}
		return nil
	}
	yes := true
	season := &cobra.Command{Use: "season"}
	addSeasonApplyCommand(season, call, &yes)
	var out strings.Builder
	season.SetOut(&out)
	season.SetArgs([]string{"apply", filepath.Join(dir, "51.yaml")})
	if err := season.Execute(); err != nil {
		t.Fatal(err)
	}
	if sent["phrase"] != "the tribe has spoken" || sent["decoys"] != 4 || sent["episode_number"] != 4 || sent["opens_at"] != "2026-10-15T00:00:00Z" {
		t.Fatalf("sent %v", sent)
	}
	if !strings.Contains(out.String(), "applied Week 4 Spell It Out") || strings.Contains(out.String(), "spoken") {
		t.Fatalf("output %s", out.String())
	}
}
