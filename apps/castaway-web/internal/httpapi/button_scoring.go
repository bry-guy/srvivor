package httpapi

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// buttonPlayer is one player's presses in a game: how many, when they first and last pressed, and the
// Eastern calendar days they pressed on.
type buttonPlayer struct {
	ID          string
	Name, Tribe string
	Presses     int64
	First, Last time.Time
	Days        []time.Time
}

// buttonScore breaks down one player's result: place and volume (buttonAwards), their longest daily
// streak, and the first/last presser points.
type buttonScore struct {
	Presses     int64
	PlaceVolume int
	StreakDays  int
	Streak      int
	First, Last int
	Total       int
}

// buttonScores adds the streak and first/last-presser rules to buttonAwards:
//   - streak: the longest run of consecutive Eastern days with a press; 3 days +1, 4 days +2, 5+ days +3.
//   - first presser: +1 to whoever pressed first; last presser: -1 to whoever pressed last.
func buttonScores(players []buttonPlayer) map[string]buttonScore {
	presses := map[string]int64{}
	for _, p := range players {
		presses[p.ID] = p.Presses
	}
	awards := buttonAwards(presses)
	scores := map[string]buttonScore{}
	var first, last *buttonPlayer
	for i := range players {
		p := &players[i]
		days := longestStreak(p.Days)
		scores[p.ID] = buttonScore{Presses: p.Presses, PlaceVolume: awards[p.ID], StreakDays: days, Streak: min(max(days-2, 0), 3)}
		if first == nil || p.First.Before(first.First) || p.First.Equal(first.First) && p.ID < first.ID {
			first = p
		}
		if last == nil || p.Last.After(last.Last) || p.Last.Equal(last.Last) && p.ID < last.ID {
			last = p
		}
	}
	if first != nil {
		s := scores[first.ID]
		s.First = 1
		scores[first.ID] = s
		s = scores[last.ID]
		s.Last = -1
		scores[last.ID] = s
	}
	for id, s := range scores {
		s.Total = s.PlaceVolume + s.Streak + s.First + s.Last
		scores[id] = s
	}
	return scores
}

// longestStreak is the longest run of consecutive calendar days among days (any order, duplicates ok).
func longestStreak(days []time.Time) int {
	keys := map[string]bool{}
	for _, d := range days {
		keys[d.Format(time.DateOnly)] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	best, run := 0, 0
	var prev time.Time
	for _, k := range sorted {
		day, err := time.Parse(time.DateOnly, k)
		if err != nil {
			continue
		}
		if run > 0 && prev.AddDate(0, 0, 1).Equal(day) {
			run++
		} else {
			run = 1
		}
		best, prev = max(best, run), day
	}
	return best
}

// Hints flash under the button after a press. Each scoring rule has its own subtle lines, plus a few
// no-points press counts for noise. A hint fires only when the player's situation changes; when several
// apply, the first in buttonHint's order wins. They hint, they never explain.
var (
	hintFirst  = []string{"Get this trailblazer a machete"}
	hintShared = []string{"There's something in the air", "You're not alone out here", "Footprints in the sand"}
	hintMost   = []string{"The view is better from up here", "All eyes on you"}
	hintSecond = []string{"So close, and yet…"}
	hintFewest = []string{"Under the radar", "Quiet ones go far"}
	hintStreak = []string{"The tide keeps coming back", "Day after day", "Old habits"}
	hintCounts = map[int64]string{
		10: "The tribe has spoken", 50: "Something shifted in the sand", // noise
		100: "This isn't a reward challenge. Or is it?",                 // volume +1
		250: "The torch burns brighter", 500: "Medical has been called", // noise
		1000: "The island trembles", 10000: "You've outlasted Ozzy", // volume +2, +3
	}
)

// buttonHint picks the hint for a player who just pressed (players is after the press). newDay says this
// was their first press today; today is the Eastern date. pick chooses one line from a list.
func buttonHint(players []buttonPlayer, me string, newDay bool, today time.Time, pick func([]string) string) string {
	var count, total int64
	var days []time.Time
	others := map[int64]bool{}
	for _, p := range players {
		total += p.Presses
		if p.ID == me {
			count, days = p.Presses, p.Days
		} else {
			others[p.Presses] = true
		}
	}
	// place is how many distinct other counts beat c (0 = most, 1 = second); -1 for no presses or a tie
	// (a tied player doesn't hold a place on their own).
	above := func(c int64) int {
		if c == 0 || others[c] {
			return -1
		}
		n := 0
		for o := range others {
			if o > c {
				n++
			}
		}
		return n
	}
	now, before := above(count), above(count-1)
	switch {
	case count == 0:
		return ""
	case total == 1:
		return pick(hintFirst)
	case others[count]:
		return pick(hintShared)
	case now == 0 && before != 0:
		return pick(hintMost)
	case now == 1 && before != 1:
		return pick(hintSecond)
	case count == 1 && now == len(others):
		return pick(hintFewest)
	}
	if newDay {
		if run := streakEnding(days, today); run >= 3 && run <= 5 {
			return pick(hintStreak)
		}
	}
	return hintCounts[count]
}

// streakEnding is the run of consecutive days ending on today.
func streakEnding(days []time.Time, today time.Time) int {
	have := map[string]bool{}
	for _, d := range days {
		have[d.Format(time.DateOnly)] = true
	}
	run := 0
	for d := today; have[d.Format(time.DateOnly)]; d = d.AddDate(0, 0, -1) {
		run++
	}
	return run
}

type buttonQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// loadButtonPlayers reads every player who pressed in a game, with their tribe at the given time.
func loadButtonPlayers(ctx context.Context, q buttonQuerier, gameID int64, at time.Time) ([]buttonPlayer, error) {
	rows, err := q.Query(ctx, `SELECT p.public_id::text, p.name, COALESCE(t.name, ''), b.presses,
			COALESCE(b.first_press_at, g.opens_at), COALESCE(b.last_press_at, g.opens_at), b.press_days
		FROM button_presses b JOIN participants p ON p.id = b.participant_id JOIN button_games g ON g.id = b.game_id
		LEFT JOIN LATERAL (SELECT pg.name FROM participant_group_membership_periods m JOIN participant_groups pg ON pg.id = m.participant_group_id
			WHERE m.participant_id = p.id AND pg.kind = 'tribe' AND m.starts_at <= $2 AND (m.ends_at IS NULL OR m.ends_at > $2)
			ORDER BY m.starts_at DESC LIMIT 1) t ON true
		WHERE b.game_id = $1 ORDER BY b.presses DESC, p.name`, gameID, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var players []buttonPlayer
	for rows.Next() {
		var p buttonPlayer
		if err := rows.Scan(&p.ID, &p.Name, &p.Tribe, &p.Presses, &p.First, &p.Last, &p.Days); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// tribePresses totals presses per tribe, in name order (players without a tribe aren't counted).
func tribePresses(players []buttonPlayer) []buttonTribeTotal {
	totals := map[string]int64{}
	for _, p := range players {
		if p.Tribe != "" {
			totals[p.Tribe] += p.Presses
		}
	}
	out := make([]buttonTribeTotal, 0, len(totals))
	for name, n := range totals {
		out = append(out, buttonTribeTotal{Name: name, Presses: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

type buttonTribeTotal struct {
	Name    string `json:"name"`
	Presses int64  `json:"presses"`
}
