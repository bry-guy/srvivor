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
