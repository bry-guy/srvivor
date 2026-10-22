package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// runTrackSet runs `track set` against a fake API and returns the output, the PUT bodies, and the error.
func runTrackSet(t *testing.T, yes bool, args ...string) (string, []string, error) {
	t.Helper()
	const plays = `{"game_id":"g","open":true,"resolved":false,"plays":[{"participant_id":"p1","name":"Adam","cups":1,"balls_used":4,"finished":false,"score":1,"place":1,"points":1}]}`
	var puts []string
	fake := func(_ context.Context, method, path string, body, out any) error {
		switch {
		case method == "GET" && strings.HasSuffix(path, "/participants"):
			return json.Unmarshal([]byte(`{"participants":[{"id":"p1","name":"Adam"}]}`), out)
		case method == "GET":
			return json.Unmarshal([]byte(plays), out)
		case method == "PUT":
			data, err := json.Marshal(body)
			if err != nil {
				return err
			}
			puts = append(puts, path+" "+string(data))
			return json.Unmarshal([]byte(`{"score":3}`), out)
		}
		t.Fatalf("unexpected %s %s", method, path)
		return nil
	}
	inst := "inst"
	cmd := newTrackCommand(fake, &inst, &yes)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{"set"}, args...))
	err := cmd.Execute()
	return buf.String(), puts, err
}

func TestTrackSetFlagsBecomeBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string // PUT path suffix and body
	}{
		{"cups keeps balls used", []string{"Adam", "--episode", "5", "--cups", "3"}, `/track-games/5/plays/p1 {"balls_used":4,"cups_mask":7,"finished":false}`},
		{"cups sets lowest bits", []string{"Adam", "--episode", "5", "--cups", "2", "--balls-used", "6"}, `/track-games/5/plays/p1 {"balls_used":6,"cups_mask":3,"finished":false}`},
		{"mask overrides cups", []string{"Adam", "--episode", "5", "--cups", "1", "--mask", "5"}, `/track-games/5/plays/p1 {"balls_used":4,"cups_mask":5,"finished":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, puts, err := runTrackSet(t, true, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if len(puts) != 1 || !strings.HasSuffix(puts[0], tc.want) {
				// the path is /instances/inst/...; compare the tail only
				t.Fatalf("PUT %v, want ...%s", puts, tc.want)
			}
		})
	}
}

func TestTrackSetDryRunAndRefusals(t *testing.T) {
	out, puts, err := runTrackSet(t, false, "Adam", "--episode", "5", "--cups", "3")
	if err != nil || len(puts) != 0 {
		t.Fatalf("dry run wrote %v (err %v)", puts, err)
	}
	if !strings.Contains(out, "cups 1 → 3, balls used 4 → 4, score 1 → 11") {
		t.Fatalf("dry run output: %s", out)
	}
	if _, puts, err := runTrackSet(t, true, "Adam", "--episode", "5", "--cups", "3", "--balls-used", "2"); err == nil || len(puts) != 0 {
		t.Fatalf("3 cups with 2 balls: err %v, PUT %v", err, puts)
	}
	if _, _, err := runTrackSet(t, true, "Adam", "--episode", "5"); err == nil {
		t.Fatal("expected --cups or --mask to be required")
	}
}
