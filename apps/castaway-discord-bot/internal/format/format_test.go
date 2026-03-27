package format

import (
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
)

func TestTrimMessageTruncatesLongResponses(t *testing.T) {
	content := strings.Repeat("a", DiscordMessageLimit+50)
	trimmed := TrimMessage(content)
	if len(trimmed) > DiscordMessageLimit {
		t.Fatalf("expected trimmed content <= %d, got %d", DiscordMessageLimit, len(trimmed))
	}
	if !strings.Contains(trimmed, "truncated") {
		t.Fatalf("expected truncation marker, got %q", trimmed)
	}
}

func TestSingleScoreIncludesTotalDraftAndBonus(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 49}
	row := castaway.LeaderboardRow{
		ParticipantName: "Bryan",
		Score:           26,
		DraftPoints:     21,
		BonusPoints:     5,
		TotalPoints:     26,
		PointsAvailable: 46,
	}

	message := SingleScore(instance, row)
	expected := "**Season 49 — Office Pool**\nBryan — 26 points (21+5; points available: 46)"
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestActivitiesListFormatsCompactOutput(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 50}
	activities := []castaway.Activity{
		{ID: "a1", Name: "Tribal Pony", ActivityType: "tribal_pony", Status: "active"},
		{ID: "a2", Name: "Journey 1", ActivityType: "journey", Status: "completed"},
	}

	message := ActivitiesList(instance, activities)
	expected := strings.Join([]string{
		"**Season 50 — Office Pool — Activities**",
		"- **Tribal Pony** (tribal_pony) — active",
		"- **Journey 1** (journey) — completed",
	}, "\n")
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestActivitiesListEmptyState(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 50}
	message := ActivitiesList(instance, nil)
	expected := "**Season 50 — Office Pool**\nNo activities found."
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestOccurrencesListFormatsCompactOutput(t *testing.T) {
	activity := castaway.Activity{ID: "a1", Name: "Tribal Pony"}
	occurrences := []castaway.Occurrence{
		{ID: "o1", Name: "Ep 1 Immunity", OccurrenceType: "immunity_result", Status: "resolved", EffectiveAt: "2026-03-05T01:00:00Z"},
		{ID: "o2", Name: "Ep 2 Immunity", OccurrenceType: "immunity_result", Status: "recorded", EffectiveAt: "2026-03-12T01:00:00Z"},
	}

	message := OccurrencesList(activity, occurrences)
	expected := strings.Join([]string{
		"**Tribal Pony — Occurrences**",
		"- **Ep 1 Immunity** (immunity_result) — resolved @ Mar 5 01:00",
		"- **Ep 2 Immunity** (immunity_result) — recorded @ Mar 12 01:00",
	}, "\n")
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestOccurrencesListEmptyState(t *testing.T) {
	activity := castaway.Activity{ID: "a1", Name: "Tribal Pony"}
	message := OccurrencesList(activity, nil)
	expected := "**Tribal Pony**\nNo occurrences found."
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestLeaderboardIncludesTotalDraftAndBonus(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 49}
	rows := []castaway.LeaderboardRow{
		{ParticipantName: "Bryan", Score: 26, DraftPoints: 21, BonusPoints: 5, TotalPoints: 26, PointsAvailable: 46},
		{ParticipantName: "Riley", Score: 19, DraftPoints: 19, BonusPoints: 0, TotalPoints: 19, PointsAvailable: 41},
	}

	message := Leaderboard(instance, rows)
	expected := strings.Join([]string{
		"**Season 49 — Office Pool**",
		"1. Bryan — 26 (21+5)",
		"2. Riley — 19 (19+0)",
	}, "\n")
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}
