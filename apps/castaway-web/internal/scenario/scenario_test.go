package scenario

import (
	"strings"
	"testing"
)

func TestParseRejectsInvalidScenarios(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{
			name: "unsupported field",
			source: `
version: 1
instance:
  name: Example
  season: 1
  unsupported: true
  episodes:
    - episode_number: 0
      label: Preseason
      airs_at: 2026-01-01T00:00:00Z
contestants: [Ada]
participants: [Bryan]
steps:
  - id: open
    action: draft.open
    key: open
    effective_at: 2026-01-01T01:00:00Z
`,
			message: "field unsupported not found",
		},
		{
			name:    "schedule does not start at zero",
			source:  invalidScheduleScenario(),
			message: "episode_number must be 0",
		},
		{
			name: "unknown draft contestant",
			source: validScenario(`
    - id: draft
      action: draft.submit
      key: draft
      effective_at: 2026-01-01T01:00:00Z
      participant: Bryan
      picks: [Missing, Ada]
`),
			message: "unknown name",
		},
		{
			name: "empty draft picks",
			source: `
version: 1
instance:
  name: Example
  season: 1
  episodes:
    - episode_number: 0
      label: Preseason
      airs_at: 2026-01-01T00:00:00Z
contestants: []
participants: [Bryan]
steps:
  - id: draft
    action: draft.submit
    key: draft
    effective_at: 2026-01-01T01:00:00Z
    participant: Bryan
    picks: []
`,
			message: "picks must not be empty",
		},
		{
			name: "retry must reference an earlier request",
			source: validScenario(`
    - id: retry
      action: retry
      retry_of: later
    - id: later
      action: draft.open
      key: open
      effective_at: 2026-01-01T01:00:00Z
`),
			message: "earlier request",
		},
		{
			name: "unsupported action",
			source: validScenario(`
    - id: unsupported
      action: magic
      key: magic
      effective_at: 2026-01-01T01:00:00Z
`),
			message: "is not supported",
		},
		{
			name: "malformed timestamp",
			source: validScenario(`
    - id: bad-time
      action: draft.close
      key: bad-time
      effective_at: tomorrow
`),
			message: "must be RFC3339",
		},
		{
			name: "incompatible fields",
			source: validScenario(`
    - id: invalid-correction
      action: draft.open
      key: invalid-correction
      effective_at: 2026-01-01T01:00:00Z
      correction: true
`),
			message: "not valid for action",
		},
		{
			name: "literal template is rejected",
			source: validScenario(`
    - id: templated
      action: draft.close
      key: "literal-{{value}}"
      effective_at: 2026-01-01T01:00:00Z
`),
			message: "Hurl template delimiters",
		},
		{
			name: "command keys are unique",
			source: validScenario(`
    - id: second
      action: draft.close
      key: open
      effective_at: 2026-01-01T02:00:00Z
`),
			message: "key \"open\" is duplicated",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.source))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestRenderUsesHurlCapturesAndExactRetries(t *testing.T) {
	source := validScenario(`
    - id: draft-with-reason
      action: draft.submit
      key: draft-with-reason
      effective_at: 2026-01-01T02:00:00Z
      participant: Bryan
      picks: [Ada, Coach O'Brien]
      reason: draft correction
    - id: outcome
      action: outcome.upsert
      key: outcome
      effective_at: 2026-01-01T03:00:00Z
      position: 1
      contestant: Coach O'Brien
      correction: true
      reason: first result
    - id: outcome-retry
      action: retry
      retry_of: outcome
    - id: hidden
      action: assert.outcomes
      outcomes:
        - position: 1
          contestant: Coach O'Brien
`)

	rendered, err := RenderYAML([]byte(source))
	if err != nil {
		t.Fatalf("RenderYAML() error = %v", err)
	}
	output := string(rendered)

	checks := []string{
		`POST {{base_url}}/instances`,
		`Authorization: Bearer {{service_token}}`,
		`contestant_1_id: jsonpath "$.contestants[?(@.name == 'Coach O\\'Brien')].id"`,
		`PUT {{base_url}}/instances/{{instance_id}}/outcomes/1`,
		`"reason": "draft correction"`,
		`"correction": true`,
		`step_2_revision: jsonpath "$.outcome.revision_number"`,
		`jsonpath "$.outcome.revision_number" == {{step_2_revision}}`,
		`GET {{base_url}}/instances/{{instance_id}}/outcomes`,
	}
	if count := strings.Count(output, `"correction": true`); count != 2 {
		t.Fatalf("rendered correction request count = %d, want 2", count)
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("rendered Hurl does not contain %q:\n%s", check, output)
		}
	}
	if count := strings.Count(output, `"idempotency_key": "outcome"`); count != 2 {
		t.Fatalf("rendered retry count = %d, want 2", count)
	}
}

func TestVariableNamesUseIndexes(t *testing.T) {
	variables := variableNames("contestant", []string{"A-B", "A B", "A_B"})
	if variables["A-B"] != "contestant_0_id" || variables["A B"] != "contestant_1_id" || variables["A_B"] != "contestant_2_id" {
		t.Fatalf("variableNames() = %#v", variables)
	}
}

func TestRenderRejectsUnsafeNames(t *testing.T) {
	source := `version: 1
instance:
  name: Example
  season: 1
  episodes:
    - episode_number: 0
      label: Preseason
      airs_at: 2026-01-01T00:00:00Z
contestants:
  - "Ada\nEvil"
participants:
  - Bryan
steps:
  - id: open
    action: draft.open
    key: open
    effective_at: 2026-01-01T01:00:00Z
`
	if _, err := RenderYAML([]byte(source)); err == nil {
		t.Fatal("RenderYAML() accepted a control character in a name")
	}
}

func invalidScheduleScenario() string {
	return `version: 1
instance:
  name: Example
  season: 1
  episodes:
    - episode_number: 1
      label: Episode 1
      airs_at: 2026-01-02T00:00:00Z
contestants: [Ada]
participants: [Bryan]
steps:
  - id: open
    action: draft.open
    key: open
    effective_at: 2026-01-01T01:00:00Z
`
}

func validScenario(extraSteps string) string {
	return `version: 1
instance:
  name: Example
  season: 1
  episodes:
    - episode_number: 0
      label: Preseason
      airs_at: 2026-01-01T00:00:00Z
    - episode_number: 1
      label: Episode 1
      airs_at: 2026-01-02T00:00:00Z
contestants:
  - Ada
  - Coach O'Brien
participants:
  - Bryan
steps:
    - id: open
      action: draft.open
      key: open
      effective_at: 2026-01-01T01:00:00Z
` + extraSteps
}
