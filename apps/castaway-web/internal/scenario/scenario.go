package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

type Scenario struct {
	Version      int      `yaml:"version"`
	Instance     Instance `yaml:"instance"`
	Contestants  []string `yaml:"contestants"`
	Participants []string `yaml:"participants"`
	Steps        []Step   `yaml:"steps"`
}

type Instance struct {
	Name     string    `yaml:"name"`
	Season   int       `yaml:"season"`
	Episodes []Episode `yaml:"episodes"`
}

type Episode struct {
	EpisodeNumber int    `yaml:"episode_number"`
	Label         string `yaml:"label"`
	AirsAt        string `yaml:"airs_at"`
}

type Step struct {
	ID          string                   `yaml:"id"`
	Action      string                   `yaml:"action"`
	Key         string                   `yaml:"key"`
	EffectiveAt string                   `yaml:"effective_at"`
	Episode     int                      `yaml:"episode"`
	Participant string                   `yaml:"participant"`
	Picks       []string                 `yaml:"picks"`
	Position    int                      `yaml:"position"`
	Contestant  string                   `yaml:"contestant"`
	Reason      string                   `yaml:"reason"`
	Correction  *bool                    `yaml:"correction"`
	RetryOf     string                   `yaml:"retry_of"`
	Leaderboard []LeaderboardExpectation `yaml:"leaderboard"`
	Outcomes    []OutcomeExpectation     `yaml:"outcomes"`
}

type LeaderboardExpectation struct {
	Participant     string `yaml:"participant"`
	Score           *int   `yaml:"score"`
	DraftPoints     *int   `yaml:"draft_points"`
	BonusPoints     *int   `yaml:"bonus_points"`
	TotalPoints     *int   `yaml:"total_points"`
	PointsAvailable *int   `yaml:"points_available"`
}

type OutcomeExpectation struct {
	Position   int    `yaml:"position"`
	Contestant string `yaml:"contestant"`
}

