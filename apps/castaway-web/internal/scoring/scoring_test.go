package scoring

import "testing"

func TestCalculateLeaderboardSort(t *testing.T) {
	participantNames := map[string]string{"p1": "Bryan", "p2": "Amanda"}
	drafts := map[string][]DraftPick{
		"p1": {
			{Position: 1, ContestantID: "A"},
			{Position: 2, ContestantID: "B"},
			{Position: 3, ContestantID: "C"},
		},
		"p2": {
			{Position: 1, ContestantID: "B"},
			{Position: 2, ContestantID: "A"},
			{Position: 3, ContestantID: "C"},
		},
	}
	finals := map[string]int{"A": 1, "B": 2}

	leaderboard := CalculateLeaderboard(3, participantNames, drafts, finals, map[string]int{"p2": 2})
	if len(leaderboard) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(leaderboard))
	}
	if leaderboard[0].ParticipantID != "p2" {
		t.Fatalf("expected p2 first after bonus points, got %s", leaderboard[0].ParticipantID)
	}
	if leaderboard[0].DraftPoints != 3 || leaderboard[0].BonusPoints != 2 || leaderboard[0].TotalPoints != 5 || leaderboard[0].Score != 5 {
		t.Fatalf("unexpected leaderboard totals: %+v", leaderboard[0])
	}
}

func TestCalculateLeaderboardPointsAvailable(t *testing.T) {
	participantNames := map[string]string{"p1": "Bryan"}
	drafts := map[string][]DraftPick{
		"p1": {
			{Position: 1, ContestantID: "A"},
			{Position: 2, ContestantID: "B"},
			{Position: 3, ContestantID: "C"},
		},
	}
	finals := map[string]int{"A": 1}

	leaderboard := CalculateLeaderboard(3, participantNames, drafts, finals, nil)
	if leaderboard[0].Score != 3 {
		t.Fatalf("expected score 3, got %d", leaderboard[0].Score)
	}
	if leaderboard[0].DraftPoints != 3 || leaderboard[0].BonusPoints != 0 || leaderboard[0].TotalPoints != 3 {
		t.Fatalf("unexpected leaderboard points: %+v", leaderboard[0])
	}
	if leaderboard[0].PointsAvailable <= 0 {
		t.Fatalf("expected points available > 0, got %d", leaderboard[0].PointsAvailable)
	}
}
