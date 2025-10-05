package main

import (
	"fmt"
	"log"

	"github.com/bry-guy/srvivor/internal/scorer"
)

func main() {
	// Test Week 4 example from specification
	draft, err := scorer.ProcessFile("test_week4_draft.txt")
	if err != nil {
		log.Fatal("Error processing draft:", err)
	}

	final, err := scorer.ProcessFile("test_week4_final.txt")
	if err != nil {
		log.Fatal("Error processing final:", err)
	}

	result, err := scorer.Scores([]*scorer.Draft{draft}, final)
	if err != nil {
		log.Fatal("Error calculating score:", err)
	}

	score := result[draft]
	fmt.Printf("Current Implementation Results:\n")
	fmt.Printf("Current Score: %d\n", score.Score)
	fmt.Printf("Points Available: %d\n", score.PointsAvailable)
	fmt.Printf("Total: %d\n", score.Score+score.PointsAvailable)

	fmt.Printf("\nExpected from Specification:\n")
	fmt.Printf("Current Score: 8\n")
	fmt.Printf("Points Available: 13\n")
	fmt.Printf("Total: 21\n")
}