func Parse(data []byte) (Scenario, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var scenario Scenario
	if err := decoder.Decode(&scenario); err != nil {
		return Scenario{}, fmt.Errorf("decode scenario YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Scenario{}, fmt.Errorf("scenario YAML must contain one document")
		}
		return Scenario{}, fmt.Errorf("decode scenario YAML: %w", err)
	}
	if err := validate(scenario); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

func RenderYAML(data []byte) ([]byte, error) {
	scenario, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return Render(scenario)
}

func Render(scenario Scenario) ([]byte, error) {
	if err := validate(scenario); err != nil {
		return nil, err
	}

	contestantNames, err := namesByKey(scenario.Contestants, "contestants")
	if err != nil {
		return nil, err
	}
	participantNames, err := namesByKey(scenario.Participants, "participants")
	if err != nil {
		return nil, err
	}
	contestantVars := variableNames("contestant", scenario.Contestants)
	participantVars := variableNames("participant", scenario.Participants)
	instanceJSON, err := instanceBody(scenario)
	if err != nil {
		return nil, err
	}

	var output strings.Builder
	appendEntry(&output, hurlEntry{
		method:   "POST",
		path:     "{{base_url}}/instances",
		body:     instanceJSON,
		status:   201,
		captures: []hurlCapture{{name: "instance_id", query: "$.instance.id"}},
		asserts: []string{
			fmt.Sprintf(`jsonpath "$.instance.name" == "%s"`, hurlString(scenario.Instance.Name)),
			fmt.Sprintf(`jsonpath "$.instance.season" == %d`, scenario.Instance.Season),
		},
	})

	for _, participant := range scenario.Participants {
		canonical := participantNames[strings.ToLower(participant)]
		appendEntry(&output, hurlEntry{
			method: "POST",
			path:   "{{base_url}}/instances/{{instance_id}}/participants",
			body:   mustJSON(map[string]any{"name": canonical}),
			status: 201,
			captures: []hurlCapture{{
				name:  participantVars[canonical],
				query: "$.participant.id",
			}},
			asserts: []string{
				fmt.Sprintf(`jsonpath "$.participant.name" == "%s"`, hurlString(canonical)),
			},
		})
	}

	lookup := hurlEntry{
		method: "GET",
		path:   "{{base_url}}/instances/{{instance_id}}",
		status: 200,
		asserts: []string{
			fmt.Sprintf(`jsonpath "$.contestants" count == %d`, len(scenario.Contestants)),
			fmt.Sprintf(`jsonpath "$.participants" count == %d`, len(scenario.Participants)),
		},
	}
	for _, contestant := range scenario.Contestants {
		canonical := contestantNames[strings.ToLower(contestant)]
		lookup.captures = append(lookup.captures, hurlCapture{
			name:  contestantVars[canonical],
			query: contestantQuery(canonical, "id"),
			first: true,
		})
		lookup.asserts = append(lookup.asserts, fmt.Sprintf(`jsonpath "%s" count == 1`, hurlString(contestantQuery(canonical, "name"))))
	}
	appendEntry(&output, lookup)

	requests := make(map[string]hurlEntry)
	revisionVariables := make(map[string]string)
	for stepIndex, step := range scenario.Steps {
		if step.Action == "retry" {
			entry, ok := requests[step.RetryOf]
			if !ok {
				return nil, fmt.Errorf("step %q retries unknown request %q", step.ID, step.RetryOf)
			}
			entry.captures = nil
			entry.asserts = append([]string{}, entry.asserts...)
			if revisionVariable, ok := revisionVariables[step.RetryOf]; ok {
				entry.asserts = append(entry.asserts, fmt.Sprintf(`jsonpath "$.outcome.revision_number" == %s`, hurlVariable(revisionVariable)))
			}
			appendEntry(&output, entry)
			continue
		}

		entry, err := renderStep(step, stepIndex, contestantNames, participantNames, contestantVars, participantVars)
		if err != nil {
			return nil, err
		}
		appendEntry(&output, entry)
		if isRequestAction(step.Action) {
			requests[step.ID] = entry
		}
		if step.Action == "outcome.upsert" && step.Correction != nil && *step.Correction {
			revisionVariables[step.ID] = revisionCaptureName(stepIndex)
		}
	}

	return []byte(output.String()), nil
}

func validate(scenario Scenario) error {
	if scenario.Version != 1 {
		return fmt.Errorf("version must be 1")
	}
	if strings.TrimSpace(scenario.Instance.Name) == "" {
		return fmt.Errorf("instance.name is required")
	}
	if scenario.Instance.Name != strings.TrimSpace(scenario.Instance.Name) || hasControl(scenario.Instance.Name) || hasHurlTemplate(scenario.Instance.Name) {
		return fmt.Errorf("instance.name must not have surrounding whitespace, control characters, or Hurl template delimiters")
	}
	if scenario.Instance.Season <= 0 {
		return fmt.Errorf("instance.season must be positive")
	}
	contestants, err := namesByKey(scenario.Contestants, "contestants")
	if err != nil {
		return err
	}
	participants, err := namesByKey(scenario.Participants, "participants")
	if err != nil {
		return err
	}
	if len(scenario.Instance.Episodes) == 0 {
		return fmt.Errorf("instance.episodes must not be empty")
	}
	var previous time.Time
	for index, episode := range scenario.Instance.Episodes {
		if episode.EpisodeNumber != index {
			return fmt.Errorf("instance.episodes[%d].episode_number must be %d", index, index)
		}
		if strings.TrimSpace(episode.Label) == "" || hasControl(episode.Label) || hasHurlTemplate(episode.Label) {
			return fmt.Errorf("instance.episodes[%d].label must not be empty or contain unsafe characters", index)
		}
		at, err := parseTimestamp(episode.AirsAt)
		if err != nil {
			return fmt.Errorf("instance.episodes[%d].airs_at: %w", index, err)
		}
		if index > 0 && !at.After(previous) {
			return fmt.Errorf("instance.episodes[%d].airs_at must be after the previous episode", index)
		}
		previous = at
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("steps must not be empty")
	}

	stepActions := make(map[string]string, len(scenario.Steps))
	stepKeys := make(map[string]string)
	for index, step := range scenario.Steps {
		prefix := fmt.Sprintf("steps[%d]", index)
		if strings.TrimSpace(step.ID) == "" {
			return fmt.Errorf("%s.id is required", prefix)
		}
		if _, exists := stepActions[step.ID]; exists {
			return fmt.Errorf("%s.id %q is duplicated", prefix, step.ID)
		}
		if strings.TrimSpace(step.Action) == "" {
			return fmt.Errorf("%s.action is required", prefix)
		}
		if err := validateStepText(step, prefix); err != nil {
			return err
		}
		if err := validateStepFields(step, prefix); err != nil {
			return err
		}

		switch step.Action {
		case "draft.open", "draft.close":
			if _, _, err := commandFields(step, prefix); err != nil {
				return err
			}
		case "draft.submit", "draft.late":
			if _, _, err := commandFields(step, prefix); err != nil {
				return err
			}
			if _, err := requireName(participants, step.Participant, prefix+".participant"); err != nil {
				return err
			}
			if len(step.Picks) == 0 {
				return fmt.Errorf("%s.picks must not be empty", prefix)
			}
			if len(step.Picks) != len(scenario.Contestants) {
				return fmt.Errorf("%s.picks must contain every contestant exactly once", prefix)
			}
			seen := make(map[string]struct{}, len(step.Picks))
			for pickIndex, pick := range step.Picks {
				canonical, err := requireName(contestants, pick, fmt.Sprintf("%s.picks[%d]", prefix, pickIndex))
				if err != nil {
					return err
				}
				key := strings.ToLower(canonical)
				if _, exists := seen[key]; exists {
					return fmt.Errorf("%s.picks contains duplicate contestant %q", prefix, canonical)
				}
				seen[key] = struct{}{}
			}
		case "episode.start", "episode.complete", "episode.score":
			if _, _, err := commandFields(step, prefix); err != nil {
				return err
			}
			if step.Episode <= 0 || step.Episode >= len(scenario.Instance.Episodes) {
				return fmt.Errorf("%s.episode must reference a positive scheduled episode", prefix)
			}
		case "outcome.upsert":
			if _, _, err := commandFields(step, prefix); err != nil {
				return err
			}
			if step.Position <= 0 || step.Position > len(scenario.Contestants) {
				return fmt.Errorf("%s.position must reference a contestant position", prefix)
			}
			if _, err := requireName(contestants, step.Contestant, prefix+".contestant"); err != nil {
				return err
			}
		case "retry":
			if strings.TrimSpace(step.RetryOf) == "" {
				return fmt.Errorf("%s.retry_of is required", prefix)
			}
			targetAction, exists := stepActions[step.RetryOf]
			if !exists {
				return fmt.Errorf("%s.retry_of must reference an earlier request step", prefix)
			}
			if !isRequestAction(targetAction) {
				return fmt.Errorf("%s.retry_of must reference an API request step", prefix)
			}
			if strings.TrimSpace(step.Key) != "" || strings.TrimSpace(step.EffectiveAt) != "" {
				return fmt.Errorf("%s.retry must not define key or effective_at", prefix)
			}
		case "assert.leaderboard":
			if len(step.Leaderboard) == 0 {
				return fmt.Errorf("%s.leaderboard must not be empty", prefix)
			}
			seen := make(map[string]struct{}, len(step.Leaderboard))
			for resultIndex, expected := range step.Leaderboard {
				canonical, err := requireName(participants, expected.Participant, fmt.Sprintf("%s.leaderboard[%d].participant", prefix, resultIndex))
				if err != nil {
					return err
				}
				key := strings.ToLower(canonical)
				if _, exists := seen[key]; exists {
					return fmt.Errorf("%s.leaderboard contains duplicate participant %q", prefix, canonical)
				}
				seen[key] = struct{}{}
			}
		case "assert.outcomes":
			if len(step.Outcomes) == 0 {
				return fmt.Errorf("%s.outcomes must not be empty", prefix)
			}
			previousPosition := 0
			for resultIndex, expected := range step.Outcomes {
				if expected.Position <= previousPosition {
					return fmt.Errorf("%s.outcomes[%d].position must be in ascending order", prefix, resultIndex)
				}
				if _, err := requireName(contestants, expected.Contestant, fmt.Sprintf("%s.outcomes[%d].contestant", prefix, resultIndex)); err != nil {
					return err
				}
				previousPosition = expected.Position
			}
		default:
			return fmt.Errorf("%s.action %q is not supported", prefix, step.Action)
		}

		if isRequestAction(step.Action) {
			key := strings.TrimSpace(step.Key)
			if _, exists := stepKeys[key]; exists {
				return fmt.Errorf("%s.key %q is duplicated; use retry for an identical request", prefix, key)
			}
			stepKeys[key] = step.ID
		}
		stepActions[step.ID] = step.Action
	}
	return nil
}

func validateStepText(step Step, prefix string) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "id", value: step.ID},
		{name: "action", value: step.Action},
		{name: "key", value: step.Key},
		{name: "effective_at", value: step.EffectiveAt},
		{name: "participant", value: step.Participant},
		{name: "contestant", value: step.Contestant},
		{name: "reason", value: step.Reason},
		{name: "retry_of", value: step.RetryOf},
	}
	for _, field := range fields {
		if hasHurlTemplate(field.value) {
			return fmt.Errorf("%s.%s must not contain Hurl template delimiters", prefix, field.name)
		}
	}
	for index, pick := range step.Picks {
		if hasHurlTemplate(pick) {
			return fmt.Errorf("%s.picks[%d] must not contain Hurl template delimiters", prefix, index)
		}
	}
	for index, expected := range step.Leaderboard {
		if hasHurlTemplate(expected.Participant) {
			return fmt.Errorf("%s.leaderboard[%d].participant must not contain Hurl template delimiters", prefix, index)
		}
	}
	for index, expected := range step.Outcomes {
		if hasHurlTemplate(expected.Contestant) {
			return fmt.Errorf("%s.outcomes[%d].contestant must not contain Hurl template delimiters", prefix, index)
		}
	}
	return nil
}

