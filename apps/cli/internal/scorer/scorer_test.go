package scorer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper to create a draft from a slice of player names. Positions start at 1.
func makeDraft(names []string) *Draft {
	d := &Draft{}
	for i, n := range names {
		d.Entries = append(d.Entries, Entry{position: i + 1, PlayerName: n})
	}
	return d
}

// Helper to create a final results Draft with totalPositions entries. The
// eliminated map maps PlayerName -> position. Positions without an eliminated
// player will have empty PlayerName (survivors).
func makeFinal(totalPositions int, eliminated map[string]int, drafter string) *Draft {
	f := &Draft{Metadata: Metadata{Drafter: drafter}}
	for i := 1; i <= totalPositions; i++ {
		entry := Entry{position: i, PlayerName: ""}
		for name, pos := range eliminated {
			if pos == i {
				entry.PlayerName = name
				break
			}
		}
		f.Entries = append(f.Entries, entry)
	}
	return f
}

// Week 4 regression test (integration): verifies the corrected algorithm
// produces the expected ScoreResult for the provided example.
func TestWeek4Regression(t *testing.T) {
	// Draft: Tom(1), Dick(2), Harry(3), Cosmo(4), Elaine(5), Larry(6), Moe(7), Curly(8)
	draftNames := []string{"Tom", "Dick", "Harry", "Cosmo", "Elaine", "Larry", "Moe", "Curly"}
	draft := makeDraft(draftNames)

	// Final: Larry(5th), Dick(6th), Harry(7th), Moe(8th)
	eliminated := map[string]int{
		"Larry": 5,
		"Dick":  6,
		"Harry": 7,
		"Moe":   8,
	}
	final := makeFinal(8, eliminated, "Current")

	// Integration: full score
	res, err := score(draft, final)
	assert.NoError(t, err)
	// Expected: Current Score=3, Points Available=13, Total=16
	assert.Equal(t, 3, res.Score, "Week 4 current score mismatch")
	assert.Equal(t, 13, res.PointsAvailable, "Week 4 points available mismatch")
}

// Unit tests for calculateCurrentScore and calculatePointsAvailable in
// isolation with known inputs and expected outputs.
func TestCalculateCurrentScore_Isolated(t *testing.T) {
	// Reuse Week 4 scenario to validate current score in isolation.
	draft := makeDraft([]string{"Tom", "Dick", "Harry", "Cosmo", "Elaine", "Larry", "Moe", "Curly"})
	finalPositions := map[string]int{"Larry": 5, "Dick": 6, "Harry": 7, "Moe": 8}
	current := calculateCurrentScore(draft, finalPositions, 8)
	assert.Equal(t, 3, current, "calculateCurrentScore produced unexpected value")

	// Additional isolated test: single elimination
	d := makeDraft([]string{"A", "B", "C"})
	finalPos := map[string]int{"C": 3}
	cur := calculateCurrentScore(d, finalPos, 3)
	// Only C contributes: positionValue = 3+1-3 =1, distance=0 => score=1
	assert.Equal(t, 1, cur)
}

func TestCalculatePointsAvailable_Isolated(t *testing.T) {
	// Week 4 scenario points available should be 13
	draft := makeDraft([]string{"Tom", "Dick", "Harry", "Cosmo", "Elaine", "Larry", "Moe", "Curly"})
	finalPositions := map[string]int{"Larry": 5, "Dick": 6, "Harry": 7, "Moe": 8}
	pa := calculatePointsAvailable(draft, finalPositions, 8)
	assert.Equal(t, 13, pa, "calculatePointsAvailable produced unexpected value")

	// Single survivor scenario
	d := makeDraft([]string{"A", "B", "C"})
	finalPos := map[string]int{"C": 3}
	pa2 := calculatePointsAvailable(d, finalPos, 3)
	// Survivors A(position1) and B(position2)
	// A: positionValue=4-1=3 bestDistance=0 => +3
	// B: positionValue=4-2=2 bestDistance=0 => +2
	assert.Equal(t, 5, pa2)
}

