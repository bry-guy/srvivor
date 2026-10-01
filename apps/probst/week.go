package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

type weekDeclaration struct {
	Version    int    `json:"version"`
	InstanceID string `json:"instance_id"`
	Episode    int    `json:"episode"`
	Puzzle     struct {
		Name       string `json:"name"`
		AnswerFile string `json:"answer_file"`
		OpensAt    string `json:"opens_at"`
		CutoffAt   string `json:"cutoff_at"`
	} `json:"puzzle"`
}

type preparedWeek struct {
	declaration weekDeclaration
	instanceID  string
	answer      string
	opensAt     time.Time
	cutoffAt    time.Time
}

type weekGameResponse struct {
	ID            string    `json:"id"`
	InstanceID    string    `json:"instance_id"`
	Name          string    `json:"name"`
	OpensAt       time.Time `json:"opens_at"`
	CutoffAt      time.Time `json:"cutoff_at"`
	Unscored      *bool     `json:"unscored"`
	EpisodeNumber *int      `json:"episode_number"`
}

func newWeekCommand(call apiCall, instance *string, yes *bool) *cobra.Command {
	var file string
	week := &cobra.Command{
		Use:   "week",
		Short: "Prepare a scored Castawordle from a JSON declaration",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			plan, err := readWeekDeclaration(file, *instance)
			if err != nil {
				return err
			}
			if err := inspectWeekInstance(c.Context(), call, plan); err != nil {
				return err
			}
			if !*yes {
				return printWeekPreview(c, plan)
			}
			if err := inspectWeekInstance(c.Context(), call, plan); err != nil {
				return err
			}
			path := "/instances/" + url.PathEscape(plan.instanceID)
			body := map[string]any{
				"name": plan.declaration.Puzzle.Name, "answer": plan.answer,
				"episode_number": plan.declaration.Episode,
				"scored":         true,
				"window": map[string]string{
					"opens_at":  plan.opensAt.Format(time.RFC3339Nano),
					"cutoff_at": plan.cutoffAt.Format(time.RFC3339Nano),
				},
			}
			var game weekGameResponse
			if err := call(c.Context(), "POST", path+"/castawordle", body, &game); err != nil {
				return errors.New("scored Castawordle preparation failed; API details are withheld to protect the private answer")
			}
			gameID, err := validateWeekGame(game, plan)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Prepared scored Castawordle %s, %q, episode %d.\n", gameID, game.Name, *game.EpisodeNumber)
			return err
		},
	}
	week.Flags().StringVar(&file, "file", "", "JSON v1 week declaration")
	week.AddCommand(&cobra.Command{
		Use:   "resolve GAME_ID",
		Short: "Resolve a scored Castawordle after its cutoff",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, err := canonicalWeekUUID(args[0])
			if err != nil {
				return errors.New("GAME_ID must be a UUID")
			}
			if *instance != "" {
				return errors.New("week resolve cannot verify GAME_ID against --instance; omit --instance")
			}
			if !*yes {
				_, err := fmt.Fprintf(c.OutOrStdout(), "Dry run — would ask the server to resolve game %s after its cutoff. Re-run with --yes to apply.\n", gameID)
				return err
			}
			if err := call(c.Context(), "POST", "/castawordle/"+url.PathEscape(gameID)+"/resolve", nil, nil); err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.OutOrStdout(), "Resolved Castawordle game %s.\n", gameID)
			return err
		},
	})
	return week
}

