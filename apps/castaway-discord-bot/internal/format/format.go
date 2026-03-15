package format

import (
	"fmt"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
)

const DiscordMessageLimit = 2000

func InstanceLabel(instance castaway.Instance) string {
	return fmt.Sprintf("Season %d — %s", instance.Season, instance.Name)
}

func SingleScore(instance castaway.Instance, row castaway.LeaderboardRow) string {
	content := fmt.Sprintf("**%s**\n%s — %s", InstanceLabel(instance), row.ParticipantName, scoreSummary(row, true))
	return TrimMessage(content)
}

func Leaderboard(instance castaway.Instance, rows []castaway.LeaderboardRow) string {
	var builder strings.Builder
	builder.WriteString("**")
	builder.WriteString(InstanceLabel(instance))
	builder.WriteString("**\n")
	for index, row := range rows {
		builder.WriteString(fmt.Sprintf("%d. %s — %s\n", index+1, row.ParticipantName, scoreSummary(row, false)))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func scoreSummary(row castaway.LeaderboardRow, includePointsLabel bool) string {
	label := ""
	if includePointsLabel {
		label = " points"
	}
	return fmt.Sprintf("%d%s (%d%+d; points available: %d)", row.Total(), label, row.Draft(), row.Bonus(), row.PointsAvailable)
}

func Draft(instance castaway.Instance, draft castaway.Draft) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s Draft** — %s\n", draft.Participant.Name, InstanceLabel(instance)))
	for _, pick := range draft.Picks {
		builder.WriteString(fmt.Sprintf("%d. %s\n", pick.Position, pick.ContestantName))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func InstanceList(instances []castaway.Instance) string {
	if len(instances) == 0 {
		return "No instances found."
	}
	var builder strings.Builder
	builder.WriteString("**Instances**\n")
	for _, instance := range instances {
		builder.WriteString("- ")
		builder.WriteString(InstanceLabel(instance))
		builder.WriteString("\n")
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func TrimMessage(content string) string {
	if len(content) <= DiscordMessageLimit {
		return content
	}
	const suffix = "\n…(truncated)"
	limit := DiscordMessageLimit - len(suffix)
	if limit < 0 {
		return suffix
	}
	return strings.TrimSpace(content[:limit]) + suffix
}
