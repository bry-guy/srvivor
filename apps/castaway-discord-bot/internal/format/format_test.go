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

	message := SingleScore(instance, row, 5, 0)
	expected := "**Season 49: Bryan Points**\nBryan: 26 points\n- Draft Points: 21\n- Bonus Points: 5\n- Secret Bonus Points: 0\n- Points Available: 46"
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestSingleScoreIncludesSecretBonusBreakdown(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 50}
	row := castaway.LeaderboardRow{ParticipantName: "Bryan", Score: 8, DraftPoints: 3, BonusPoints: 5, TotalPoints: 8, PointsAvailable: 246}

	message := SingleScore(instance, row, 4, 1)
	expected := "**Season 50: Bryan Points**\nBryan: 8 points\n- Draft Points: 3\n- Bonus Points: 4\n- Secret Bonus Points: 1\n- Points Available: 245"
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}

func TestActivitiesListFormatsCompactOutput(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 50}
	activities := []castaway.Activity{{ID: "a1", Name: "Tribal Pony", ActivityType: "tribal_pony", Status: "active"}, {ID: "a2", Name: "Journey 1", ActivityType: "journey", Status: "completed"}}

	message := ActivitiesList(instance, activities)
	expected := strings.Join([]string{
		"**Season 50: Activities**",
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
		"Instance: Season 50",
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
		Instance: castaway.Instance{ID: "i1", Name: "Season 50", Season: 50, Episodes: []castaway.InstanceEpisode{
			{ID: "e0", EpisodeNumber: 0, Label: "Episode 0", AirsAt: "2026-02-26T00:00:00Z"},
			{ID: "e1", EpisodeNumber: 1, Label: "Episode 1", AirsAt: "2026-03-05T00:00:00Z"},
			{ID: "e2", EpisodeNumber: 2, Label: "Episode 2", AirsAt: "2026-03-12T00:00:00Z"},
			{ID: "e3", EpisodeNumber: 3, Label: "Episode 3", AirsAt: "2026-03-19T00:00:00Z"},
		}},
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
				Activity: castaway.Activity{ID: "a2", Name: "Monty Hall Memorial Castaway Game", ActivityType: "manual_adjustment"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence:  castaway.Occurrence{ID: "o2", Name: "Monty Hall — Leaf Loan Shark Advantage Scroll (+1 secret bonus)", EffectiveAt: "2026-03-19T01:02:00Z"},
					Involvement: &castaway.ParticipantOccurrenceInvolvement{Role: "adjustment", Metadata: []byte(`{"award_key":"season50:monty-hall:leaf:loan-shark-secret","points":1,"reason":"Monty Hall — Leaf Loan Shark Advantage Scroll (+1 secret bonus)"}`)},
				}},
			},
			{
				Activity: castaway.Activity{ID: "a3", Name: "Tribal Pony", ActivityType: "tribal_pony"},
				Occurrences: []castaway.ParticipantActivityHistoryOccurrence{{
					Occurrence: castaway.Occurrence{ID: "o3", Name: "Episode 1 Immunity", EffectiveAt: "2026-03-05T01:00:00Z"},
					Ledger:     []castaway.BonusLedgerEntry{{ParticipantName: "Mooney", Points: 1, Visibility: "public"}},
				}},
			},
		},
	}

	message := ParticipantHistory(history)
	for _, fragment := range []string{
		"**Season 50: Mooney History**",
		"**Episode 0**",
		"n/a",
		"**Episode 1**",
		"Tribal Pony",
		"- Episode 1 Immunity @ Mar 5 01:00",
		"- impact: +1 public",
		"**Episode 2**",
		"Journey 1",
		"- Lost for Words — Mooney @ Mar 14 02:00",
		"- action: delegate, group=Leaf",
		"- result: risked",
		"- impact: +1 secret",
		"**Episode 3**",
		"Monty Hall Memorial Castaway Game",
		"- Monty Hall — Leaf Loan Shark Advantage Scroll (+1 secret bonus) @ Mar 19 01:02",
		"- action: adjustment",
		"- result: Monty Hall — Leaf Loan Shark Advantage Scroll (+1 secret bonus)",
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("expected fragment %q in %q", fragment, message)
		}
	}
	if strings.Contains(message, "award_key=") {
		t.Fatalf("expected manual adjustment metadata to be humanized, got %q", message)
	}
}

func TestLeaderboardIncludesTotalDraftAndBonus(t *testing.T) {
	instance := castaway.Instance{Name: "Office Pool", Season: 49}
	rows := []castaway.LeaderboardRow{{ParticipantName: "Bryan", Score: 26, DraftPoints: 21, BonusPoints: 5, TotalPoints: 26, PointsAvailable: 46}, {ParticipantName: "Riley", Score: 19, DraftPoints: 19, BonusPoints: 0, TotalPoints: 19, PointsAvailable: 41}}

	message := Leaderboard(instance, rows)
	expected := strings.Join([]string{
		"**Season 49: Leaderboard**",
		"1. Bryan — 26 (21+5)",
		"2. Riley — 19 (19+0)",
	}, "\n")
	if message != expected {
		t.Fatalf("unexpected message:\nexpected: %q\nactual:   %q", expected, message)
	}
}
