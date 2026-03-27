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
	row := castaway.LeaderboardRow{ParticipantName: "Bryan", Score: 26, DraftPoints: 21, BonusPoints: 5, TotalPoints: 26, PointsAvailable: 46}

	message := SingleScore(instance, row)
	expected := "**Season 49 — Office Pool**\nBryan — 26 points (21+5; points available: 46)"
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestActivitiesListFormatsCompactOutput(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 50}
	activities := []castaway.Activity{{ID: "a1", Name: "Tribal Pony", ActivityType: "tribal_pony", Status: "active"}, {ID: "a2", Name: "Journey 1", ActivityType: "journey", Status: "completed"}}

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

func TestActivityDetailFormatsAssignmentsAndOccurrences(t *testing.T) {
	instance := castaway.Instance{ID: "i1", Name: "Season 50", Season: 50}
	detail := castaway.ActivityDetail{
		Activity:               castaway.Activity{ID: "a1", Name: "Journey 1", ActivityType: "journey", Status: "completed", StartsAt: "2026-03-12T00:00:00Z"},
		GroupAssignments:       []castaway.ActivityGroupAssignment{{ParticipantGroupName: "Leaf", Role: "tribe"}},
		ParticipantAssignments: []castaway.ActivityParticipantAssignment{{ParticipantName: "Mooney", ParticipantGroupName: "Leaf", Role: "delegate"}},
	}
	occurrences := []castaway.Occurrence{{ID: "o1", Name: "Journey 1 Attendance", OccurrenceType: "attendance", Status: "resolved", EffectiveAt: "2026-03-12T00:00:00Z"}}

	message := ActivityDetail(detail, occurrences, instance)
	for _, fragment := range []string{
		"**Journey 1**",
		"Instance: Season 50 — Season 50",
		"**Assignments**",
		"Leaf — role=tribe",
		"Mooney — role=delegate, group=Leaf",
		"**Occurrences**",
		"Journey 1 Attendance",
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestOccurrencesListFormatsDetailedCompactOutput(t *testing.T) {
	activity := castaway.Activity{ID: "a1", Name: "Tribal Pony"}
	details := []castaway.OccurrenceDetail{
		{
			Occurrence: castaway.Occurrence{ID: "o1", Name: "Ep 1 Immunity", OccurrenceType: "immunity_result", Status: "resolved", EffectiveAt: "2026-03-05T01:00:00Z"},
			Ledger: []castaway.BonusLedgerEntry{
				{ParticipantName: "Amanda", Points: 1, Visibility: "public"},
				{ParticipantName: "Bryan", Points: 1, Visibility: "public"},
			},
		},
	}

	message := OccurrencesList(activity, details)
	expected := strings.Join([]string{
		"**Tribal Pony — Occurrences**",
		"- **Ep 1 Immunity** (immunity_result) — resolved @ Mar 5 01:00 · impact: Amanda — +1 public; Bryan — +1 public",
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

func TestOccurrenceDetailFormatsRecordedAndImpactSections(t *testing.T) {
	activity := castaway.Activity{ID: "a1", Name: "Journey 1"}
	detail := castaway.OccurrenceDetail{
		Occurrence: castaway.Occurrence{ID: "o1", Name: "Journey 1 Tribal Diplomacy", OccurrenceType: "journey_resolution", Status: "resolved", EffectiveAt: "2026-03-14T01:00:00Z"},
		Participants: []castaway.OccurrenceParticipant{
			{ParticipantName: "Adam", ParticipantGroupName: "Tangerine", Role: "delegate", Result: "STEAL"},
			{ParticipantName: "Katie", ParticipantGroupName: "Lotus", Role: "delegate", Result: "SHARE"},
		},
		Ledger: []castaway.BonusLedgerEntry{{ParticipantName: "Katie", Points: 1, Visibility: "public"}},
	}

	message := OccurrenceDetail(detail, activity)
	for _, fragment := range []string{
		"**Journey 1 Tribal Diplomacy**",
		"Activity: Journey 1",
		"**Recorded**",
		"Adam — role=delegate, result=STEAL, group=Tangerine",
		"**Impact**",
		"Katie — +1 public",
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestParticipantHistoryFormatsGroupedEntries(t *testing.T) {
	history := castaway.ParticipantActivityHistory{
		Participant: castaway.Participant{ID: "p1", Name: "Mooney"},
		Instance:    castaway.Instance{ID: "i1", Name: "Season 50", Season: 50},
		Activities: []castaway.ParticipantActivityHistoryActivity{
			{
				Activity: castaway.Activity{ID: "a1", Name: "Journey 1", ActivityType: "journey"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence:  castaway.Occurrence{ID: "o1", Name: "Lost for Words — Mooney", EffectiveAt: "2026-03-14T02:00:00Z"},
					Involvement: &castaway.ParticipantOccurrenceInvolvement{Role: "delegate", Result: "risked", ParticipantGroupName: "Leaf"},
					Ledger:      []castaway.BonusLedgerEntry{{ParticipantName: "Mooney", Points: 1, Visibility: "secret"}},
				}},
			},
			{
				Activity: castaway.Activity{ID: "a2", Name: "Tribal Pony", ActivityType: "tribal_pony"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence: castaway.Occurrence{ID: "o2", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z"},
					Ledger:     []castaway.BonusLedgerEntry{{ParticipantName: "Mooney", Points: 1, Visibility: "public"}},
				}},
			},
		},
	}

	message := ParticipantHistory(history)
	for _, fragment := range []string{
		"**Mooney — Activity History**",
		"**Journey 1** (journey)",
		"Lost for Words — Mooney @ Mar 14 02:00",
		"role=delegate, result=risked, group=Leaf",
		"impact: Mooney — +1 secret",
		"**Tribal Pony** (tribal_pony)",
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
}

func TestLeaderboardIncludesTotalDraftAndBonus(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 49}
	rows := []castaway.LeaderboardRow{{ParticipantName: "Bryan", Score: 26, DraftPoints: 21, BonusPoints: 5, TotalPoints: 26, PointsAvailable: 46}, {ParticipantName: "Riley", Score: 19, DraftPoints: 19, BonusPoints: 0, TotalPoints: 19, PointsAvailable: 41}}

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
