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

func TestButtonHint(t *testing.T) {
	first := func(lines []string) string { return lines[0] }
	p := func(id string, n int64, ds ...int) buttonPlayer {
		return buttonPlayer{ID: id, Presses: n, Days: days(ds...)}
	}
	for name, tc := range map[string]struct {
		players []buttonPlayer
		newDay  bool
		want    string
	}{
		"very first press":           {[]buttonPlayer{p("me", 1, 7)}, true, "Get this trailblazer a machete"},
		"landed on a shared one":     {[]buttonPlayer{p("me", 5, 7), p("b", 5, 7), p("c", 9, 7)}, false, "There's something in the air"},
		"took the top":               {[]buttonPlayer{p("me", 10, 7), p("b", 9, 7)}, false, "The view is better from up here"},
		"still on top: quiet":        {[]buttonPlayer{p("me", 12, 7), p("b", 9, 7)}, false, ""},
		"moved into second":          {[]buttonPlayer{p("me", 6, 7), p("b", 5, 7), p("c", 9, 7)}, false, "So close, and yet…"},
		"first press, fewest":        {[]buttonPlayer{p("me", 1, 7), p("b", 5, 7), p("c", 9, 7)}, true, "Under the radar"},
		"streak day 3":               {[]buttonPlayer{p("me", 7, 7, 8, 9), p("b", 2, 7), p("c", 99, 7)}, true, "The tide keeps coming back"},
		"same day, no streak":        {[]buttonPlayer{p("me", 7, 7, 8, 9), p("b", 2, 7), p("c", 99, 7)}, false, ""},
		"streak day 6: quiet":        {[]buttonPlayer{p("me", 7, 4, 5, 6, 7, 8, 9), p("b", 2, 7), p("c", 99, 7)}, true, ""},
		"noise count":                {[]buttonPlayer{p("me", 50, 9), p("b", 2, 9), p("c", 99, 9)}, false, "Something shifted in the sand"},
		"volume 10,000":              {[]buttonPlayer{p("me", 10000, 9), p("b", 2, 9), p("c", 99999, 9)}, false, "You've outlasted Ozzy"},
		"shared beats a noise count": {[]buttonPlayer{p("me", 50, 9), p("b", 50, 9), p("c", 99, 9)}, false, "There's something in the air"},
	} {
		if got := buttonHint(tc.players, "me", tc.newDay, day(9), first); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
