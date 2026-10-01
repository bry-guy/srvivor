package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	weekTestInstance = "123e4567-e89b-12d3-a456-426614174000"
	weekTestGame     = "123e4567-e89b-12d3-a456-426614174001"
	weekTestAnswer   = "SECRET"
)

type weekTestCall struct {
	method string
	path   string
	body   string
}

type weekTestAPI struct {
	calls     []weekTestCall
	gameReply any
	gameErr   error
}

func (f *weekTestAPI) call(_ context.Context, method, path string, body, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	f.calls = append(f.calls, weekTestCall{method: method, path: path, body: string(encoded)})
	switch {
	case method == "GET" && path == "/instances/"+weekTestInstance:
		return nil
	case method == "POST" && path == "/instances/"+weekTestInstance+"/castawordle":
		if f.gameErr != nil {
			return f.gameErr
		}
		if out != nil && f.gameReply != nil {
			data, err := json.Marshal(f.gameReply)
			if err != nil {
				return err
			}
			return json.Unmarshal(data, out)
		}
		return nil
	case method == "POST" && path == "/castawordle/"+weekTestGame+"/resolve":
		return nil
	default:
		return fmt.Errorf("unexpected API call: %s %s", method, path)
	}
}

func writeWeekFixture(t *testing.T, answer string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "answer.txt"), []byte(answer), 0600); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"version":1,"instance_id":%q,"episode":2,"puzzle":{"name":"Episode 2","answer_file":"answer.txt","opens_at":"2026-09-30T20:00:00-04:00","cutoff_at":"2026-10-07T12:00:00-04:00"}}`, weekTestInstance)
	file := filepath.Join(dir, "week.json")
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func verifiedWeekGame() map[string]any {
	return map[string]any{
		"id": weekTestGame, "instance_id": weekTestInstance, "name": "Episode 2",
		"opens_at": "2026-10-01T00:00:00Z", "cutoff_at": "2026-10-07T16:00:00Z",
		"unscored": false, "episode_number": 2,
	}
}

func runWeekCommand(t *testing.T, call apiCall, file string, instance string, yes bool, args ...string) (string, error) {
	t.Helper()
	command := newWeekCommand(call, &instance, &yes)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs(args)
	if file != "" && len(args) == 0 {
		command.SetArgs([]string{"--file", file})
	} else if file != "" && args[0] == "--file" {
		command.SetArgs(append(args, file))
	}
	err := command.Execute()
	return output.String(), err
}

func TestWeekDryRunReadsOnlyAndHidesAnswer(t *testing.T) {
	api := &weekTestAPI{}
	file := writeWeekFixture(t, weekTestAnswer+"\n")
	out, err := runWeekCommand(t, api.call, file, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.calls) != 1 || api.calls[0].method != "GET" || api.calls[0].path != "/instances/"+weekTestInstance {
		t.Fatalf("dry run calls = %#v", api.calls)
	}
	if strings.Contains(out, weekTestAnswer) || !strings.Contains(out, "No writes") {
		t.Fatalf("unsafe or unexpected dry-run output: %q", out)
	}
}

func TestWeekRejectsLocalErrorsBeforeAPI(t *testing.T) {
	for _, tc := range []struct {
		name     string
		answer   string
		instance string
	}{
		{name: "answer format", answer: "secret\n"},
		{name: "instance mismatch", answer: weekTestAnswer, instance: "123e4567-e89b-12d3-a456-426614174002"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &weekTestAPI{}
			file := writeWeekFixture(t, tc.answer)
			_, err := runWeekCommand(t, api.call, file, tc.instance, true)
			if err == nil || len(api.calls) != 0 {
				t.Fatalf("expected local rejection before API calls, err=%v calls=%#v", err, api.calls)
			}
		})
	}
}

func TestWeekApplyUsesStableScoredPayloadAndVerifiesResponse(t *testing.T) {
	api := &weekTestAPI{gameReply: verifiedWeekGame()}
	file := writeWeekFixture(t, weekTestAnswer)
	for i := 0; i < 2; i++ {
		out, err := runWeekCommand(t, api.call, file, "", true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, weekTestAnswer) || !strings.Contains(out, weekTestGame) {
			t.Fatalf("unsafe or missing apply output: %q", out)
		}
	}
	var posts []weekTestCall
	for _, call := range api.calls {
		if call.method == "POST" {
			posts = append(posts, call)
		}
	}
	if len(posts) != 2 || posts[0].body != posts[1].body {
		t.Fatalf("expected stable payload on idempotent reapply, got %#v", posts)
	}
	var body struct {
		Answer  string            `json:"answer"`
		Episode int               `json:"episode_number"`
		Scored  bool              `json:"scored"`
		Window  map[string]string `json:"window"`
	}
	if err := json.Unmarshal([]byte(posts[0].body), &body); err != nil {
		t.Fatal(err)
	}
	if body.Answer != weekTestAnswer || body.Episode != 2 || !body.Scored || body.Window["opens_at"] != "2026-10-01T00:00:00Z" || body.Window["cutoff_at"] != "2026-10-07T16:00:00Z" {
		t.Fatalf("unexpected scored payload: %#v", body)
	}
}

func TestWeekApplyHidesAnswerOnAPIError(t *testing.T) {
	api := &weekTestAPI{gameErr: errors.New("bad answer SECRET")}
	file := writeWeekFixture(t, weekTestAnswer)
	out, err := runWeekCommand(t, api.call, file, "", true)
	if err == nil || strings.Contains(err.Error(), weekTestAnswer) || strings.Contains(out, weekTestAnswer) {
		t.Fatalf("answer leaked on API error: out=%q err=%v", out, err)
	}
}

func TestWeekApplyRejectsMissingOrUnscoredConfirmation(t *testing.T) {
	unscored := true
	for _, reply := range []any{
		map[string]any{},
		map[string]any{"id": weekTestGame, "instance_id": weekTestInstance, "name": "Episode 2", "opens_at": "2026-10-01T00:00:00Z", "cutoff_at": "2026-10-07T16:00:00Z", "unscored": unscored, "episode_number": 2},
	} {
		api := &weekTestAPI{gameReply: reply}
		file := writeWeekFixture(t, weekTestAnswer)
		out, err := runWeekCommand(t, api.call, file, "", true)
		if err == nil || strings.Contains(out, weekTestAnswer) {
			t.Fatalf("unverified create was reported as success: out=%q err=%v", out, err)
		}
	}
}

func TestWeekResolveRequiresConfirmationAndNoUnverifiedInstance(t *testing.T) {
	api := &weekTestAPI{}
	out, err := runWeekCommand(t, api.call, "", "", false, "resolve", weekTestGame)
	if err != nil || !strings.Contains(out, "Dry run") || len(api.calls) != 0 {
		t.Fatalf("resolve dry-run: out=%q err=%v calls=%#v", out, err, api.calls)
	}
	out, err = runWeekCommand(t, api.call, "", "", true, "resolve", weekTestGame)
	if err != nil || len(api.calls) != 1 || api.calls[0].method != "POST" || api.calls[0].path != "/castawordle/"+weekTestGame+"/resolve" {
		t.Fatalf("resolve apply: out=%q err=%v calls=%#v", out, err, api.calls)
	}
	if _, err := runWeekCommand(t, api.call, "", weekTestInstance, true, "resolve", weekTestGame); err == nil {
		t.Fatal("resolve silently accepted an unverified --instance")
	}
}

func TestValidateWeekGameRequiresMatchingScoredFields(t *testing.T) {
	plan, err := readWeekDeclaration(writeWeekFixture(t, weekTestAnswer), "")
	if err != nil {
		t.Fatal(err)
	}
	game := weekGameResponse{
		ID: weekTestGame, InstanceID: weekTestInstance, Name: "Episode 2",
		OpensAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), CutoffAt: time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC),
		EpisodeNumber: new(int),
	}
	*game.EpisodeNumber = 2
	game.Unscored = new(bool)
	if _, err := validateWeekGame(game, plan); err != nil {
		t.Fatalf("valid scored response rejected: %v", err)
	}
	wrong := game
	wrong.Name = "different puzzle"
	if _, err := validateWeekGame(wrong, plan); err == nil {
		t.Fatal("mismatched response accepted")
	}
}
