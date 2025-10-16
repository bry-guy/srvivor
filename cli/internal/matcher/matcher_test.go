package matcher

import (
	"testing"

	"github.com/bry-guy/srvivor/internal/roster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Hello World  ", "hello world"},
		{"MC", "mc"},
		{"Sophie", "sophie"},
		{"Kristina", "kristina"},
		{"", ""},
		{"  multiple   spaces  ", "multiple spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchContestant(t *testing.T) {
	// Create a test roster similar to season 49
	testRoster := &roster.SeasonRoster{
		Season: 49,
		Contestants: []roster.Contestant{
			{CanonicalName: "Sophie S", FirstName: "Sophie", LastName: "Stevens", Nickname: ""},
			{CanonicalName: "Sophi B", FirstName: "Sophi", LastName: "Briggs", Nickname: ""},
			{CanonicalName: "Michelle", FirstName: "Michelle", LastName: "", Nickname: "MC"},
			{CanonicalName: "Kristen", FirstName: "Kristen", LastName: "", Nickname: ""},
		},
	}

	tests := []struct {
		name        string
		input       string
		expected    *roster.Contestant
		matchType   string
		expectError bool
	}{
		{
			name:      "exact canonical match",
			input:     "Sophie S",
			expected:  &testRoster.Contestants[0],
			matchType: "exact match",
		},
		{
			name:      "nickname match",
			input:     "MC",
			expected:  &testRoster.Contestants[2],
			matchType: "nickname match",
		},
		{
			name:      "first name match Sophie",
			input:     "Sophie",
			expected:  &testRoster.Contestants[0],
			matchType: "name component match",
		},
		{
			name:      "first name match Sophi",
			input:     "Sophi",
			expected:  &testRoster.Contestants[1],
			matchType: "name component match",
		},
		{
			name:      "fuzzy match Kristina to Kristen",
			input:     "Kristina",
			expected:  &testRoster.Contestants[3],
			matchType: "fuzzy match",
		},
		{
			name:        "no match",
			input:       "Unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MatchContestant(tt.input, testRoster)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result.Contestant)
			assert.Equal(t, tt.matchType, result.MatchType)
			assert.GreaterOrEqual(t, result.Score, minimumThreshold)
		})
	}
}

func TestCalculateMatchScore(t *testing.T) {
	contestant := &roster.Contestant{
		CanonicalName: "Sophie S",
		FirstName:     "Sophie",
		LastName:      "Stevens",
		Nickname:      "",
	}

	tests := []struct {
		input    string
		expected float64
	}{
		{"sophie s", 1.0},       // exact canonical
		{"sophie", 0.85},        // first name
		{"stevens", 0.85},       // last name
		{"sophie stevens", 0.9}, // full name
		{"stevens sophie", 0.9}, // reverse full name
		{"sophi", 0.67},         // fuzzy on first name
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			score := calculateMatchScore(tt.input, contestant)
			assert.InDelta(t, tt.expected, score, 0.01)
		})
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected float64
	}{
		{"kristen", "kristina", 0.75}, // 6/8 similarity (distance 2)
		{"sophie", "sophi", 0.833},    // 5/6 similarity
		{"exact", "exact", 1.0},
		{"", "", 1.0},
		{"a", "b", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			result := fuzzyMatch(tt.s1, tt.s2)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestDetermineMatchType(t *testing.T) {
	contestant := &roster.Contestant{CanonicalName: "Test"}

	tests := []struct {
		score    float64
		expected string
	}{
		{1.0, "exact match"},
		{0.96, "nickname match"},
		{0.90, "name component match"},
		{0.80, "fuzzy match"},
		{0.60, "low confidence match"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := determineMatchType("test", contestant, tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkMatchContestant(b *testing.B) {
	// Create a test roster
	testRoster := &roster.SeasonRoster{
		Season: 49,
		Contestants: []roster.Contestant{
			{CanonicalName: "Sophie S", FirstName: "Sophie", LastName: "Stevens", Nickname: ""},
			{CanonicalName: "Sophi B", FirstName: "Sophi", LastName: "Briggs", Nickname: ""},
			{CanonicalName: "Michelle", FirstName: "Michelle", LastName: "", Nickname: "MC"},
			{CanonicalName: "Kristen", FirstName: "Kristen", LastName: "", Nickname: ""},
		},
	}

	testNames := []string{"Sophie", "Sophi", "MC", "Kristina", "Unknown"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range testNames {
			MatchContestant(name, testRoster)
		}
	}
}
