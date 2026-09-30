package httpapi

import (
	"errors"
	"time"
	_ "time/tzdata"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
)

type castawordleEpisodeOption struct {
	Number  int32
	OpensAt time.Time
}

func castawordleEpisodeWindow(episodes []db.ListInstanceEpisodesRow, number int32) (time.Time, time.Time, error) {
	var airs, next time.Time
	for _, episode := range episodes {
		if episode.EpisodeNumber == number {
			airs = episode.AirsAt.Time
		}
		if int64(episode.EpisodeNumber) == int64(number)+1 {
			next = episode.AirsAt.Time
		}
	}
	if number < 0 || airs.IsZero() || next.IsZero() {
		return time.Time{}, time.Time{}, errors.New("episode and following episode must exist in this instance's schedule")
	}
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	year, month, day := airs.In(location).Date()
	opens := time.Date(year, month, day, 13, 0, 0, 0, location)
	year, month, day = next.In(location).Date()
	cutoff := time.Date(year, month, day, 12, 0, 0, 0, location)
	if !cutoff.After(opens) {
		return time.Time{}, time.Time{}, errors.New("episode schedule does not provide a valid game window")
	}
	return opens, cutoff, nil
}