// Edge case: All eliminated (no survivors remain). PointsAvailable should be
// computed by the preserved legacy path and in the canonical "perfect match"
// case (final positions match draft positions) it should be 0.
func TestAllEliminatedRegression(t *testing.T) {
	d := makeDraft([]string{"P1", "P2", "P3", "P4", "P5"})
	// Final exactly matches draft positions (everyone eliminated in draft order)
	eliminated := map[string]int{"P1": 1, "P2": 2, "P3": 3, "P4": 4, "P5": 5}
	final := makeFinal(5, eliminated, "Finished")

	res, err := score(d, final)
	assert.NoError(t, err)
	assert.Equal(t, 0, res.PointsAvailable, "All-eliminated canonical case should have 0 points available")
}

// Edge case: None eliminated yet. Current score should be 0.
func TestNoneEliminated(t *testing.T) {
	d := makeDraft([]string{"X", "Y", "Z"})
	cur := calculateCurrentScore(d, map[string]int{}, 3)
	assert.Equal(t, 0, cur)

	// Points available should be positive for the starting state
	pa := calculatePointsAvailable(d, map[string]int{}, 3)
	assert.Greater(t, pa, 0)
}

// Negative scenarios: verify known negative behavior preserved for a
// particular fixture (ensures regression protection).
func TestNegativePointsAvailable_PreservedRegression(t *testing.T) {
	// Use existing fixture that previously yielded a negative points available
	draftFile, err := os.Open("../../../cli/test_fixtures/drafts/0/bryan.txt")
	assert.NoError(t, err)
	defer draftFile.Close()
	draft, err := readDraft(draftFile)
	assert.NoError(t, err)

	finalFile, err := os.Open("../../../cli/test_fixtures/drafts/0/final.txt")
	assert.NoError(t, err)
	defer finalFile.Close()
	final, err := readDraft(finalFile)
	assert.NoError(t, err)

	res, err := score(draft, final)
	assert.NoError(t, err)
	// This fixture is expected to produce a negative PointsAvailable per
	// historical behavior (regression test ensures this doesn't change).
	assert.Less(t, res.PointsAvailable, 0)
}

// Additional unit tests to exercise edge branches in calculatePointsAvailable.
func TestCalculatePointsAvailable_NoRemainingPositions(t *testing.T) {
	// Draft contains A,B,C but finalPositions map claims other players filled all positions
	d := makeDraft([]string{"A", "B", "C"})
	// FinalPositions occupies positions 1..3 but with different player names
	finalPositions := map[string]int{"X": 1, "Y": 2, "Z": 3}
	// Since none of the draft names are in finalPositions, remainder == all draft entries,
	// and remainingPositions after deleting finalPositions is empty -> function should return 0
	pa := calculatePointsAvailable(d, finalPositions, 3)
	assert.Equal(t, 0, pa)
}

func TestCalculatePointsAvailable_NegativeContribution(t *testing.T) {
	// Construct scenario where at least one survivor yields a negative 'additional' value
	// Draft positions 1..5
	d := makeDraft([]string{"A", "B", "C", "D", "E"})
	// finalPositions occupy positions 3,4,5 (with different names), leaving remaining positions 1&2
	finalPositions := map[string]int{"X": 3, "Y": 4, "Z": 5}
	pa := calculatePointsAvailable(d, finalPositions, 5)
	// Manually computed expected value:
	// pos1: pv=5 bestDistance=0 => +5
	// pos2: pv=4 bestDistance=0 => +4
	// pos3: pv=3 bestDistance=1 => +2
	// pos4: pv=2 bestDistance=2 => 0
	// pos5: pv=1 bestDistance=3 => -2
	// total = 5+4+2+0-2 = 9
	assert.Equal(t, 9, pa)
}

