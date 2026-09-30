package castawordle

import (
	"embed"
	"strings"
)

const GuessLimit = 6
const DictionaryVersion = "cmudict-74790861f652b15e4ac49015a90074ad62a27690"

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
