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
func score(draft, final *Draft) (ScoreResult, error) {
	log := slog.With("draft", draft.Metadata.Drafter)
	var result ScoreResult
	totalPositions := len(final.Entries)
	log.Debug("final", "total_positions", totalPositions)

	// Calculate perfect score possible (n + (n-1) + ... + 1)
	perfectScore := (totalPositions * (totalPositions + 1)) / 2

	// Map to store the final positions of the players for easy lookup
	currentPositions := 0 // number of positions taken in the final (players eliminated)
	finalPositions := make(map[string]int)
	maxPosition := 0
	for _, e := range final.Entries {
		finalPositions[e.playerName] = e.position
		log.Debug("final", "player", e.playerName, "position", e.position)
		if e.playerName != "" {
			currentPositions++
		}
		if e.position > maxPosition {
			maxPosition = e.position
		}
	}

	knownLosses, currentScore, currentMax := 0, 0, 0

	for _, draftEntry := range draft.Entries {
		positionValue, entryScore, distance, lossDistance, knownLoss := 0, 0, 0, 0, 0

		finalPosition, ok := finalPositions[draftEntry.playerName]
		if ok {
			distance = abs(draftEntry.position - finalPosition)
			positionValue = totalPositions - finalPosition + 1
			entryScore = max(0, positionValue-distance)

			lossDistance = abs(draftEntry.position - maxPosition)
			if lossDistance > positionValue {
				knownLoss = positionValue // Complete loss of points
			} else if lossDistance > 0 {
				knownLoss = distance // Partial loss of points
			}
		} else {
			if final.Metadata.Drafter == "Current" {
				log.Warn("Season is current. Assuming player has not finished.", "player", draftEntry.playerName)
			} else {
				return ScoreResult{}, fmt.Errorf("player not found in final results: %v", draftEntry.playerName)
			}
		}

		currentMax += positionValue
		currentScore += entryScore
		knownLosses += knownLoss

		log.Debug("score",
			"player", draftEntry.playerName,
			"final_position", finalPosition,
			"draft_position", draftEntry.position,
			"position_val", positionValue,
			"distance", distance,
			"points", entryScore,
			"currentScore", currentScore,
		)
		log.Debug("loss",
			"player", draftEntry.playerName,
			"draft_position", draftEntry.position,
			"max_position", maxPosition,
			"position_val", positionValue,
			"lossDistance", lossDistance,
			"knownLoss", knownLoss,
			"knownLosses", knownLosses,
		)
	}

	// currentMax := (currentPositions * (currentPositions + 1)) / 2
	currentMiss := currentMax - currentScore
	pointsAvailable := perfectScore - currentMax - currentMiss - knownLosses

	log.Debug("points_available",
		"points_available", pointsAvailable,
		"perfect_score", perfectScore,
		"known_losses", knownLosses,
		"current_positions", currentPositions,
		"current_max", currentMax,
		"current_miss", currentMiss,
		"current_max", currentMax,
	)

	result.Score = currentScore
	result.PointsAvailable = max(0, pointsAvailable)

	return result, nil
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
