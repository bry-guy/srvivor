package matcher

import (
	"regexp"
	"strings"
)

var extraSpacesRegex = regexp.MustCompile(`\s+`)

// Normalize normalizes a name string by converting to lowercase,
// trimming whitespace, removing quotes, and collapsing multiple spaces into single spaces
func Normalize(name string) string {
	// Trim whitespace
	name = strings.TrimSpace(name)
	// Remove quotes
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "\u201c", "")
	name = strings.ReplaceAll(name, "\u201d", "")
	// Convert to lowercase
	name = strings.ToLower(name)
	// Collapse multiple spaces into single space
	name = extraSpacesRegex.ReplaceAllString(name, " ")
	return name
}
