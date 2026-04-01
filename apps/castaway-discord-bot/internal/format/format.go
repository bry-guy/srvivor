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
	seasonLabel := fmt.Sprintf("Season %d", instance.Season)
	name := strings.TrimSpace(instance.Name)
	if name == "" || strings.EqualFold(name, seasonLabel) {
		return seasonLabel
	}
	return fmt.Sprintf("%s — %s", seasonLabel, name)
}

func SingleScore(instance castaway.Instance, row castaway.LeaderboardRow, publicBonusPoints, secretBonusPoints int) string {
	pointsAvailable := row.PointsAvailable - secretBonusPoints
	content := strings.Join([]string{
		fmt.Sprintf("**Season %d: %s Points**", instance.Season, row.ParticipantName),
		fmt.Sprintf("%s: %d points", row.ParticipantName, row.Total()),
		fmt.Sprintf("- Draft Points: %d", row.Draft()),
		fmt.Sprintf("- Bonus Points: %d", publicBonusPoints),
		fmt.Sprintf("- Secret Bonus Points: %d", secretBonusPoints),
		fmt.Sprintf("- Points Available: %d", pointsAvailable),
	}, "\n")
	return TrimMessage(content)
}

func Leaderboard(instance castaway.Instance, rows []castaway.LeaderboardRow) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: Leaderboard**\n", instance.Season))
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
	builder.WriteString(fmt.Sprintf("**Season %d: %s Draft**\n", instance.Season, draft.Participant.Name))
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
	builder.WriteString(fmt.Sprintf("**Season %d: Activities**\n", instance.Season))
	for _, a := range activities {
		builder.WriteString(fmt.Sprintf("- **%s** (%s) — %s\n", a.Name, a.ActivityType, a.Status))
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func ActivityDetail(detail castaway.ActivityDetail, occurrences []castaway.Occurrence, instance castaway.Instance) string {
	activity := detail.Activity
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**Season %d: %s**\n", instance.Season, activity.Name))
	builder.WriteString(fmt.Sprintf("- Type: %s\n", activity.ActivityType))
	builder.WriteString(fmt.Sprintf("- Status: %s\n", activity.Status))
	if when := strings.TrimSpace(activity.StartsAt); when != "" {
		builder.WriteString(fmt.Sprintf("- Starts: %s\n", formatTimeLong(when)))
	}
	if when := strings.TrimSpace(activity.EndsAt); when != "" {
		builder.WriteString(fmt.Sprintf("- Ends: %s\n", formatTimeLong(when)))
	}

	if lines := activityAssignmentLines(detail); len(lines) > 0 {
		builder.WriteString("\n**Assignments**\n")
		for _, line := range lines {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	if len(occurrences) > 0 {
		builder.WriteString("\n**Occurrences**\n")
		for _, occurrence := range occurrences {
			builder.WriteString(occurrence.Name)
			builder.WriteString("\n")
			builder.WriteString(fmt.Sprintf("- Type: %s\n", occurrence.OccurrenceType))
			builder.WriteString(fmt.Sprintf("- Status: %s\n", occurrence.Status))
			if when := strings.TrimSpace(occurrence.EffectiveAt); when != "" {
				builder.WriteString(fmt.Sprintf("- Date: %s\n", formatTime(when)))
			}
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
	builder.WriteString(fmt.Sprintf("**%s: Occurrences**\n", activity.Name))
	for detailIndex, detail := range details {
		if detailIndex > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(detail.Occurrence.Name)
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("- Status: %s\n", detail.Occurrence.Status))
		if when := strings.TrimSpace(detail.Occurrence.EffectiveAt); when != "" {
			builder.WriteString(fmt.Sprintf("- Date: %s\n", formatTime(when)))
		}
		if awards := ledgerLines(detail.Ledger); len(awards) > 0 {
			for _, line := range awards {
				builder.WriteString("- Impact: ")
				builder.WriteString(line)
				builder.WriteString("\n")
			}
		}
	}
	return TrimMessage(strings.TrimSpace(builder.String()))
}

func OccurrenceDetail(detail castaway.OccurrenceDetail, activity castaway.Activity) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s**\n", detail.Occurrence.Name))
	builder.WriteString(fmt.Sprintf("- Activity: %s\n", activity.Name))
	builder.WriteString(fmt.Sprintf("- Type: %s\n", detail.Occurrence.OccurrenceType))
	builder.WriteString(fmt.Sprintf("- Status: %s\n", detail.Occurrence.Status))
	builder.WriteString(fmt.Sprintf("- Date: %s\n", formatTimeLong(detail.Occurrence.EffectiveAt)))

	if recorded := recordedLines(detail); len(recorded) > 0 {
		builder.WriteString("\n**Recorded**\n")
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
	builder.WriteString(fmt.Sprintf("**Season %d: %s History**", history.Instance.Season, history.Participant.Name))

	currentEpisodeNumber := int32(-1)
	if history.Instance.CurrentEpisode != nil {
		currentEpisodeNumber = history.Instance.CurrentEpisode.EpisodeNumber
		builder.WriteString(fmt.Sprintf(" (currently %s)", strings.TrimSpace(history.Instance.CurrentEpisode.Label)))
	}

	episodeGroups := groupHistoryByEpisode(history)
	if len(episodeGroups) == 0 {
		builder.WriteString("\n\nNo activity history found.")
		return TrimMessage(builder.String())
	}

	for _, group := range episodeGroups {
		if currentEpisodeNumber >= 0 && group.episodeNumber > currentEpisodeNumber {
			break
		}
		builder.WriteString("\n\n**")
		builder.WriteString(group.label)
		builder.WriteString("**")
		if len(group.items) == 0 {
			builder.WriteString("\n\nn/a")
			continue
		}
		builder.WriteString("\n")
		for itemIndex, item := range group.items {
			if itemIndex > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(item.activityName)
			builder.WriteString("\n")
			for _, detail := range item.details {
				builder.WriteString("- ")
				builder.WriteString(detail)
				builder.WriteString("\n")
			}
		}
	}

	return TrimMessage(strings.TrimSpace(builder.String()))
}

type historyEpisodeGroup struct {
	episodeNumber int32
	label         string
	items         []historyEpisodeItem
}

type historyEpisodeItem struct {
	activityName string
	details      []string
}

func groupHistoryByEpisode(history castaway.ParticipantActivityHistory) []historyEpisodeGroup {
	groups := make([]historyEpisodeGroup, 0)
	groupIndex := make(map[int32]int)
	for _, episode := range history.Instance.Episodes {
		label := strings.TrimSpace(episode.Label)
		if label == "" {
			label = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
		}
		groups = append(groups, historyEpisodeGroup{episodeNumber: episode.EpisodeNumber, label: label})
		groupIndex[episode.EpisodeNumber] = len(groups) - 1
	}
	if len(groups) == 0 {
		groups = append(groups, historyEpisodeGroup{episodeNumber: 0, label: "History"})
		groupIndex[0] = 0
	}

	for _, activity := range history.Activities {
		for _, occurrence := range activity.Occurrences {
			episodeNumber := historyEpisodeNumberForOccurrence(history.Instance.Episodes, occurrence.Occurrence.EffectiveAt)
			index, ok := groupIndex[episodeNumber]
			if !ok {
				label := fmt.Sprintf("Episode %d", episodeNumber)
				groups = append(groups, historyEpisodeGroup{episodeNumber: episodeNumber, label: label})
				index = len(groups) - 1
				groupIndex[episodeNumber] = index
			}
			groups[index].items = append(groups[index].items, historyEpisodeItem{
				activityName: activity.Activity.Name,
				details:      historyEpisodeDetails(occurrence),
			})
		}
	}
	return groups
}

func historyEpisodeNumberForOccurrence(episodes []castaway.InstanceEpisode, effectiveAt string) int32 {
	if len(episodes) == 0 {
		return 0
	}
	when, ok := parseTimeValue(effectiveAt)
	if !ok {
		return 0
	}
	current := episodes[0].EpisodeNumber
	for _, episode := range episodes {
		airsAt, ok := parseTimeValue(episode.AirsAt)
		if !ok {
			continue
		}
		if when.Before(airsAt) {
			break
		}
		current = episode.EpisodeNumber
	}
	return current
}

func historyEpisodeDetails(item castaway.ParticipantActivityHistoryOccurrence) []string {
	lines := make([]string, 0, 4)
	label := strings.TrimSpace(item.Occurrence.Name)
	if label == "" {
		label = "Recorded event"
	}
	if when := strings.TrimSpace(item.Occurrence.EffectiveAt); when != "" {
		label += " @ " + formatTime(when)
	}
	lines = append(lines, label)
	if action := historyActionSummary(item.Involvement); action != "" {
		lines = append(lines, "action: "+action)
	}
	if result := historyResultSummary(item.Involvement); result != "" {
		lines = append(lines, "result: "+result)
	}
	if impact := historyImpactSummary(item.Ledger); impact != "" {
		lines = append(lines, "impact: "+impact)
	}
	return lines
}

func historyActionSummary(involvement *castaway.ParticipantOccurrenceInvolvement) string {
	if involvement == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if role := strings.TrimSpace(involvement.Role); role != "" {
		parts = append(parts, role)
	}
	if groupName := strings.TrimSpace(involvement.ParticipantGroupName); groupName != "" {
		parts = append(parts, "group="+groupName)
	}
	return strings.Join(parts, ", ")
}

func historyResultSummary(involvement *castaway.ParticipantOccurrenceInvolvement) string {
	if involvement == nil {
		return ""
	}
	if result := strings.TrimSpace(involvement.Result); result != "" {
		return result
	}
	if len(involvement.Metadata) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(involvement.Metadata, &payload); err != nil || len(payload) == 0 {
		return ""
	}
	if strings.TrimSpace(involvement.Role) == "adjustment" {
		if reason, ok := payload["reason"].(string); ok && strings.TrimSpace(reason) != "" {
			return strings.TrimSpace(reason)
		}
	}
	filtered := make(map[string]any, len(payload))
	for key, value := range payload {
		switch key {
		case "award_key", "reason":
			continue
		default:
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return ""
	}
	return formatMetadataSummary(encoded)
}

func historyImpactSummary(entries []castaway.BonusLedgerEntry) string {
	if len(entries) == 0 {
		return ""
	}
	items := make([]string, 0, len(entries))
	for _, entry := range entries {
		items = append(items, fmt.Sprintf("%+d %s", entry.Points, visibilityLabel(entry.Visibility)))
	}
	return strings.Join(items, ", ")
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

func parseTimeValue(raw string) (time.Time, bool) {
	for _, layout := range []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatTime(raw string) string {
	if t, ok := parseTimeValue(raw); ok {
		return t.Format("Jan 2 15:04")
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
