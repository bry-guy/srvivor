package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCastawordleEpisodeZeroView(t *testing.T) {
	view := castawordleGame(db.GetCastawordleGameRow{EpisodeNumber: pgtype.Int4{Int32: 0, Valid: true}})
	body, err := json.Marshal(view)
	if err != nil || view.Test || !strings.Contains(string(body), `"episode_number":0`) {
		t.Fatalf("scheduled preseason was confused with an admin test: %s, %v", body, err)
	}
}

func TestCastawordleEpisodeWindow(t *testing.T) {
	episodes := []db.ListInstanceEpisodesRow{
		{EpisodeNumber: 1, AirsAt: pgtype.Timestamptz{Time: time.Date(2026, 10, 29, 2, 30, 0, 0, time.UTC), Valid: true}},
		{EpisodeNumber: 2, AirsAt: pgtype.Timestamptz{Time: time.Date(2026, 11, 5, 4, 0, 0, 0, time.UTC), Valid: true}},
	}
	opens, cutoff, err := castawordleEpisodeWindow(episodes, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !opens.Equal(time.Date(2026, 10, 28, 17, 0, 0, 0, time.UTC)) || !cutoff.Equal(time.Date(2026, 11, 4, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 1pm EDT through noon EST despite changed airtimes: %s → %s", opens, cutoff)
	}
	for _, number := range []int32{-1, 2, 3, 2147483647} {
		if _, _, err := castawordleEpisodeWindow(episodes, number); err == nil {
			t.Fatalf("accepted missing or invalid episode %d", number)
		}
	}
	episodes[1].AirsAt = episodes[0].AirsAt
	if _, _, err := castawordleEpisodeWindow(episodes, 1); err == nil {
		t.Fatal("accepted a nonpositive game window")
	}
}