func validateStepFields(step Step, prefix string) error {
	var allowed map[string]bool
	switch step.Action {
	case "draft.open", "draft.close":
		allowed = map[string]bool{"key": true, "effective_at": true}
	case "draft.submit", "draft.late":
		allowed = map[string]bool{"key": true, "effective_at": true, "participant": true, "picks": true, "reason": true}
	case "episode.start", "episode.complete", "episode.score":
		allowed = map[string]bool{"key": true, "effective_at": true, "episode": true}
	case "outcome.upsert":
		allowed = map[string]bool{"key": true, "effective_at": true, "position": true, "contestant": true, "reason": true, "correction": true}
	case "retry":
		allowed = map[string]bool{"retry_of": true}
	case "assert.leaderboard":
		allowed = map[string]bool{"leaderboard": true}
	case "assert.outcomes":
		allowed = map[string]bool{"outcomes": true}
	default:
		return nil
	}
	fields := []struct {
		name    string
		present bool
	}{
		{name: "key", present: strings.TrimSpace(step.Key) != ""},
		{name: "effective_at", present: strings.TrimSpace(step.EffectiveAt) != ""},
		{name: "episode", present: step.Episode != 0},
		{name: "participant", present: strings.TrimSpace(step.Participant) != ""},
		{name: "picks", present: len(step.Picks) > 0},
		{name: "position", present: step.Position != 0},
		{name: "contestant", present: strings.TrimSpace(step.Contestant) != ""},
		{name: "reason", present: strings.TrimSpace(step.Reason) != ""},
		{name: "correction", present: step.Correction != nil},
		{name: "retry_of", present: strings.TrimSpace(step.RetryOf) != ""},
		{name: "leaderboard", present: len(step.Leaderboard) > 0},
		{name: "outcomes", present: len(step.Outcomes) > 0},
	}
	for _, field := range fields {
		if field.present && !allowed[field.name] {
			return fmt.Errorf("%s.%s is not valid for action %q", prefix, field.name, step.Action)
		}
	}
	return nil
}

