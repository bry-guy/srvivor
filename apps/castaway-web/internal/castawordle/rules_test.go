package castawordle

import (
	"reflect"
	"testing"
)

func TestRules(t *testing.T) {
	for _, word := range []string{"TORCH", "OUTCAST", "SURVIVOR"} {
		if !ValidWord(Normalize(" " + word + " ")) {
			t.Fatalf("valid word rejected: %s", word)
		}
	}
	for _, word := range []string{"", "ZZZZZ", "A-B-C", "NAÏVE", "TORCHES!!"} {
		if ValidWord(Normalize(word)) {
			t.Fatalf("invalid word accepted: %s", word)
		}
	}
	for _, tc := range []struct {
		answer, guess string
		want          []string
	}{
		{"CACAO", "COCOA", []string{"correct", "present", "correct", "absent", "present"}},
		{"SHEEP", "PEEPS", []string{"present", "present", "correct", "absent", "present"}},
		{"OUTCAST", "OUTCAST", []string{"correct", "correct", "correct", "correct", "correct", "correct", "correct"}},
		{"SURVIVOR", "SURVIVOR", []string{"correct", "correct", "correct", "correct", "correct", "correct", "correct", "correct"}},
	} {
		if got := Feedback(tc.answer, tc.guess); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s/%s: got %v, want %v", tc.answer, tc.guess, got, tc.want)
		}
	}
}
