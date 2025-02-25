package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"log/slog"

	fpath "path/filepath"

	"github.com/bry-guy/srvivor/internal/config"
	"github.com/bry-guy/srvivor/internal/log"
	"github.com/spf13/cobra"
)

func main() {
	cfg, err := config.Validate()
	if err != nil {
		fmt.Printf("Invalid environment configuration. %v\n", err)
		os.Exit(1)
	}

	log := log.NewLogger(cfg)

	var rootCmd = &cobra.Command{Use: "srvivor"}

	var cmdScore = &cobra.Command{
		Use:   "score [-f --file [filepath] | -d --drafters [drafters]] -s --season [season]",
		Short: "Calculate the score for a Survivor drafts",
		Long:  `Calculate and display the total score for Survivor drafts for a particular season.`,
		Run: func(cmd *cobra.Command, args []string) {
			drafters, err := cmd.Flags().GetStringSlice("drafters")
			if err != nil {
				log.Error("parsing drafters flag", "error", err)
				os.Exit(1)
			}

			filepath, err := cmd.Flags().GetString("file")
			if err != nil {
				log.Error("parsing file flag", "error", err)
				os.Exit(1)
			}

			if (filepath != "" && len(drafters) > 0) || (filepath == "" && len(drafters) == 0) {
				log.Error("You must specify either a file or drafters, but not both")
				os.Exit(1)
			}

			// command season
			season, err := cmd.Flags().GetInt("season")
			if err != nil || season < 1 {
				log.Error("You must specify a valid season.")
				os.Exit(1)
			}

			log.Debug("drafters: ", "drafters", drafters)

			if len(drafters) == 1 && drafters[0] == "*" {
				// Wildcard logic
				draftFilepaths, err := fpath.Glob(fmt.Sprintf("./drafts/%d/*.txt", season))
				log.Debug("draftFilepaths: ", "draftFilepaths", draftFilepaths)
				if err != nil {
					log.Error("Unable to list draft files.", "error", err)
					os.Exit(1)
				}

				drafters = []string{}
				for _, draftFilepath := range draftFilepaths {
					// drafter should be the name of the txt file, with no directory, stripped of the .txt extension
					drafter := strings.TrimSuffix(fpath.Base(draftFilepath), ".txt")
					log.Debug("drafter: ", "drafter", drafter)
					drafters = append(drafters, drafter)
				}
			}

			wd, err := os.Getwd()
			if err != nil {
				log.Error("Unable to get working directory.", "error", err)
				os.Exit(1)
			}

			log.Debug("workdir: ", "workdir", wd)

			var drafts []*draft
			if filepath != "" {
				// Single file mode
				draft, err := processFile(filepath, log)
				if err != nil {
					log.Error("Failed to process file.", "error", err)
					os.Exit(1)
				}
				drafts = append(drafts, draft)
			} else {
				// Drafter mode
				for _, drafter := range drafters {
					filepath := fmt.Sprintf("./drafts/%d/%s.txt", season, drafter)
					draft, err := processFile(filepath, log)
					if err != nil {
						log.Error("Failed to process file.", "error", err)
						continue
					}
					drafts = append(drafts, draft)
				}
			}

			finalFilepath := fmt.Sprintf("./finals/%d.txt", season)
			final, err := processFile(finalFilepath, log)
			if err != nil {
				log.Error("Failed to process final file.", "error", err)
				os.Exit(1)
			}

			log.Info("Calculating score for each draft.")
			scores, err := scores(log, drafts, final)
			if err != nil {
				log.Error("Failed to score drafts.", "error", err)
				os.Exit(1)
			}

			for _, d := range drafts {
				result := scores[d]
				fmt.Printf("%s: %d (points available: %d)\n", d.metadata.drafter, result.score, result.pointsAvailable)
			}
		},
	}

	cmdScore.Flags().StringP("file", "f", "", "Input file containing the draft")
	cmdScore.Flags().StringSliceP("drafters", "d", []string{}, "Drafter name(s) to lookup the draft")
	cmdScore.Flags().IntP("season", "s", 0, "Season number of the Survivor game")
	cmdScore.MarkFlagRequired("season")

	rootCmd.AddCommand(cmdScore)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type metadata struct {
	drafter string // Name of the person who created the draft
	date    string // Date of the draft
	season  string // Season or edition of the Survivor game
	hash    string // A unique hash to identify this particular draft
}

type draft struct {
	metadata metadata
	entries  []entry
}

type entry struct {
	position   int    // Position in the draft
	playerName string // Name of the Survivor player
}

type scoreResult struct {
	score           int
	pointsAvailable int
}

func score(log *slog.Logger, draft, final *draft) (scoreResult, error) {
	log = log.With("draft", draft.metadata.drafter)
	var result scoreResult
	totalPositions := len(final.entries)
	log.Debug("final", "total_positions", totalPositions)

	// Calculate perfect score possible (n + (n-1) + ... + 1)
	perfectScore := (totalPositions * (totalPositions + 1)) / 2

	// Map to store the final positions of the players for easy lookup
	currentPositions := 0 // number of positions taken in the final (players eliminated)
	finalPositions := make(map[string]int)
	maxPosition := 0
	for _, e := range final.entries {
		finalPositions[e.playerName] = e.position
		log.Debug("final", "player", e.playerName, "position", e.position)
		if e.playerName != "" {
			currentPositions++
		} else {
			if e.position > maxPosition {
				maxPosition = e.position
			}
		}
	}

	knownLosses := 0
	currentScore := 0
	currentMax := (currentPositions * (currentPositions + 1)) / 2

	for _, draftEntry := range draft.entries {
		// Calculate position value (inverse of position)
		positionValue := totalPositions - draftEntry.position + 1

		finalPosition, ok := finalPositions[draftEntry.playerName]
		if !ok {
			if final.metadata.drafter == "Current" {
				log.Warn("Season is current. Assuming player has not finished.", "player", draftEntry.playerName)
			} else {
				return scoreResult{}, fmt.Errorf("Player not found in final results: %v", draftEntry.playerName)
			}
		}

		// Calculate position distance and entry score
		distance := abs(draftEntry.position - finalPosition)
		entryScore := max(0, positionValue-distance)

		currentScore += entryScore

		log.Debug("score",
			"player", draftEntry.playerName,
			"final_position", finalPosition,
			"draft_position", draftEntry.position,
			"position_val", positionValue,
			"distance", distance,
			"points", entryScore,
			"currentScore", currentScore,
		)

		// Calculate known losses
		knownLoss := 0
		lossDistance := 0
		if !ok {
			lossDistance = abs(draftEntry.position - maxPosition)
			if lossDistance > positionValue {
				knownLoss = positionValue // Complete loss of points
			} else if lossDistance > 0 {
				knownLoss = distance // Partial loss of points
			}
			knownLosses += knownLoss

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

	}

	currentMisses := currentMax - currentScore

	pointsAvailable := perfectScore - currentMax - currentMisses - knownLosses

	log.Debug("pointsAvailable",
		"pointsAvailable", pointsAvailable,
		"perfectScore", perfectScore,
		"currentMisses", currentMisses,
		"currentMax", currentMax,
		"knownLosses", knownLosses,
	)

	result.score = currentScore
	result.pointsAvailable = max(0, pointsAvailable)

	return result, nil
}

func scores(log *slog.Logger, drafts []*draft, final *draft) (map[*draft]scoreResult, error) {
	scores := map[*draft]scoreResult{}
	for _, draft := range drafts {
		result, err := score(log, draft, final)
		if err != nil {
			return nil, err
		}
		scores[draft] = result
	}
	return scores, nil
}

// func score(log *slog.Logger, draft, final *draft) (int, error) {
//     totalScore := 0
//     totalPositions := len(final.entries)
// 	log.Debug("final", "total_positions", totalPositions)

//     // Map to store the final positions of the players for easy lookup
//     finalPositions := make(map[string]int)
//     for _, e := range final.entries {
//         finalPositions[e.playerName] = e.position
// 		log.Debug("final", "player", e.playerName, "position", e.position)
//     }

//     for _, draftEntry := range draft.entries {
//         // Get the final position of the player
//         finalPosition, ok := finalPositions[draftEntry.playerName]
//         if !ok {
//             log.Warn("Player not found in final results", "player", draftEntry.playerName)
//             if final.metadata.drafter == "Current" {
//                 log.Warn("Season is current. Assuming player has not finished.", "final", final)
//                 continue
//             } else {
//                 return 0, fmt.Errorf("Player not found in final results: %v", draftEntry.playerName)
//             }
//         }

// 		log.Debug("draft", "player", draftEntry.playerName, "position", finalPosition)

//         // Calculate position value (inverse of position)
//         positionValue := totalPositions - draftEntry.position + 1

//         // Calculate position distance
//         distance := abs(draftEntry.position - finalPosition)

//         // Calculate entry score (minimum 0)
//         entryScore := max(0, positionValue-distance)

// 		log.Debug("score",
// 				"player",
// 				draftEntry.playerName,
// 				"final_position",
// 				finalPosition,
// 				"draft_position",
// 				draftEntry.position,
// 				"position_val",
// 				positionValue,
// 				"distance",
// 				distance,
// 				"points",
// 				entryScore,
// 		)

//         totalScore += entryScore
// 		log.Debug("totalScore", "points", totalScore)
//     }

//     return totalScore, nil
// }

// func scores(log *slog.Logger, drafts []*draft, final *draft) (map[*draft]int, error) {
// 	scores := map[*draft]int{}
// 	for _, draft := range drafts {
// 		score, err := score(log, draft, final)
// 		if err != nil {
// 			return nil, err
// 		}
// 		scores[draft] = score
// 	}
// 	return scores, nil
// }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func readDraft(file *os.File) (*draft, error) {
	// Scan File
	var d draft
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
				d.metadata.drafter = value
			case "Date":
				d.metadata.date = value
			case "Season":
				d.metadata.season = value
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

			d.entries = append(d.entries, entry{
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
func processFile(filepath string, log *slog.Logger) (*draft, error) {
	log.Info("Processing file.", "filepath", filepath)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readDraft(file)
}