func hasHurlTemplate(value string) bool {
	return strings.Contains(value, "{{") || strings.Contains(value, "}}")
}

func namesByKey(names []string, field string) (map[string]string, error) {
	result := make(map[string]string, len(names))
	for index, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, fmt.Errorf("%s[%d] must not be empty", field, index)
		}
		if name != raw || hasControl(name) || hasHurlTemplate(name) {
			return nil, fmt.Errorf("%s[%d] must not have surrounding whitespace, control characters, or Hurl template delimiters", field, index)
		}
		key := strings.ToLower(name)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("%s contains duplicate name %q", field, name)
		}
		result[key] = name
	}
	return result, nil
}

func requireName(names map[string]string, raw, field string) (string, error) {
	if hasHurlTemplate(raw) {
		return "", fmt.Errorf("%s must not contain Hurl template delimiters", field)
	}
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	canonical, exists := names[strings.ToLower(name)]
	if !exists {
		return "", fmt.Errorf("%s references unknown name %q", field, raw)
	}
	return canonical, nil
}

func commandFields(step Step, prefix string) (time.Time, string, error) {
	key := strings.TrimSpace(step.Key)
	if key == "" {
		return time.Time{}, "", fmt.Errorf("%s.key is required", prefix)
	}
	at, err := parseTimestamp(step.EffectiveAt)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("%s.effective_at: %w", prefix, err)
	}
	return at, key, nil
}

