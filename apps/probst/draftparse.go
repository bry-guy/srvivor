package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type contestant struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	aliases []string
}

// Pick is one line of a draft message and what it resolved to.
type pick struct {
	Raw        string
	Number     int // 0 when the line has no number
	Contestant *contestant
	Method     string // exact, fuzzy, ambiguous, unmatched
	Note       string
}

type parsedDraft struct {
	Picks    []pick
	Order    []*contestant // final ranking, 1 first; set only when Problems is empty
	Matched  int
	Problems []string
}

func (d parsedDraft) Ready() bool { return len(d.Problems) == 0 }

var (
	numberedLine = regexp.MustCompile(`^(?:#|no\.?\s*)?(\d{1,2})\s*(?:[.):\-–—]+|\s)\s*(.*)$`)
	discordToken = regexp.MustCompile(`<a?:\w+:\d+>|<[@#][!&]?\d+>|https?://\S+`)
	asides       = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	nickname     = regexp.MustCompile(`["“”]([^"“”]+)["“”]`)
	accents      = strings.NewReplacer("á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "é", "e", "è", "e", "ë", "e", "ê", "e", "í", "i", "ì", "i", "ï", "i", "î", "i", "ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "ú", "u", "ù", "u", "ü", "u", "û", "u", "ñ", "n", "ç", "c")
)

// normalize lowercases, folds common accents, and turns anything that is not a letter or digit into spaces.
func normalize(s string) string {
	s = accents.Replace(strings.ToLower(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		if r == '\'' || r == '’' {
			return -1
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// newRoster derives match aliases from stored names shaped like `First "Nick" Last`.
func newRoster(rows []contestant) []*contestant {
	roster := make([]*contestant, 0, len(rows))
	for i := range rows {
		c := rows[i]
		seen := map[string]bool{}
		addAlias := func(a string) {
			if a = normalize(a); a != "" && !seen[a] {
				seen[a] = true
				c.aliases = append(c.aliases, a)
			}
		}
		addAlias(c.Name)
		base := c.Name
		if m := nickname.FindStringSubmatch(c.Name); m != nil {
			addAlias(m[1])
			base = nickname.ReplaceAllString(c.Name, " ")
		}
		words := strings.Fields(base) // split before normalizing so "Jean-Charles" stays one surname
		addAlias(strings.Join(words, " "))
		if len(words) > 1 {
			addAlias(words[0])
			addAlias(words[len(words)-1])
			addAlias(strings.Join(words[:len(words)-1], " "))
		}
		roster = append(roster, &c)
	}
	return roster
}

func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 0
	}
	return 1 - float64(prev[len(rb)])/float64(longest)
}

const (
	fuzzyThreshold = 0.75
	fuzzyMargin    = 0.1
	fuzzyMinLength = 4
)

// matchName resolves free text to one contestant. It compares the whole cleaned text and its leading
// one to three words against every alias. Exact alias hits win; otherwise it accepts only a clear fuzzy
// winner and reports close calls as ambiguous instead of guessing.
func matchName(text string, roster []*contestant) (*contestant, string, string) {
	words := strings.Fields(text)
	windows := []string{text}
	for k := 1; k <= 3 && k < len(words); k++ {
		windows = append(windows, strings.Join(words[:k], " "))
	}
	type scored struct {
		c     *contestant
		score float64
	}
	var exact []*contestant
	var best []scored
	for _, c := range roster {
		top := 0.0
		for _, w := range windows {
			for _, a := range c.aliases {
				if w == a {
					top = 2
				} else if len([]rune(w)) >= fuzzyMinLength && len([]rune(a)) >= fuzzyMinLength {
					top = max(top, similarity(w, a))
				}
			}
		}
		if top == 2 {
			exact = append(exact, c)
		}
		best = append(best, scored{c, top})
	}
	switch {
	case len(exact) == 1:
		return exact[0], "exact", ""
	case len(exact) > 1:
		return nil, "ambiguous", "could be " + names(exact)
	}
	sort.SliceStable(best, func(i, j int) bool { return best[i].score > best[j].score })
	if len(best) == 0 || best[0].score < fuzzyThreshold {
		return nil, "unmatched", ""
	}
	if len(best) > 1 && best[0].score-best[1].score < fuzzyMargin {
		return nil, "ambiguous", "could be " + names([]*contestant{best[0].c, best[1].c})
	}
	return best[0].c, "fuzzy", fmt.Sprintf("%.2f", best[0].score)
}

func names(cs []*contestant) string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return strings.Join(out, " or ")
}

func cleanLine(s string) string {
	s = discordToken.ReplaceAllString(s, " ")
	s = asides.ReplaceAllString(s, " ")
	s = strings.TrimLeft(strings.TrimSpace(s), "-*•>|~_` \t")
	return strings.TrimSpace(s)
}

// candidateLines splits a message into lines, dropping code fences and splitting comma lists.
func candidateLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "```", ""))
		if line == "" {
			continue
		}
		if strings.Count(line, ",")+strings.Count(line, ";") >= 2 {
			for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ';' }) {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, part)
				}
			}
			continue
		}
		out = append(out, line)
	}
	return out
}

