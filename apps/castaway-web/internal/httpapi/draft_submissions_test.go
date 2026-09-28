package httpapi

import "testing"

func TestPickDraftTribeStaysBalanced(t *testing.T) {
	tribes := []string{"Savu", "Toka"}
	for seed := 0; seed < 50; seed++ {
		counts := map[string]int{}
		n := seed
		for i := 0; i < 16; i++ {
			n = (n*31 + 7) % 1000003
			counts[pickDraftTribe(tribes, counts, func(k int) int { return n % k })]++
			if d := counts["Savu"] - counts["Toka"]; d > 1 || d < -1 {
				t.Fatalf("unbalanced %v", counts)
			}
		}
	}
}
