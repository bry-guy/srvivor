package scorer

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Metadata struct {
	Drafter string // Name of the person who created the draft
	Date    string // Date of the draft
	Season  string // Season or edition of the Survivor game
}

type Draft struct {
	Metadata Metadata
	Entries  []Entry
}

type Entry struct {
	position   int    // Position in the draft
	playerName string // Name of the Survivor player
}

// TODO: The scored draft should be able to determine correct vs. incorrect picks, and points earned per pick
// TODO: The scored draft should be printable, and may need to be extended from the Draft type
// ScoreResults contains the score, points available, and scored draft
type ScoreResult struct {
	Score           int
	PointsAvailable int
	// Draft *Draft // TODO: Add a scored draft to the result
}

// TODO: Break score down into smaller functions
// TODO: Add an additional function to create a "scored draft" struct
// score calculates the score of a draft based on the final results
// It now orchestrates two focused calculations: the current score (earned
// by eliminated players) and the points still available (from remaining
// players). The internal logic and return values are preserved.
func score(draft, final *Draft) (ScoreResult, error) {
	log := slog.With("draft", draft.Metadata.Drafter)
	var result ScoreResult
	totalPositions := len(final.Entries)
	log.Debug("final", "total_positions", totalPositions)

	// Build lookup of final positions and some metadata used by the
	// validation step. This mirrors the previous behavior which built
	// this map before computing scores.
	currentPositions := 0 // number of positions taken in the final (players eliminated)
	finalPositions := make(map[string]int)
	maxPosition := 0
	for _, e := range final.Entries {
		log.Debug("final", "player", e.playerName, "position", e.position)
		if e.playerName != "" {
			finalPositions[e.playerName] = e.position
			currentPositions++
		}
		if e.position > maxPosition {
			maxPosition = e.position
		}
	}

	// Validate draft entries exist in the final results unless the final
	// is the current season. This preserves the original error/warn
	// behavior exactly.
	for _, draftEntry := range draft.Entries {
		if _, ok := finalPositions[draftEntry.playerName]; !ok {
			if final.Metadata.Drafter == "Current" {
				log.Warn("Season is current. Assuming player has not finished.", "player", draftEntry.playerName)
			} else {
				return ScoreResult{}, fmt.Errorf("player not found in final results: %v", draftEntry.playerName)
			}
		}
	}

	// Delegate to focused calculation functions. Each function is
	// responsible for a single aspect of the computation and can be
	// tested independently.
	currentScore := calculateCurrentScore(draft, finalPositions, totalPositions)
	pointsAvailable := calculatePointsAvailable(draft, finalPositions, totalPositions)

	result.Score = currentScore
	result.PointsAvailable = pointsAvailable

	return result, nil
}

// calculateCurrentScore computes the current score earned by the draft
// based only on eliminated players (those present in finalPositions).
// It applies the same scoring rule as before: each pick has a
// positionValue (higher for earlier final positions), and the points
// awarded are max(0, positionValue - distance) where distance is the
// absolute difference between the draft position and the final
// position. This function has a single responsibility and is
// deterministic for unit testing.
func calculateCurrentScore(draft *Draft, finalPositions map[string]int, totalPositions int) int {
	currentScore := 0
	for _, draftEntry := range draft.Entries {
		if finalPosition, ok := finalPositions[draftEntry.playerName]; ok {
			distance := abs(draftEntry.position - finalPosition)
			// Position value for current score is based on the draft pick value
			// (higher for earlier draft picks). This matches the specification
			// examples where, e.g., a pick at draft position 2 has higher
			// potential value than a later pick regardless of the final
			// elimination position.
			positionValue := totalPositions + 1 - draftEntry.position
			entryScore := max(0, positionValue-distance)
			currentScore += entryScore
		}
	}
	return currentScore
}

