package format

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/messages"
)

const DiscordMessageLimit = 2000

// occurrenceView is one occurrence in the occurrences list and activity detail templates.
type occurrenceView struct {
	Name, Type, Status, Date string
	Impacts                  []string
}

// historyGroupView and historyItemView feed the participant history template.
type historyGroupView struct {
	Label string
	Items []historyItemView
}

type historyItemView struct {
	Activity string
	Details  []string
}

func InstanceLabel(instance castaway.Instance) string {
	name := strings.TrimSpace(instance.Name)
	if strings.EqualFold(name, fmt.Sprintf("Season %d", instance.Season)) {
		name = ""
	}
	return messages.Fragment("instance-label", map[string]any{"Season": instance.Season, "Name": name})
}

func SingleScore(instance castaway.Instance, row castaway.LeaderboardRow, rank int) string {
	return TrimMessage(messages.Fragment("score.md", map[string]any{"Season": instance.Season, "Line": leaderboardLine(rank, row)}))
}

func PrivateScore(instance castaway.Instance, row castaway.LeaderboardRow, rank int, visibleBonusPoints, secretBonusPoints int) string {
	prefix := ""
	if badge := tribeBadge(row.CurrentTribeName); badge != "" {
		prefix = badge + " "
	}
	displayName := strings.TrimSpace(row.ParticipantName)
	if discordUserID := strings.TrimSpace(row.ParticipantDiscordUserID); discordUserID != "" {
		displayName = "<@" + discordUserID + ">"
	}
	return TrimMessage(messages.Fragment("score-private.md", map[string]any{
		"Season": instance.Season, "Rank": rank, "Prefix": prefix, "Name": displayName,
		"Total": row.Total(), "Draft": row.Draft(), "Bonus": visibleBonusPoints, "Secret": secretBonusPoints,
	}))
}

func Leaderboard(instance castaway.Instance, rows []castaway.LeaderboardRow) string {
	lines := make([]string, 0, len(rows))
	for index, row := range rows {
		lines = append(lines, leaderboardLine(index+1, row))
	}
	return TrimMessage(messages.Render("leaderboard.md", map[string]any{"Season": instance.Season, "Lines": lines}))
}

func leaderboardLine(rank int, row castaway.LeaderboardRow) string {
	prefix := ""
	if badge := tribeBadge(row.CurrentTribeName); badge != "" {
		prefix = badge + " "
	}
	displayName := strings.TrimSpace(row.ParticipantName)
	if discordUserID := strings.TrimSpace(row.ParticipantDiscordUserID); discordUserID != "" {
		displayName = "<@" + discordUserID + ">"
	}
	return messages.Fragment("leaderboard-line", map[string]any{
		"Rank": rank, "Prefix": prefix, "Name": displayName, "Total": row.Total(), "Draft": row.Draft(), "Bonus": row.Bonus(),
	})
}

func tribeBadge(name string) string {
	normalized := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(name)))
	switch normalized {
	case "lotus":
		return ":lotus:"
	case "tangerine":
		return ":tangerine:"
	case "leaf", "leafygreen":
		return ":leafy_green:"
	default:
		return ""
	}
}

func Draft(instance castaway.Instance, draft castaway.Draft) string {
	return TrimMessage(messages.Render("draft.md", map[string]any{
		"Season": instance.Season, "Player": draft.Participant.Name, "Picks": draft.Picks,
	}))
}

func InstanceList(instances []castaway.Instance) string {
	if len(instances) == 0 {
		return messages.Fragment("no-instances", nil)
	}
	labels := make([]string, 0, len(instances))
	for _, instance := range instances {
		labels = append(labels, InstanceLabel(instance))
	}
	return TrimMessage(messages.Render("instances.md", map[string]any{"Labels": labels}))
}

func ActivitiesList(instance castaway.Instance, activities []castaway.Activity) string {
	if len(activities) == 0 {
		return messages.Fragment("activities-empty", map[string]any{"Instance": InstanceLabel(instance)})
	}
	return TrimMessage(messages.Render("activities.md", map[string]any{"Season": instance.Season, "Activities": activities}))
}

func ActivityDetail(detail castaway.ActivityDetail, occurrences []castaway.Occurrence, instance castaway.Instance) string {
	activity := detail.Activity
	data := map[string]any{
		"Season": instance.Season, "Name": activity.Name, "Type": activity.ActivityType, "Status": activity.Status,
		"Starts": "", "Ends": "", "Assignments": activityAssignmentLines(detail), "Occurrences": occurrenceViews(occurrences),
	}
	if when := strings.TrimSpace(activity.StartsAt); when != "" {
		data["Starts"] = formatTimeLong(when)
	}
	if when := strings.TrimSpace(activity.EndsAt); when != "" {
		data["Ends"] = formatTimeLong(when)
	}
	return TrimMessage(messages.Render("activity-detail.md", data))
}

func occurrenceViews(occurrences []castaway.Occurrence) []occurrenceView {
	views := make([]occurrenceView, 0, len(occurrences))
	for _, occurrence := range occurrences {
		view := occurrenceView{Name: occurrence.Name, Type: occurrence.OccurrenceType, Status: occurrence.Status}
		if when := strings.TrimSpace(occurrence.EffectiveAt); when != "" {
			view.Date = formatTime(when)
		}
		views = append(views, view)
	}
	return views
}

