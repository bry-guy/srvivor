package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// `season merge FILE N` sets merge_episode, opens Pick Your Champion (merge episode airs → next episode airs),
// and syncs; the dry run changes nothing, and a second merge is refused.
func TestSeasonMerge(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "s.yaml")
	for src, dst := range map[string]string{"testdata/season-brainland.yaml": file, "testdata/season-brainland-scores.md": filepath.Join(dir, "s-scores.md")} {
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var rounds []map[string]any
	puts := 0
	call := func(_ context.Context, method, path string, body, out any) error {
		write := func(v any) error {
			if out == nil {
				return nil
			}
			b, _ := json.Marshal(v)
			return json.Unmarshal(b, out)
		}
		switch {
		case strings.HasSuffix(path, "/champion-round"):
			rounds = append(rounds, body.(map[string]any))
			return nil
		case strings.HasSuffix(path, "/announcements"):
			return write(map[string]any{"announcements": []any{}})
		case strings.HasSuffix(path, "/season-config") && method == "GET":
			return fmt.Errorf("HTTP 404")
		case strings.HasSuffix(path, "/season-config"):
			puts++
			return write(map[string]string{"revision": "r1"})
		case method == "GET": // episodes: weekly from the fixture's first air time
			var eps []episode
			at := time.Date(2026, 9, 23, 20, 0, 0, 0, eastern())
			for n := 1; n <= 13; n++ {
				eps = append(eps, episode{Number: n, AirsAt: at.AddDate(0, 0, 7*(n-1))})
			}
			return write(map[string]any{"episodes": eps})
		}
		return fmt.Errorf("unexpected %s %s", method, path)
	}
	run := func(yes bool, args ...string) (string, error) {
		instance, guild := "", ""
		cmd := newSeasonCommand(call, &instance, &guild, &yes)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		cmd.SetContext(context.Background())
		err := cmd.Execute()
		return out.String(), err
	}
	before, _ := os.ReadFile(file)
	if out, err := run(false, "merge", file, "7"); err != nil || len(rounds) != 0 || puts != 0 || !strings.Contains(out, "lock Wed Nov 11 8:00pm") {
		t.Fatalf("dry run: %v %q", err, out)
	}
	if after, _ := os.ReadFile(file); !bytes.Equal(before, after) {
		t.Fatal("dry run edited the file")
	}
	if _, err := run(true, "merge", file, "13"); err == nil {
		t.Fatal("merge at the finale has no next episode to lock at")
	}
	if _, err := run(true, "merge", file, "7"); err != nil || len(rounds) != 1 || puts != 1 {
		t.Fatalf("merge: %v rounds=%v puts=%d", err, rounds, puts)
	}
	if rounds[0]["merge_episode"] != 7 || rounds[0]["opens_at"] != "2026-11-05T01:00:00Z" || rounds[0]["locks_at"] != "2026-11-12T01:00:00Z" {
		t.Fatalf("round = %v", rounds[0])
	}
	if f, err := loadSeasonFile(file); err != nil || f.Scoring.MergeEpisode != 7 {
		t.Fatalf("file merge_episode = %v (%v)", f.Scoring.MergeEpisode, err)
	}
	if _, err := run(true, "merge", file, "8"); err == nil || !strings.Contains(err.Error(), "already set") {
		t.Fatalf("second merge: %v", err)
	}
}