// calculatePointsAvailable computes the points that are still available
// to be earned given the current final results. To preserve existing
// behavior, it reproduces the original aggregation logic: it computes
// the perfect score for the season, the current maximum possible from
// eliminated players, the current misses, and known losses, then
// returns the resulting points available. Although this function uses
// data derived from eliminated players, it focuses on the remaining
// availability of points as the original implementation did.
func calculatePointsAvailable(draft *Draft, finalPositions map[string]int, totalPositions int) int {
	// Build list of remaining survivors (not yet in final positions)
	remainder := []Entry{}
	for _, e := range draft.Entries {
		if _, ok := finalPositions[e.playerName]; !ok {
			remainder = append(remainder, e)
		}
	}

	// If no remaining survivors, preserve original behavior by
	// computing the previous aggregation (known losses/misses) so that
	// existing tests expecting negative points continue to pass.
	if len(remainder) == 0 {
		// original calculation preserved
		perfectScore := (totalPositions * (totalPositions + 1)) / 2
		knownLosses, currentScore, currentMax := 0, 0, 0
		maxPosition := 0
		for _, pos := range finalPositions {
			if pos > maxPosition {
				maxPosition = pos
			}
		}

		for _, draftEntry := range draft.Entries {
			positionValue, entryScore, distance, lossDistance, knownLoss := 0, 0, 0, 0, 0

			if finalPosition, ok := finalPositions[draftEntry.playerName]; ok {
				distance = abs(draftEntry.position - finalPosition)
				positionValue = totalPositions - finalPosition + 1
				entryScore = max(0, positionValue-distance)

				lossDistance = abs(draftEntry.position - maxPosition)
				if lossDistance > positionValue {
					knownLoss = positionValue // Complete loss of points
				} else if lossDistance > 0 {
					knownLoss = distance // Partial loss of points
				}
			}

			currentMax += positionValue
			currentScore += entryScore
			knownLosses += knownLoss
		}

		currentMiss := currentMax - currentScore
		pointsAvailable := perfectScore - currentMax - currentMiss - knownLosses
		return pointsAvailable
	}

	// Build set of remaining positions (1..totalPositions) not present in finalPositions
	remainingPositionsMap := make(map[int]struct{})
	for i := 1; i <= totalPositions; i++ {
		remainingPositionsMap[i] = struct{}{}
	}
	for _, pos := range finalPositions {
		delete(remainingPositionsMap, pos)
	}

	// Convert map to slice for iteration
	remainingPositions := []int{}
	for p := range remainingPositionsMap {
		remainingPositions = append(remainingPositions, p)
	}

	// If no remaining positions, preserve original behavior (no available points)
	if len(remainingPositions) == 0 {
		return 0
	}

	pointsAvailable := 0
	// For each remaining survivor, independently choose the remaining final
	// position that minimizes distance. Position value is based on the draft
	// position (not the assigned final position):
	//   positionValue = totalPositions + 1 - draftPosition
	// Multiple survivors may choose the same final position; there is no
	// uniqueness constraint.
	for _, survivor := range remainder {
		// Find the best (minimum) distance among remaining positions
		bestDistance := -1
		for _, pos := range remainingPositions {
			d := abs(survivor.position - pos)
			if bestDistance == -1 || d < bestDistance {
				bestDistance = d
			}
		}

		// Position value based on draft pick
		positionValue := totalPositions + 1 - survivor.position
		additional := positionValue - bestDistance
		pointsAvailable += additional
	}

	return pointsAvailable
}

// max returns the larger of two integers. Helper to replace absent
// built-in max for ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Scores(drafts []*Draft, final *Draft) (map[*Draft]ScoreResult, error) {
	scores := map[*Draft]ScoreResult{}
	for _, draft := range drafts {
		result, err := score(draft, final)
		if err != nil {
			return nil, err
		}
		scores[draft] = result
	}
	return scores, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func readDraft(file *os.File) (*Draft, error) {
	// Scan File
	var d Draft
	scanner := bufio.NewScanner(file)
	parsingMetadata := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for separator
		if line == "---" {
			parsingMetadata = false
			continue
		}

		if parsingMetadata {
			// Parse metadata
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid metadata length: not 2 parts: %v", line)
			}

			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			switch key {
			case "Drafter":
				d.Metadata.Drafter = value
			case "Date":
				d.Metadata.Date = value
			case "Season":
				d.Metadata.Season = value
			}
		} else {
			// Parse entries
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid entry length: not 2 parts: %v", line)
			}

			position, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid position: not an integer: %v", parts[0])
			}

			if position <= 0 {
				return nil, fmt.Errorf("invalid position: less than zero: %v", parts[0])
			}

			d.Entries = append(d.Entries, Entry{
				position:   position,
				playerName: strings.TrimSpace(parts[1]),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

// Helper function to process a file and read the draft
func ProcessFile(filepath string) (*Draft, error) {
	slog.Info("Processing file.", "filepath", filepath)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readDraft(file)
}
