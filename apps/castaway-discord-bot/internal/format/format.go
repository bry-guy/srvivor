package format

import (
	"encoding/json"
	"fmt"
	"sort"
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

func ActivityDetail(detail castaway.ActivityDetail, occurrences []castaway.Occurrence, instance castaway.Instance) string {
	activity := detail.Activity
	var builder strings.Builder
	builder.WriteString("**")
	builder.WriteString(activity.Name)
	builder.WriteString("**\n")
	builder.WriteString("Instance: ")
	builder.WriteString(InstanceLabel(instance))
	builder.WriteString("\nType: ")
	builder.WriteString(activity.ActivityType)
	builder.WriteString("\nStatus: ")
	builder.WriteString(activity.Status)
	if when := strings.TrimSpace(activity.StartsAt); when != "" {
		builder.WriteString("\nStarts: ")
		builder.WriteString(formatTimeLong(when))
	}
	if when := strings.TrimSpace(activity.EndsAt); when != "" {
		builder.WriteString("\nEnds: ")
		builder.WriteString(formatTimeLong(when))
	}

	if lines := activityAssignmentLines(detail); len(lines) > 0 {
		builder.WriteString("\n\n**Assignments**\n")
		for _, line := range lines {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	if len(occurrences) > 0 {
		builder.WriteString("\n**Occurrences**\n")
		for _, occurrence := range occurrences {
			builder.WriteString("- **")
			builder.WriteString(occurrence.Name)
			builder.WriteString("** (")
			builder.WriteString(occurrence.OccurrenceType)
			builder.WriteString(") — ")
			builder.WriteString(occurrence.Status)
			if when := strings.TrimSpace(occurrence.EffectiveAt); when != "" {
				builder.WriteString(" @ ")
				builder.WriteString(formatTime(when))
			}
			builder.WriteString("\n")
		}
	}

	return TrimMessage(strings.TrimSpace(builder.String()))
}

func activityAssignmentLines(detail castaway.ActivityDetail) []string {
	lines := make([]string, 0, len(detail.GroupAssignments)+len(detail.ParticipantAssignments))
	for _, assignment := range detail.GroupAssignments {
		line := assignment.ParticipantGroupName
		if strings.TrimSpace(assignment.Role) != "" {
			line += " — role=" + strings.TrimSpace(assignment.Role)
		}
		if metadata := formatMetadataSummary(assignment.Configuration); metadata != "" {
			line += " [" + metadata + "]"
		}
		lines = append(lines, line)
	}
	for _, assignment := range detail.ParticipantAssignments {
		line := assignment.ParticipantName
		parts := make([]string, 0, 2)
		if strings.TrimSpace(assignment.Role) != "" {
			parts = append(parts, "role="+strings.TrimSpace(assignment.Role))
		}
		if strings.TrimSpace(assignment.ParticipantGroupName) != "" {
			parts = append(parts, "group="+strings.TrimSpace(assignment.ParticipantGroupName))
		}
		if len(parts) > 0 {
			line += " — " + strings.Join(parts, ", ")
		}
		if metadata := formatMetadataSummary(assignment.Configuration); metadata != "" {
			line += " [" + metadata + "]"
		}
		lines = append(lines, line)
	}
	return lines
}

func OccurrencesList(activity castaway.Activity, details []castaway.OccurrenceDetail) string {
	if len(details) == 0 {
		return fmt.Sprintf("**%s**\nNo occurrences found.", activity.Name)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s — Occurrences**\n", activity.Name))
	for _, detail := range details {
		builder.WriteString("- **")
		builder.WriteString(detail.Occurrence.Name)
		builder.WriteString("** (")
		builder.WriteString(detail.Occurrence.OccurrenceType)
		builder.WriteString(") — ")
		builder.WriteString(detail.Occurrence.Status)
		builder.WriteString(" @ ")
		builder.WriteString(formatTime(detail.Occurrence.EffectiveAt))
		if impact := compactOccurrenceImpact(detail); impact != "" {
			builder.WriteString(" · ")
			builder.WriteString(impact)
		}
		builder.WriteString("\n")
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func OccurrenceDetail(detail castaway.OccurrenceDetail, activity castaway.Activity) string {
	var builder strings.Builder
	builder.WriteString("**")
	builder.WriteString(detail.Occurrence.Name)
	builder.WriteString("**\n")
	builder.WriteString("Activity: ")
	builder.WriteString(activity.Name)
	builder.WriteString("\nType: ")
	builder.WriteString(detail.Occurrence.OccurrenceType)
	builder.WriteString("\nStatus: ")
	builder.WriteString(detail.Occurrence.Status)
	builder.WriteString("\nEffective: ")
	builder.WriteString(formatTimeLong(detail.Occurrence.EffectiveAt))

	if recorded := recordedLines(detail); len(recorded) > 0 {
		builder.WriteString("\n\n**Recorded**\n")
		for _, line := range recorded {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	if awards := ledgerLines(detail.Ledger); len(awards) > 0 {
		builder.WriteString("\n**Impact**\n")
		for _, line := range awards {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	return TrimMessage(strings.TrimSpace(builder.String()))
}

func ParticipantHistory(history castaway.ParticipantActivityHistory) string {
	var builder strings.Builder
	builder.WriteString("**")
	builder.WriteString(history.Participant.Name)
	builder.WriteString(" — Activity History**\n")
	builder.WriteString(InstanceLabel(history.Instance))
	if len(history.Activities) == 0 {
		builder.WriteString("\n\nNo activity history found.")
		return TrimMessage(builder.String())
	}

	for _, activity := range history.Activities {
		builder.WriteString("\n\n**")
		builder.WriteString(activity.Activity.Name)
		builder.WriteString("**")
		if activity.Activity.ActivityType != "" {
			builder.WriteString(" (")
			builder.WriteString(activity.Activity.ActivityType)
			builder.WriteString(")")
		}
		builder.WriteString("\n")

		for _, item := range activity.Occurrences {
			label := strings.TrimSpace(item.Occurrence.Name)
			if label == "" {
				label = "Recorded event"
			}
			builder.WriteString("- ")
			builder.WriteString(label)
			if when := strings.TrimSpace(item.Occurrence.EffectiveAt); when != "" {
				builder.WriteString(" @ ")
				builder.WriteString(formatTime(when))
			}
			builder.WriteString("\n")

			for _, line := range historyOccurrenceSubLines(item) {
				builder.WriteString("  - ")
				builder.WriteString(line)
				builder.WriteString("\n")
			}
		}
	}

	return TrimMessage(strings.TrimSpace(builder.String()))
}

func historyOccurrenceSubLines(item castaway.ParticipantActivityHistoryOccurrence) []string {
	lines := make([]string, 0, 4)
	if item.Involvement != nil {
		if result := formatResultLine(item.Involvement.Role, item.Involvement.Result, item.Involvement.ParticipantGroupName); result != "" {
			lines = append(lines, result)
		}
		if metadata := formatHistoryInvolvementMetadata(item.Involvement); metadata != "" {
			lines = append(lines, metadata)
		}
	}
	for _, line := range ledgerLines(item.Ledger) {
		lines = append(lines, "impact: "+line)
	}
	return lines
}

func formatHistoryInvolvementMetadata(involvement *castaway.ParticipantOccurrenceInvolvement) string {
	if involvement == nil || len(involvement.Metadata) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(involvement.Metadata, &payload); err != nil || len(payload) == 0 {
		return ""
	}
	if strings.TrimSpace(involvement.Role) == "adjustment" {
		if reason, ok := payload["reason"].(string); ok && strings.TrimSpace(reason) != "" {
			return "adjustment: " + strings.TrimSpace(reason)
		}
	}
	if summary := formatMetadataSummary(involvement.Metadata); summary != "" {
		return "metadata: " + summary
	}
	return ""
}

func compactOccurrenceImpact(detail castaway.OccurrenceDetail) string {
	parts := make([]string, 0, 3)
	if len(detail.Participants) > 0 {
		parts = append(parts, summarizeRecordedParticipants(detail.Participants))
	}
	if len(detail.Groups) > 0 {
		parts = append(parts, summarizeRecordedGroups(detail.Groups))
	}
	if awards := summarizeLedger(detail.Ledger); awards != "" {
		parts = append(parts, awards)
	}
	return strings.Join(parts, " · ")
}

func summarizeRecordedParticipants(participants []castaway.OccurrenceParticipant) string {
	labels := make([]string, 0, min(len(participants), 3))
	for _, participant := range participants {
		labels = append(labels, compactRecordedParticipant(participant))
		if len(labels) == 3 {
			break
		}
	}
	if len(participants) > len(labels) {
		labels = append(labels, fmt.Sprintf("+%d more", len(participants)-len(labels)))
	}
	return "recorded: " + strings.Join(labels, ", ")
}

func summarizeRecordedGroups(groups []castaway.OccurrenceGroup) string {
	labels := make([]string, 0, min(len(groups), 2))
	for _, group := range groups {
		labels = append(labels, compactRecordedGroup(group))
		if len(labels) == 2 {
			break
		}
	}
	if len(groups) > len(labels) {
		labels = append(labels, fmt.Sprintf("+%d more", len(groups)-len(labels)))
	}
	return "groups: " + strings.Join(labels, ", ")
}

func summarizeLedger(entries []castaway.BonusLedgerEntry) string {
	if len(entries) == 0 {
		return ""
	}
	items := ledgerLines(entries)
	if len(items) > 2 {
		return fmt.Sprintf("impact: %s; +%d more", strings.Join(items[:2], "; "), len(items)-2)
	}
	return "impact: " + strings.Join(items, "; ")
}

func recordedLines(detail castaway.OccurrenceDetail) []string {
	lines := make([]string, 0, len(detail.Participants)+len(detail.Groups))
	for _, participant := range detail.Participants {
		lines = append(lines, recordedParticipantLine(participant))
	}
	for _, group := range detail.Groups {
		lines = append(lines, recordedGroupLine(group))
	}
	return lines
}

func recordedParticipantLine(participant castaway.OccurrenceParticipant) string {
	line := participantLabel(participant)
	if result := formatResultLine(participant.Role, participant.Result, participant.ParticipantGroupName); result != "" {
		line += " — " + result
	}
	if metadata := formatMetadataSummary(participant.Metadata); metadata != "" {
		line += " [" + metadata + "]"
	}
	return line
}

func recordedGroupLine(group castaway.OccurrenceGroup) string {
	line := group.ParticipantGroupName
	if group.Role != "" {
		line += " — role=" + group.Role
	}
	if strings.TrimSpace(group.Result) != "" {
		line += ", result=" + strings.TrimSpace(group.Result)
	}
	if metadata := formatMetadataSummary(group.Metadata); metadata != "" {
		line += " [" + metadata + "]"
	}
	return line
}

func ledgerLines(entries []castaway.BonusLedgerEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	grouped := make(map[string]int)
	ordered := make([]string, 0, len(entries))
	for _, entry := range entries {
		label := strings.TrimSpace(entry.ParticipantName)
		if label == "" {
			label = strings.TrimSpace(entry.SourceGroupName)
		}
		if label == "" {
			label = entry.EntryKind
		}
		delta := fmt.Sprintf("%+d %s", entry.Points, visibilityLabel(entry.Visibility))
		key := fmt.Sprintf("%s — %s", label, delta)
		if _, ok := grouped[key]; !ok {
			ordered = append(ordered, key)
		}
		grouped[key]++
	}
	lines := make([]string, 0, len(ordered))
	for _, key := range ordered {
		count := grouped[key]
		if count > 1 {
			lines = append(lines, fmt.Sprintf("%s each (%d)", key, count))
		} else {
			lines = append(lines, key)
		}
	}
	return lines
}

func formatResultLine(role, result, groupName string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(role) != "" {
		parts = append(parts, "role="+strings.TrimSpace(role))
	}
	if strings.TrimSpace(result) != "" {
		parts = append(parts, "result="+strings.TrimSpace(result))
	}
	if strings.TrimSpace(groupName) != "" {
		parts = append(parts, "group="+strings.TrimSpace(groupName))
	}
	return strings.Join(parts, ", ")
}

func compactRecordedParticipant(participant castaway.OccurrenceParticipant) string {
	if strings.TrimSpace(participant.Result) != "" {
		return fmt.Sprintf("%s=%s", participantLabel(participant), strings.TrimSpace(participant.Result))
	}
	if strings.TrimSpace(participant.Role) != "" {
		return fmt.Sprintf("%s(%s)", participantLabel(participant), strings.TrimSpace(participant.Role))
	}
	return participantLabel(participant)
}

func compactRecordedGroup(group castaway.OccurrenceGroup) string {
	if strings.TrimSpace(group.Result) != "" {
		return fmt.Sprintf("%s=%s", group.ParticipantGroupName, strings.TrimSpace(group.Result))
	}
	if strings.TrimSpace(group.Role) != "" {
		return fmt.Sprintf("%s(%s)", group.ParticipantGroupName, strings.TrimSpace(group.Role))
	}
	return group.ParticipantGroupName
}

func formatMetadataSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload) == 0 {
		return ""
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, min(len(keys), 3))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, payload[key]))
		if len(parts) == 3 {
			break
		}
	}
	if len(keys) > len(parts) {
		parts = append(parts, fmt.Sprintf("+%d more", len(keys)-len(parts)))
	}
	return strings.Join(parts, ", ")
}

func participantLabel(participant castaway.OccurrenceParticipant) string {
	if strings.TrimSpace(participant.ParticipantName) != "" {
		return participant.ParticipantName
	}
	return participant.ParticipantID
}

func visibilityLabel(visibility string) string {
	if strings.TrimSpace(visibility) == "" {
		return "points"
	}
	return strings.TrimSpace(visibility)
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

func formatTimeLong(raw string) string {
	for _, layout := range []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format("2006-01-02 15:04 UTC")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