func parseTimestamp(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be RFC3339: %w", err)
	}
	return at.UTC(), nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

type hurlEntry struct {
	method   string
	path     string
	body     string
	status   int
	captures []hurlCapture
	asserts  []string
}

type hurlCapture struct {
	name  string
	query string
	first bool
}

func appendEntry(output *strings.Builder, entry hurlEntry) {
	fmt.Fprintf(output, "%s %s\n", entry.method, entry.path)
	output.WriteString("Authorization: Bearer {{service_token}}\n")
	output.WriteString("X-Discord-User-ID: {{operator_discord_user_id}}\n")
	if entry.body != "" {
		output.WriteString("Content-Type: application/json\n")
		output.WriteString(entry.body)
		output.WriteByte('\n')
	}
	fmt.Fprintf(output, "HTTP %d\n", entry.status)
	if len(entry.captures) > 0 {
		output.WriteString("[Captures]\n")
		for _, capture := range entry.captures {
			fmt.Fprintf(output, "%s: jsonpath \"%s\"", capture.name, hurlString(capture.query))
			if capture.first {
				output.WriteString(" first")
			}
			output.WriteByte('\n')
		}
	}
	if len(entry.asserts) > 0 {
		output.WriteString("[Asserts]\n")
		for _, assertion := range entry.asserts {
			output.WriteString(assertion)
			output.WriteByte('\n')
		}
	}
	output.WriteByte('\n')
}

