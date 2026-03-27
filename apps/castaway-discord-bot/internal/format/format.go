package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
)

const DiscordMessageLimit = 2000

func InstanceLabel(instance castaway.Instance) string {
	return fmt.Sprintf("Season %d — %s", instance.Season, instance.Name)
}

func SingleScore(instance castaway.Instance, row castaway.LeaderboardRow) string {
	content := fmt.Sprintf("**%s**\n%s — %s", InstanceLabel(instance), row.ParticipantName, scoreSummary(row, true, true))
	return TrimMessage(content)
}

func Leaderboard(instance castaway.Instance, rows []castaway.LeaderboardRow) string {
	var builder strings.Builder
	builder.WriteString("**")
	builder.WriteString(InstanceLabel(instance))
	builder.WriteString("**\n")
	for index, row := range rows {
		builder.WriteString(fmt.Sprintf("%d. %s — %s\n", index+1, row.ParticipantName, scoreSummary(row, false, false)))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func scoreSummary(row castaway.LeaderboardRow, includePointsLabel bool, includePointsAvailable bool) string {
	label := ""
	if includePointsLabel {
		label = " points"
	}
	if !includePointsAvailable {
		return fmt.Sprintf("%d%s (%d%+d)", row.Total(), label, row.Draft(), row.Bonus())
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

func ActivitiesList(instance castaway.Instance, activities []castaway.Activity) string {
	if len(activities) == 0 {
		return fmt.Sprintf("**%s**\nNo activities found.", InstanceLabel(instance))
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s — Activities**\n", InstanceLabel(instance)))
	for _, a := range activities {
		builder.WriteString(fmt.Sprintf("- **%s** (%s) — %s\n", a.Name, a.ActivityType, a.Status))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func OccurrencesList(activity castaway.Activity, occurrences []castaway.Occurrence) string {
	if len(occurrences) == 0 {
		return fmt.Sprintf("**%s**\nNo occurrences found.", activity.Name)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s — Occurrences**\n", activity.Name))
	for _, o := range occurrences {
		effective := formatTime(o.EffectiveAt)
		builder.WriteString(fmt.Sprintf("- **%s** (%s) — %s @ %s\n", o.Name, o.OccurrenceType, o.Status, effective))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func formatTime(raw string) string {
	for _, layout := range []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("Jan 2 15:04")
		}
	}
	return raw
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
