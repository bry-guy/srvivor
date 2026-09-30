package castawordle

import "testing"

func TestSCOWLEnglishDictionary(t *testing.T) {
	if len(dictionary) != 59212 {
		t.Fatalf("dictionary has %d words, want 59212", len(dictionary))
	}
	for _, word := range []string{"SWADDLE", "SWADDLED", "SWADDLES", "COLOR", "COLOUR", "TORCH", "OUTCAST", "SURVIVOR"} {
		if !ValidWord(word) {
			t.Errorf("ValidWord(%q) = false, want true", word)
		}
	}
	for _, word := range []string{"", "AAA", "AAAAAAAAA", "JEFF", "BAD WORD", "NAÏVE", "TORCH!"} {
		if ValidWord(word) {
			t.Errorf("ValidWord(%q) = true, want false", word)
		}
	}
}