func instanceBody(scenario Scenario) (string, error) {
	episodes := make([]map[string]any, 0, len(scenario.Instance.Episodes))
	for index, episode := range scenario.Instance.Episodes {
		at, err := parseTimestamp(episode.AirsAt)
		if err != nil {
			return "", fmt.Errorf("instance.episodes[%d].airs_at: %w", index, err)
		}
		episodes = append(episodes, map[string]any{
			"episode_number": episode.EpisodeNumber,
			"label":          episode.Label,
			"airs_at":        at.Format(time.RFC3339Nano),
		})
	}
	return mustJSON(map[string]any{
		"name":                scenario.Instance.Name,
		"season":              scenario.Instance.Season,
		"contestants":         scenario.Contestants,
		"managed_progression": true,
		"episodes":            episodes,
	}), nil
}

func renderStep(step Step, stepIndex int, contestants, participants map[string]string, contestantVars, participantVars map[string]string) (hurlEntry, error) {
	at, key, err := commandFieldsIfRequest(step)
	if err != nil {
		return hurlEntry{}, err
	}
	instancePath := "{{base_url}}/instances/{{instance_id}}"
	commandBody := func() string {
		return mustJSON(map[string]any{
			"idempotency_key": key,
			"effective_at":    at.Format(time.RFC3339Nano),
		})
	}

	switch step.Action {
	case "draft.open":
		return hurlEntry{method: "POST", path: instancePath + "/progression/draft/open", body: commandBody(), status: 200}, nil
	case "draft.close":
		return hurlEntry{method: "POST", path: instancePath + "/progression/draft/close", body: commandBody(), status: 200}, nil
	case "episode.start", "episode.complete", "episode.score":
		action := strings.TrimPrefix(step.Action, "episode.")
		return hurlEntry{
			method: "POST",
			path:   instancePath + "/progression/episodes/" + strconv.Itoa(step.Episode) + "/" + action,
			body:   commandBody(),
			status: 200,
		}, nil
	case "draft.submit", "draft.late":
		participant, err := requireName(participants, step.Participant, "step "+step.ID+".participant")
		if err != nil {
			return hurlEntry{}, err
		}
		picks := make([]string, 0, len(step.Picks))
		for _, pick := range step.Picks {
			contestant, err := requireName(contestants, pick, "step "+step.ID+".picks")
			if err != nil {
				return hurlEntry{}, err
			}
			picks = append(picks, hurlVariable(contestantVars[contestant]))
		}
		body := map[string]any{
			"contestant_ids":  picks,
			"idempotency_key": key,
			"effective_at":    at.Format(time.RFC3339Nano),
		}
		if strings.TrimSpace(step.Reason) != "" {
			body["reason"] = step.Reason
		}
		path := instancePath + "/drafts/" + hurlVariable(participantVars[participant])
		if step.Action == "draft.late" {
			path += "/late"
		}
		return hurlEntry{method: methodForDraft(step.Action), path: path, body: mustJSON(body), status: 200}, nil
	case "outcome.upsert":
		contestant, err := requireName(contestants, step.Contestant, "step "+step.ID+".contestant")
		if err != nil {
			return hurlEntry{}, err
		}
		body := map[string]any{
			"contestant_id":   hurlVariable(contestantVars[contestant]),
			"idempotency_key": key,
			"effective_at":    at.Format(time.RFC3339Nano),
		}
		if strings.TrimSpace(step.Reason) != "" {
			body["reason"] = step.Reason
		}
		if step.Correction != nil && *step.Correction {
			body["correction"] = true
		}
		entry := hurlEntry{
			method: "PUT",
			path:   instancePath + "/outcomes/" + strconv.Itoa(step.Position),
			body:   mustJSON(body),
			status: 200,
			asserts: []string{
				fmt.Sprintf(`jsonpath "$.outcome.position" == %d`, step.Position),
				fmt.Sprintf(`jsonpath "$.outcome.contestant_id" == "%s"`, hurlString(hurlVariable(contestantVars[contestant]))),
			},
		}
		if step.Correction != nil && *step.Correction {
			entry.captures = []hurlCapture{{name: revisionCaptureName(stepIndex), query: "$.outcome.revision_number"}}
		}
		return entry, nil
	case "assert.leaderboard":
		asserts := []string{fmt.Sprintf(`jsonpath "$.leaderboard" count == %d`, len(step.Leaderboard))}
		for index, expected := range step.Leaderboard {
			participant, err := requireName(participants, expected.Participant, "step "+step.ID+".leaderboard.participant")
			if err != nil {
				return hurlEntry{}, err
			}
			prefix := fmt.Sprintf("$.leaderboard[%d]", index)
			asserts = append(asserts, fmt.Sprintf(`jsonpath "%s.participant_name" == "%s"`, prefix, hurlString(participant)))
			asserts = appendLeaderboardValue(asserts, prefix, "score", expected.Score)
			asserts = appendLeaderboardValue(asserts, prefix, "draft_points", expected.DraftPoints)
			asserts = appendLeaderboardValue(asserts, prefix, "bonus_points", expected.BonusPoints)
			asserts = appendLeaderboardValue(asserts, prefix, "total_points", expected.TotalPoints)
			asserts = appendLeaderboardValue(asserts, prefix, "points_available", expected.PointsAvailable)
		}
		return hurlEntry{method: "GET", path: instancePath + "/leaderboard", status: 200, asserts: asserts}, nil
	case "assert.outcomes":
		asserts := []string{fmt.Sprintf(`jsonpath "$.outcomes" count == %d`, len(step.Outcomes))}
		for index, expected := range step.Outcomes {
			contestant, err := requireName(contestants, expected.Contestant, "step "+step.ID+".outcomes.contestant")
			if err != nil {
				return hurlEntry{}, err
			}
			prefix := fmt.Sprintf("$.outcomes[%d]", index)
			asserts = append(asserts, fmt.Sprintf(`jsonpath "%s.position" == %d`, prefix, expected.Position))
			asserts = append(asserts, fmt.Sprintf(`jsonpath "%s.contestant_name" == "%s"`, prefix, hurlString(contestant)))
		}
		return hurlEntry{method: "GET", path: instancePath + "/outcomes", status: 200, asserts: asserts}, nil
	default:
		return hurlEntry{}, fmt.Errorf("step %q action %q cannot be rendered", step.ID, step.Action)
	}
}

