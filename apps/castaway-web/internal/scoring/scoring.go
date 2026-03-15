package scoring

import "sort"

type DraftPick struct {
	Position     int
	ContestantID string
}

type LeaderboardEntry struct {
	ParticipantID   string
	ParticipantName string
	Score           int
	DraftPoints     int
	BonusPoints     int
	TotalPoints     int
	PointsAvailable int
}

func CalculateLeaderboard(
	totalPositions int,
	participantNames map[string]string,
	draftsByParticipant map[string][]DraftPick,
	finalPositions map[string]int,
	visibleBonusByParticipant map[string]int,
) []LeaderboardEntry {
	entries := make([]LeaderboardEntry, 0, len(participantNames))
	for participantID, participantName := range participantNames {
		draft := draftsByParticipant[participantID]
		draftPoints := calculateCurrentScore(draft, finalPositions, totalPositions)
		bonusPoints := visibleBonusByParticipant[participantID]
		totalPoints := draftPoints + bonusPoints
		entry := LeaderboardEntry{
			ParticipantID:   participantID,
			ParticipantName: participantName,
			Score:           totalPoints,
			DraftPoints:     draftPoints,
			BonusPoints:     bonusPoints,
			TotalPoints:     totalPoints,
			PointsAvailable: calculatePointsAvailable(draft, finalPositions, totalPositions),
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalPoints != entries[j].TotalPoints {
			return entries[i].TotalPoints > entries[j].TotalPoints
		}
		return entries[i].ParticipantName < entries[j].ParticipantName
	})

	return entries
}

func calculateCurrentScore(draft []DraftPick, finalPositions map[string]int, totalPositions int) int {
	currentScore := 0
	for _, draftEntry := range draft {
		if finalPosition, ok := finalPositions[draftEntry.ContestantID]; ok {
			distance := abs(draftEntry.Position - finalPosition)
			positionValue := totalPositions - finalPosition + 1
			entryScore := max(0, positionValue-distance)
			currentScore += entryScore
		}
	}
	return currentScore
}

func calculatePointsAvailable(draft []DraftPick, finalPositions map[string]int, totalPositions int) int {
	remainder := make([]DraftPick, 0)
	for _, pick := range draft {
		if _, ok := finalPositions[pick.ContestantID]; !ok {
			remainder = append(remainder, pick)
		}
	}

	if len(remainder) == 0 {
		perfectScore := (totalPositions * (totalPositions + 1)) / 2
		knownLosses, currentScore, currentMax := 0, 0, 0
		maxPosition := 0
		for _, pos := range finalPositions {
			if pos > maxPosition {
				maxPosition = pos
			}
		}

		for _, draftEntry := range draft {
			positionValue, entryScore, distance, lossDistance, knownLoss := 0, 0, 0, 0, 0
			if finalPosition, ok := finalPositions[draftEntry.ContestantID]; ok {
				distance = abs(draftEntry.Position - finalPosition)
				positionValue = totalPositions - finalPosition + 1
				entryScore = max(0, positionValue-distance)
				lossDistance = abs(draftEntry.Position - maxPosition)
				if lossDistance > positionValue {
					knownLoss = positionValue
				} else if lossDistance > 0 {
					knownLoss = distance
				}
			}

			currentMax += positionValue
			currentScore += entryScore
			knownLosses += knownLoss
		}

		currentMiss := currentMax - currentScore
		return perfectScore - currentMax - currentMiss - knownLosses
	}

	remainingPositionsMap := make(map[int]struct{}, totalPositions)
	for i := 1; i <= totalPositions; i++ {
		remainingPositionsMap[i] = struct{}{}
	}
	for _, pos := range finalPositions {
		delete(remainingPositionsMap, pos)
	}

	remainingPositions := make([]int, 0, len(remainingPositionsMap))
	for position := range remainingPositionsMap {
		remainingPositions = append(remainingPositions, position)
	}
	if len(remainingPositions) == 0 {
		return 0
	}

	pointsAvailable := 0
	for _, survivor := range remainder {
		bestDistance := -1
		for _, pos := range remainingPositions {
			distance := abs(survivor.Position - pos)
			if bestDistance == -1 || distance < bestDistance {
				bestDistance = distance
			}
		}
		positionValue := totalPositions + 1 - survivor.Position
		pointsAvailable += positionValue - bestDistance
	}

	return pointsAvailable
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
