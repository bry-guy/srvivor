package castawordle

import (
	"embed"
	"strings"
)

const GuessLimit = 6
const DictionaryVersion = "scowl-1e5b7d3a72f47a71da5d28686c1dd4b397178485-70-ABZ-1"

//go:embed data/words.txt data/LICENSE
var dictionaryFiles embed.FS

var dictionary = func() map[string]bool {
	data, err := dictionaryFiles.ReadFile("data/words.txt")
	if err != nil {
		panic(err)
	}
	words := make(map[string]bool)
	for _, word := range strings.Fields(string(data)) {
		words[word] = true
	}
	return words
}()

func Normalize(word string) string {
	return strings.ToUpper(strings.TrimSpace(word))
}

func ValidWord(word string) bool {
	return dictionary[word]
}

func Feedback(answer, guess string) []string {
	feedback := make([]string, len(answer))
	var remaining [26]int
	for i := range answer {
		feedback[i] = "absent"
		if answer[i] == guess[i] {
			feedback[i] = "correct"
		} else {
			remaining[answer[i]-'A']++
		}
	}
	for i := range guess {
		if feedback[i] != "correct" && remaining[guess[i]-'A'] > 0 {
			feedback[i] = "present"
			remaining[guess[i]-'A']--
		}
	}
	return feedback
}