// parseDraft reads one free-text message. Numbered lines are ranked by their number; without numbers,
// matched lines are ranked top to bottom. Unnumbered lines that match nobody are treated as chatter.
func parseDraft(text string, roster []*contestant) parsedDraft {
	var d parsedDraft
	numbered := false
	for _, raw := range candidateLines(text) {
		line := cleanLine(raw)
		p := pick{Raw: raw}
		if m := numberedLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n >= 1 && n <= len(roster) {
				p.Number, line = n, m[2]
			}
		}
		p.Contestant, p.Method, p.Note = matchName(normalize(line), roster)
		if p.Contestant != nil {
			d.Matched++
		}
		if p.Number > 0 {
			numbered = true
		}
		if p.Contestant != nil || p.Number > 0 {
			d.Picks = append(d.Picks, p)
		}
	}
	if numbered {
		kept := d.Picks[:0]
		for _, p := range d.Picks {
			if p.Number > 0 {
				kept = append(kept, p)
			}
		}
		d.Picks = kept
		sort.SliceStable(d.Picks, func(i, j int) bool { return d.Picks[i].Number < d.Picks[j].Number })
	}
	d.Problems = validate(d.Picks, roster, numbered)
	if len(d.Problems) == 0 {
		for _, p := range d.Picks {
			d.Order = append(d.Order, p.Contestant)
		}
	}
	return d
}

func validate(picks []pick, roster []*contestant, numbered bool) []string {
	var problems []string
	seenNumber := map[int]bool{}
	seen := map[*contestant]int{}
	for _, p := range picks {
		if numbered {
			if seenNumber[p.Number] {
				problems = append(problems, fmt.Sprintf("number %d used twice", p.Number))
			}
			seenNumber[p.Number] = true
		}
		switch {
		case p.Contestant == nil && p.Method == "ambiguous":
			problems = append(problems, fmt.Sprintf("%q is ambiguous (%s)", strings.TrimSpace(p.Raw), p.Note))
		case p.Contestant == nil:
			problems = append(problems, fmt.Sprintf("%q matches no contestant", strings.TrimSpace(p.Raw)))
		default:
			seen[p.Contestant]++
		}
	}
	for _, c := range roster {
		if seen[c] > 1 {
			problems = append(problems, c.Name+" appears more than once")
		}
	}
	var missing []string
	for _, c := range roster {
		if seen[c] == 0 {
			missing = append(missing, c.Name)
		}
	}
	if len(missing) > 0 {
		problems = append(problems, "missing "+strings.Join(missing, ", "))
	}
	if numbered {
		for n := 1; n <= len(roster); n++ {
			if !seenNumber[n] {
				problems = append(problems, fmt.Sprintf("no pick numbered %d", n))
			}
		}
	}
	return problems
}

// isDraftCandidate separates drafts from chat: a draft names most of the cast.
func isDraftCandidate(d parsedDraft, roster []*contestant) bool {
	return d.Matched >= min(15, len(roster))
}
