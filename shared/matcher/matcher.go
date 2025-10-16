package matcher

import (
	"fmt"
	"math"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/bry-guy/srvivor/internal/roster"
)

const minimumThreshold = 0.70

// MatchResult represents the result of matching a contestant name
type MatchResult struct {
	Contestant *roster.Contestant
	MatchType  string
	Score      float64
}

// MatchContestant attempts to match an input name against a season roster
func MatchContestant(inputName string, seasonRoster *roster.SeasonRoster) (*MatchResult, error) {
	normalizedInput := Normalize(inputName)
	candidates := []MatchResult{}

	for i := range seasonRoster.Contestants {
		contestant := &seasonRoster.Contestants[i]
		score := calculateMatchScore(normalizedInput, contestant)
		if score > 0 {
			candidates = append(candidates, MatchResult{
				Contestant: contestant,
				Score:      score,
			})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no match found for '%s'", inputName)
	}

	// Sort by score descending (simple sort since small list)
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Score > candidates[i].Score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	best := candidates[0]

	// Require minimum confidence threshold
	if best.Score < minimumThreshold {
		return nil, fmt.Errorf("no match found above threshold for '%s' (best score: %.2f)", inputName, best.Score)
	}

	// Determine match type
	matchType := determineMatchType(normalizedInput, best.Contestant, best.Score)
	best.MatchType = matchType

	return &best, nil
}

// calculateMatchScore calculates the match score between normalized input and contestant
func calculateMatchScore(input string, contestant *roster.Contestant) float64 {
	maxScore := 0.0

	// Exact match to canonical name
	if input == Normalize(contestant.CanonicalName) {
		return 1.0
	}

	// Exact match to nickname
	if contestant.Nickname != "" && input == Normalize(contestant.Nickname) {
		return 0.95
	}

	// Match if any word in input matches canonical or nickname exactly
	words := strings.Split(input, " ")
	canonicalNorm := Normalize(contestant.CanonicalName)
	nicknameNorm := ""
	if contestant.Nickname != "" {
		nicknameNorm = Normalize(contestant.Nickname)
	}
	for _, word := range words {
		if word == canonicalNorm || (nicknameNorm != "" && word == nicknameNorm) {
			maxScore = math.Max(maxScore, 0.95)
		}
	}

	// Exact match to nickname
	if contestant.Nickname != "" && input == Normalize(contestant.Nickname) {
		return 0.95
	}

	// Exact match to first name
	if input == Normalize(contestant.FirstName) {
		maxScore = math.Max(maxScore, 0.85)
	}

	// Exact match to last name
	if contestant.LastName != "" && input == Normalize(contestant.LastName) {
		maxScore = math.Max(maxScore, 0.85)
	}

	// Match to "FirstName LastName" or "LastName FirstName"
	if contestant.LastName != "" {
		fullName := Normalize(contestant.FirstName + " " + contestant.LastName)
		reverseName := Normalize(contestant.LastName + " " + contestant.FirstName)
		if input == fullName || input == reverseName {
			maxScore = math.Max(maxScore, 0.90)
		}
		// Fuzzy match to full name
		fullSimilarity := fuzzyMatch(input, fullName)
		maxScore = math.Max(maxScore, fullSimilarity*0.9)
		reverseSimilarity := fuzzyMatch(input, reverseName)
		maxScore = math.Max(maxScore, reverseSimilarity*0.9)
	}

	// Fuzzy matching with Levenshtein distance
	canonicalSimilarity := fuzzyMatch(input, Normalize(contestant.CanonicalName))
	maxScore = math.Max(maxScore, canonicalSimilarity*1.0)

	if contestant.Nickname != "" {
		nicknameSimilarity := fuzzyMatch(input, Normalize(contestant.Nickname))
		maxScore = math.Max(maxScore, nicknameSimilarity*0.9)
	}

	firstSimilarity := fuzzyMatch(input, Normalize(contestant.FirstName))
	maxScore = math.Max(maxScore, firstSimilarity*0.8)

	if contestant.LastName != "" {
		lastSimilarity := fuzzyMatch(input, Normalize(contestant.LastName))
		maxScore = math.Max(maxScore, lastSimilarity*0.8)
	}

	return maxScore
}

// fuzzyMatch calculates similarity using Levenshtein distance
func fuzzyMatch(s1, s2 string) float64 {
	distance := levenshtein.ComputeDistance(s1, s2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - (float64(distance) / maxLen)
}

// determineMatchType determines the type of match based on score
func determineMatchType(input string, contestant *roster.Contestant, score float64) string {
	if score == 1.0 {
		return "exact match"
	}
	if score >= 0.95 {
		return "nickname match"
	}
	if score >= 0.85 {
		return "name component match"
	}
	if score >= 0.7 {
		return "fuzzy match"
	}
	return "low confidence match"
}
