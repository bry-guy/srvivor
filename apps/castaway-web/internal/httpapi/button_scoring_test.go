package httpapi

import (
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }

func days(ds ...int) []time.Time {
	out := make([]time.Time, len(ds))
	for i, d := range ds {
		out[i] = day(d)
	}
	return out
}

func TestLongestStreak(t *testing.T) {
	for want, ds := range map[int][]int{0: {}, 1: {7}, 2: {7, 8, 10}, 3: {10, 8, 9, 9}, 5: {7, 8, 9, 10, 11, 13}, 8: {7, 8, 9, 10, 11, 12, 13, 14}} {
		if got := longestStreak(days(ds...)); got != want {
			t.Errorf("longestStreak(%v) = %d, want %d", ds, got, want)
		}
	}
	// Month boundaries are consecutive days.
	if got := longestStreak([]time.Time{time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}); got != 3 {
		t.Errorf("across a month: %d", got)
	}
}

func TestButtonScores(t *testing.T) {
	scores := buttonScores([]buttonPlayer{
		// a: most presses (+2) and 1,000+ (+2), 5-day streak (+3), first presser (+1) = +8
		{ID: "a", Presses: 1500, First: day(7), Last: day(12), Days: days(7, 8, 9, 10, 11)},
		// b: second most (-1) +1 for 100+, 3-day streak (+1), last presser (-1) = 0
		{ID: "b", Presses: 120, First: day(8), Last: day(14), Days: days(8, 9, 10, 14)},
		// c: least (+1), 4-day streak (+2) = +3
		{ID: "c", Presses: 4, First: day(9), Last: day(13), Days: days(10, 11, 12, 13)},
	})
	want := map[string]buttonScore{
		"a": {Presses: 1500, PlaceVolume: 4, StreakDays: 5, Streak: 3, First: 1, Total: 8},
		"b": {Presses: 120, PlaceVolume: 0, StreakDays: 3, Streak: 1, Last: -1, Total: 0},
		"c": {Presses: 4, PlaceVolume: 1, StreakDays: 4, Streak: 2, Total: 3},
	}
	for id, w := range want {
		if scores[id] != w {
			t.Errorf("%s: got %+v, want %+v", id, scores[id], w)
		}
	}
	// One player is both first and last presser: +1 -1.
	solo := buttonScores([]buttonPlayer{{ID: "x", Presses: 1, First: day(7), Last: day(7), Days: days(7)}})["x"]
	if solo.First != 1 || solo.Last != -1 || solo.Total != 2 { // alone: most presses +2
		t.Errorf("solo: %+v", solo)
	}
	if len(buttonScores(nil)) != 0 {
		t.Error("nobody pressed should score nobody")
	}
}