func commandFieldsIfRequest(step Step) (time.Time, string, error) {
	if !isRequestAction(step.Action) {
		return time.Time{}, "", nil
	}
	return commandFields(step, "step "+step.ID)
}

func methodForDraft(action string) string {
	if action == "draft.late" {
		return "POST"
	}
	return "PUT"
}

func appendLeaderboardValue(asserts []string, prefix, field string, value *int) []string {
	if value == nil {
		return asserts
	}
	return append(asserts, fmt.Sprintf(`jsonpath "%s.%s" == %d`, prefix, field, *value))
}

func isRequestAction(action string) bool {
	switch action {
	case "draft.open", "draft.close", "draft.submit", "draft.late", "episode.start", "episode.complete", "episode.score", "outcome.upsert":
		return true
	default:
		return false
	}
}

func variableNames(prefix string, names []string) map[string]string {
	result := make(map[string]string, len(names))
	for index, name := range names {
		result[name] = fmt.Sprintf("%s_%d_id", prefix, index)
	}
	return result
}

func revisionCaptureName(stepIndex int) string {
	return fmt.Sprintf("step_%d_revision", stepIndex)
}

func contestantQuery(name, field string) string {
	return "$.contestants[?(@.name == '" + jsonPathString(name) + "')]." + field
}

func jsonPathString(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			result.WriteString(`\\`)
		case '\'':
			result.WriteString(`\'`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

func hurlVariable(name string) string {
	return "{{" + name + "}}"
}

func hurlString(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			result.WriteString(`\\`)
		case '"':
			result.WriteString(`\"`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

func mustJSON(value any) string {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