func activityAssignmentLines(detail castaway.ActivityDetail) []string {
	lines := make([]string, 0, len(detail.GroupAssignments)+len(detail.ParticipantAssignments))
	for _, assignment := range detail.GroupAssignments {
		line := assignment.ParticipantGroupName
		if strings.TrimSpace(assignment.Role) != "" {
			line += " — " + kv("role", strings.TrimSpace(assignment.Role))
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
			parts = append(parts, kv("role", strings.TrimSpace(assignment.Role)))
		}
		if strings.TrimSpace(assignment.ParticipantGroupName) != "" {
			parts = append(parts, kv("group", strings.TrimSpace(assignment.ParticipantGroupName)))
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
		return messages.Fragment("occurrences-empty", map[string]any{"Activity": activity.Name})
	}
	views := make([]occurrenceView, 0, len(details))
	for _, detail := range details {
		view := occurrenceView{Name: detail.Occurrence.Name, Status: detail.Occurrence.Status, Impacts: ledgerLines(detail.Ledger)}
		if when := strings.TrimSpace(detail.Occurrence.EffectiveAt); when != "" {
			view.Date = formatTime(when)
		}
		views = append(views, view)
	}
	return TrimMessage(messages.Render("occurrences.md", map[string]any{"Activity": activity.Name, "Occurrences": views}))
}

func OccurrenceDetail(detail castaway.OccurrenceDetail, activity castaway.Activity) string {
	return TrimMessage(messages.Render("occurrence-detail.md", map[string]any{
		"Name": detail.Occurrence.Name, "Activity": activity.Name, "Type": detail.Occurrence.OccurrenceType,
		"Status": detail.Occurrence.Status, "Date": formatTimeLong(detail.Occurrence.EffectiveAt),
		"Recorded": recordedLines(detail), "Impact": ledgerLines(detail.Ledger),
	}))
}

func ParticipantHistory(history castaway.ParticipantActivityHistory) string {
	currentEpisodeNumber := int32(-1)
	data := map[string]any{"Season": history.Instance.Season, "Name": history.Participant.Name, "HasCurrent": false, "Current": ""}
	if history.Instance.CurrentEpisode != nil {
		currentEpisodeNumber = history.Instance.CurrentEpisode.EpisodeNumber
		data["HasCurrent"] = true
		data["Current"] = strings.TrimSpace(history.Instance.CurrentEpisode.Label)
	}

	episodeGroups := groupHistoryByEpisode(history)
	data["Empty"] = len(episodeGroups) == 0
	groups := make([]historyGroupView, 0, len(episodeGroups))
	for _, group := range episodeGroups {
		if currentEpisodeNumber >= 0 && group.episodeNumber > currentEpisodeNumber {
			break
		}
		view := historyGroupView{Label: group.label}
		for _, item := range group.items {
			view.Items = append(view.Items, historyItemView{Activity: item.activityName, Details: item.details})
		}
		groups = append(groups, view)
	}
	data["Groups"] = groups
	return TrimMessage(messages.Render("history.md", data))
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
			label = episodeLabel(episode.EpisodeNumber)
		}
		groups = append(groups, historyEpisodeGroup{episodeNumber: episode.EpisodeNumber, label: label})
		groupIndex[episode.EpisodeNumber] = len(groups) - 1
	}
	if len(groups) == 0 {
		groups = append(groups, historyEpisodeGroup{episodeNumber: 0, label: messages.Fragment("history-label", nil)})
		groupIndex[0] = 0
	}

	for _, activity := range history.Activities {
		for _, occurrence := range activity.Occurrences {
			episodeNumber := historyEpisodeNumberForOccurrence(history.Instance.Episodes, occurrence.Occurrence.EffectiveAt)
			index, ok := groupIndex[episodeNumber]
			if !ok {
				label := episodeLabel(episodeNumber)
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
		label = messages.Fragment("recorded-event", nil)
	}
	if when := strings.TrimSpace(item.Occurrence.EffectiveAt); when != "" {
		label += " @ " + formatTime(when)
	}
	lines = append(lines, label)
	if action := historyActionSummary(item.Involvement); action != "" {
		lines = append(lines, labeled("action", action))
	}
	if result := historyResultSummary(item.Involvement); result != "" {
		lines = append(lines, labeled("result", result))
	}
	if impact := historyImpactSummary(item.Ledger); impact != "" {
		lines = append(lines, labeled("impact", impact))
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
		parts = append(parts, kv("group", groupName))
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
		line += " — " + kv("role", group.Role)
	}
	if strings.TrimSpace(group.Result) != "" {
		line += ", " + kv("result", strings.TrimSpace(group.Result))
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
			lines = append(lines, messages.Fragment("ledger-each", map[string]any{"Key": key, "Count": count}))
		} else {
			lines = append(lines, key)
		}
	}
	return lines
}

func formatResultLine(role, result, groupName string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(role) != "" {
		parts = append(parts, kv("role", strings.TrimSpace(role)))
	}
	if strings.TrimSpace(result) != "" {
		parts = append(parts, kv("result", strings.TrimSpace(result)))
	}
	if strings.TrimSpace(groupName) != "" {
		parts = append(parts, kv("group", strings.TrimSpace(groupName)))
	}
	return strings.Join(parts, ", ")
}

// The helpers below keep the label wording in the messages templates.
func kv(key, value string) string {
	return messages.Fragment("kv", map[string]any{"Key": key, "Value": value})
}

func labeled(label, value string) string {
	return messages.Fragment("labeled", map[string]any{"Label": label, "Value": value})
}

func episodeLabel(number int32) string {
	return messages.Fragment("episode-label", number)
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
		parts = append(parts, kv(key, fmt.Sprintf("%v", payload[key])))
		if len(parts) == 3 {
			break
		}
	}
	if len(keys) > len(parts) {
		parts = append(parts, messages.Fragment("more-count", map[string]any{"N": len(keys) - len(parts)}))
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
	suffix := messages.Fragment("truncated-suffix", nil)
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
