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

func TestSingleScoreIncludesInstanceLabel(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 49}
	row := castaway.LeaderboardRow{ParticipantName: "Bryan", Score: 21, PointsAvailable: 46}
	message := SingleScore(instance, row)
	if !strings.Contains(message, "Season 49") || !strings.Contains(message, "Bryan") {
		t.Fatalf("unexpected message: %q", message)
	}
}
