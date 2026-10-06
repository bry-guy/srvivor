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

// buttonHints are the lines the button page flashes when a player's count reaches each threshold.
var buttonHints = []struct {
	At   int64
	Text string
}{
	{10, "The tribe has spoken"},
	{50, "Something shifted in the sand"},
	{100, "This isn't a reward challenge. Or is it?"},
	{250, "The torch burns brighter"},
	{500, "Medical has been called"},
	{1000, "You've outlasted Ozzy"},
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
