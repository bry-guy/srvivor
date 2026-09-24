package main

import (
	"fmt"
	"strings"
	"testing"
)

var season51Names = []string{
	"Aaliyah Puglia", "Alexis Levine", "Ana Sani", `Angelica "Jelly" Loblack`, "Brady Booker",
	"Carter Krull", "Cristian Chavez", `Danny "Kilby" Kilby`, "Devin Way", "Eric Macksoud",
	"Jenna Doore", "Kristin Flickinger", "Lewis Kelly", "Linnea Capobianco", "Maggie Nestor",
	"Mike Pinsky", "Ori Jean-Charles", "Patt Cannaday", "Rob Antonson", "Sharonda Cox", "Thien An Nguyen",
}

func season51Roster() []*contestant {
	rows := make([]contestant, len(season51Names))
	for i, n := range season51Names {
		rows[i] = contestant{ID: fmt.Sprintf("c%02d", i), Name: n}
	}
	return newRoster(rows)
}

func orderNames(d parsedDraft) []string {
	out := make([]string, len(d.Order))
	for i, c := range d.Order {
		out[i] = c.Name
	}
	return out
}

func TestParseDraftMessyNumberedMessage(t *testing.T) {
	roster := season51Roster()
	msg := "ok here's mine, don't judge 😅\n```\n" +
		"1) Kilby 🔥 (my guy)\n" +
		"2. Jelly\n" +
		"3 - Thein An\n" +
		"4. **Cristian Chávez**\n" +
		"5: Macksod\n" +
		"6. Ori Jean-Charles\n" +
		"7. Devin <@123> called it\n" +
		"8. Sharonda\n" +
		"9. Rob\n" +
		"10. Aaliyah\n" +
		"11. Alexis\n" +
		"12. Ana Sani\n" +
		"13. brady booker\n" +
		"14. Carter\n" +
		"15. Jenna\n" +
		"16. Kristin F\n" +
		"17. Lewis\n" +
		"18. Linnea\n" +
		"19. Maggie\n" +
		"20. Mike\n" +
		"21. Patt\n```\ngood luck everyone!!"
	d := parseDraft(msg, roster)
	if !d.Ready() {
		t.Fatalf("expected ready, got %v", d.Problems)
	}
	got := orderNames(d)
	want := []string{`Danny "Kilby" Kilby`, `Angelica "Jelly" Loblack`, "Thien An Nguyen", "Cristian Chavez", "Eric Macksoud", "Ori Jean-Charles", "Devin Way"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("position %d: got %q want %q (all: %v)", i+1, got[i], w, got)
		}
	}
	if !isDraftCandidate(d, roster) {
		t.Fatal("draft not recognised")
	}
}

func TestParseDraftOrderingForms(t *testing.T) {
	roster := season51Roster()
	var unnumbered, descending, commas []string
	for i := range season51Names {
		name := strings.Fields(strings.ReplaceAll(season51Names[i], `"`, ""))[0]
		unnumbered = append(unnumbered, name)
		descending = append(descending, fmt.Sprintf("%d. %s", len(season51Names)-i, name))
		commas = append(commas, name)
	}
	for label, msg := range map[string]string{
		"unnumbered": strings.Join(unnumbered, "\n"),
		"descending": strings.Join(descending, "\n"),
		"commas":     strings.Join(commas, ", "),
	} {
		d := parseDraft(msg, roster)
		if !d.Ready() {
			t.Fatalf("%s: %v", label, d.Problems)
		}
		first, last := d.Order[0].Name, d.Order[len(d.Order)-1].Name
		if label == "descending" {
			first, last = last, first
		}
		if first != "Aaliyah Puglia" || last != "Thien An Nguyen" {
			t.Fatalf("%s: order %v", label, orderNames(d))
		}
	}
}

func TestParseDraftReportsProblemsInsteadOfGuessing(t *testing.T) {
	roster := season51Roster()
	lines := make([]string, 0, len(season51Names))
	for i, n := range season51Names {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, n))
	}
	lines[20] = "21. An"       // Ana or Thien An: must not guess
	lines[1] = "2. Aaliyah"    // duplicate, Alexis missing
	lines[5] = "7. Carter"     // number 6 missing, 7 used twice
	lines[9] = "10. Mystery X" // no such contestant
	d := parseDraft(strings.Join(lines, "\n"), roster)
	if d.Ready() {
		t.Fatal("broken draft accepted")
	}
	all := strings.Join(d.Problems, "\n")
	for _, want := range []string{`"21. An" matches no contestant`, "Aaliyah Puglia appears more than once", "missing Alexis Levine", "number 7 used twice", "no pick numbered 6", `"10. Mystery X" matches no contestant`} {
		if !strings.Contains(all, want) {
			t.Errorf("missing problem %q in:\n%s", want, all)
		}
	}
}

func TestMatchNameAmbiguityAndChat(t *testing.T) {
	roster := season51Roster()
	if c, method, _ := matchName(normalize("Kristen Flickenger"), roster); c == nil || c.Name != "Kristin Flickinger" || method != "fuzzy" {
		t.Fatalf("fuzzy surname: %v %s", c, method)
	}
	if c, method, _ := matchName(normalize("An"), roster); c != nil {
		t.Fatalf("guessed %q via %s", c.Name, method)
	}
	chat := "lol Kilby is going home first\nJelly too tbh\nwhen are drafts due?"
	if isDraftCandidate(parseDraft(chat, roster), roster) {
		t.Fatal("chat treated as a draft")
	}
}