func readWeekDeclaration(file, globalInstance string) (preparedWeek, error) {
	if file == "" {
		return preparedWeek{}, fmt.Errorf("--file is required")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return preparedWeek{}, err
	}
	var declaration weekDeclaration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declaration); err != nil {
		return preparedWeek{}, fmt.Errorf("read week declaration: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return preparedWeek{}, errors.New("week declaration must contain one JSON value")
	}
	if declaration.Version != 1 {
		return preparedWeek{}, fmt.Errorf("week declaration version must be 1")
	}
	instanceID, err := canonicalWeekUUID(declaration.InstanceID)
	if err != nil {
		return preparedWeek{}, fmt.Errorf("instance_id must be a UUID")
	}
	if globalInstance != "" {
		selected, err := canonicalWeekUUID(globalInstance)
		if err != nil || selected != instanceID {
			return preparedWeek{}, fmt.Errorf("declaration instance_id does not match --instance")
		}
	}
	if declaration.Episode < 1 {
		return preparedWeek{}, fmt.Errorf("episode must be a positive integer")
	}
	name := strings.TrimSpace(declaration.Puzzle.Name)
	if name == "" || len(name) > 80 || !utf8.ValidString(name) {
		return preparedWeek{}, fmt.Errorf("puzzle name must be nonempty and at most 80 bytes")
	}
	if declaration.Puzzle.AnswerFile == "" {
		return preparedWeek{}, fmt.Errorf("puzzle.answer_file is required")
	}
	answerPath := declaration.Puzzle.AnswerFile
	if !filepath.IsAbs(answerPath) {
		answerPath = filepath.Join(filepath.Dir(file), answerPath)
	}
	answerBytes, err := os.ReadFile(answerPath)
	if err != nil {
		return preparedWeek{}, errors.New("could not read private puzzle answer file")
	}
	answer := strings.TrimSpace(string(answerBytes))
	if len(answer) != 6 || !utf8.ValidString(answer) {
		return preparedWeek{}, fmt.Errorf("private puzzle answer must be six uppercase ASCII letters")
	}
	for i := 0; i < len(answer); i++ {
		if answer[i] < 'A' || answer[i] > 'Z' {
			return preparedWeek{}, errors.New("private puzzle answer must be six uppercase ASCII letters")
		}
	}
	opensAt, err := parseWeekTime("puzzle.opens_at", declaration.Puzzle.OpensAt)
	if err != nil {
		return preparedWeek{}, err
	}
	cutoffAt, err := parseWeekTime("puzzle.cutoff_at", declaration.Puzzle.CutoffAt)
	if err != nil {
		return preparedWeek{}, err
	}
	if !cutoffAt.After(opensAt) {
		return preparedWeek{}, fmt.Errorf("puzzle.cutoff_at must be after puzzle.opens_at")
	}
	declaration.InstanceID = instanceID
	declaration.Puzzle.Name = name
	return preparedWeek{declaration: declaration, instanceID: instanceID, answer: answer, opensAt: opensAt, cutoffAt: cutoffAt}, nil
}

func parseWeekTime(field, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required in RFC3339 format", field)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339", field)
	}
	return parsed.UTC(), nil
}

func canonicalWeekUUID(value string) (string, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", fmt.Errorf("invalid UUID")
	}
	compact := strings.ReplaceAll(value, "-", "")
	if decoded, err := hex.DecodeString(compact); err != nil || len(decoded) != 16 {
		return "", fmt.Errorf("invalid UUID")
	}
	return strings.ToLower(value), nil
}

func inspectWeekInstance(ctx context.Context, call apiCall, plan preparedWeek) error {
	return call(ctx, "GET", "/instances/"+url.PathEscape(plan.instanceID), nil, nil)
}

func validateWeekGame(game weekGameResponse, plan preparedWeek) (string, error) {
	gameID, err := canonicalWeekUUID(game.ID)
	if err != nil {
		return "", errors.New("server did not confirm a valid scored game; re-run after checking instance state")
	}
	instanceID, err := canonicalWeekUUID(game.InstanceID)
	if err != nil || instanceID != plan.instanceID || game.Name != plan.declaration.Puzzle.Name || game.Unscored == nil || *game.Unscored || game.EpisodeNumber == nil || *game.EpisodeNumber != plan.declaration.Episode || game.OpensAt.IsZero() || !game.OpensAt.Equal(plan.opensAt) || game.CutoffAt.IsZero() || !game.CutoffAt.Equal(plan.cutoffAt) {
		return "", errors.New("server did not confirm the requested scored game; re-run after checking instance state")
	}
	return gameID, nil
}

func printWeekPreview(c *cobra.Command, plan preparedWeek) error {
	_, err := fmt.Fprintf(c.OutOrStdout(), "Dry run — instance %s, episode %d: scored Castawordle %q (%s → %s); private answer loaded but not shown. No writes; the API validates the answer dictionary and scored-game contract on apply. Re-run with --yes to apply.\n",
		plan.instanceID, plan.declaration.Episode, plan.declaration.Puzzle.Name,
		plan.opensAt.Format(time.RFC3339), plan.cutoffAt.Format(time.RFC3339))
	return err
}