// Property tests: determinism, monotonicity, and consistency
func TestProperties_DeterminismAndMonotonicity(t *testing.T) {
	d := makeDraft([]string{"A", "B", "C", "D"})
	// No eliminations -> compute points available
	pa1 := calculatePointsAvailable(d, map[string]int{}, 4)
	pa1b := calculatePointsAvailable(d, map[string]int{}, 4)
	assert.Equal(t, pa1, pa1b, "PointsAvailable should be deterministic for same input")

	// Add an elimination and verify points available generally decreases
	final1 := map[string]int{"D": 4}
	pa2 := calculatePointsAvailable(d, final1, 4)
	assert.GreaterOrEqual(t, pa1, pa2, "PointsAvailable should generally decrease as eliminations occur")
}

// Integration: end-to-end scoring pipeline with a larger draft to ensure
// performance and CLI-like behavior (value display). This is not a strict
// performance benchmark but ensures realistic sizes run quickly.
func TestIntegration_LargerDraft(t *testing.T) {
	names := []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "P12"}
	d := makeDraft(names)
	eliminated := map[string]int{"P10": 10, "P11": 11, "P12": 12}
	f := makeFinal(12, eliminated, "Current")
	res, err := score(d, f)
	assert.NoError(t, err)
	// Ensure values are reasonable and non-nil
	assert.GreaterOrEqual(t, res.Score, 0)
	// Points available should be non-negative in this mid-season scenario
	assert.GreaterOrEqual(t, res.PointsAvailable, 0)
}

// Existing tests that used fixtures are preserved to ensure previous
// expectations still hold (regression protection).
func TestSeason48Regression(t *testing.T) {
	// Load final results
	finalFile, err := os.Open("../../../cli/finals/48.txt")
	assert.NoError(t, err)
	defer finalFile.Close()
	final, err := readDraft(finalFile)
	assert.NoError(t, err)

	// Expected scores map
	expectedScores := map[string]int{
		"Kyle":   129,
		"Lauren": 111,
		"Kenny":  109,
		"Marv":   104,
		"Grant":  103,
		"Kate":   103,
		"Katie":  101,
		"Riley":  100,
		"Mooney": 99,
		"Bryan":  96,
	}

	// Score each draft and verify
	for drafter, expected := range expectedScores {
		draftFile, err := os.Open(fmt.Sprintf("../../../cli/drafts/48/%s.txt", strings.ToLower(drafter)))
		assert.NoError(t, err)
		draft, err := readDraft(draftFile)
		assert.NoError(t, err)
		draftFile.Close()

		res, err := score(draft, final)
		assert.NoError(t, err)
		assert.Equal(t, expected, res.Score, "Score mismatch for %s", drafter)
	}
}

func TestScoreCalculation_FixturesRegression(t *testing.T) {
	testCases := []struct {
		description       string
		haveDraftFilePath string      // filePath to the draft data file
		haveFinalFilePath string      // filePath to the final results data file
		want              ScoreResult // expected score
	}{
		{
			description:       "0_draft 0_final",
			haveDraftFilePath: "../../../cli/test_fixtures/drafts/0/bryan.txt",
			haveFinalFilePath: "../../../cli/test_fixtures/drafts/0/final.txt",
			want: ScoreResult{
				Score:           3,
				PointsAvailable: -4,
			},
		},
		{
			description:       "1_draft 0_final scores to 3",
			haveDraftFilePath: "../../../cli/test_fixtures/drafts/0/bryan.txt",
			haveFinalFilePath: "../../../cli/test_fixtures/drafts/0/final.txt",
			want: ScoreResult{
				Score:           3,
				PointsAvailable: -4,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Open draft data file
			draftFile, err := os.Open(tc.haveDraftFilePath)
			assert.NoError(t, err, "Failed to open draft file")
			defer draftFile.Close()

			// Read draft
			draft, err := readDraft(draftFile)
			assert.NoError(t, err, "Failed to parse draft")

			// Open final results file
			finalFile, err := os.Open(tc.haveFinalFilePath)
			assert.NoError(t, err, "Failed to open final file")
			defer finalFile.Close()

			// Read final results
			final, err := readDraft(finalFile)
			assert.NoError(t, err, "Failed to parse final results")

			// Calculate and assert score
			got, error := score(draft, final)
			assert.NoError(t, error)
			assert.Equal(t, tc.want, got, "Score mismatch")
		})
	}
}
