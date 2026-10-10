package httpapi

import (
	"bytes"
	"strings"
	"testing"
)

func TestHomeScoresTribeDots(t *testing.T) {
	for _, tc := range []struct {
		name, tribe, dot string
	}{
		{"Savu", "Savu", `<span class="tribe-dot tribe-dot-purple" aria-hidden="true"></span> `},
		{"Toka", "Toka", `<span class="tribe-dot tribe-dot-yellow" aria-hidden="true"></span> `},
		{"unknown", "Other", ""},
		{"empty", "", ""},
		{"no stored color", "Uncolored", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rendered bytes.Buffer
			data := sitePageData{TribeColors: map[string]string{"Savu": "purple", "Toka": "yellow"}, Allowed: true, User: &webSession{Username: "Player"}, Rows: []homeRow{{ID: "p1", Name: "Player", Tribe: tc.tribe}}}
			if err := siteTemplates.ExecuteTemplate(&rendered, "home.html", data); err != nil {
				t.Fatal(err)
			}
			html := rendered.String()
			if want := `<th scope="row">` + tc.dot + `<a href="/players/p1">Player</a></th>`; !strings.Contains(html, want) {
				t.Errorf("player cell = %q, want it to contain %q", html, want)
			}
			if want := `<td>` + tc.tribe + `</td>`; !strings.Contains(html, want) {
				t.Errorf("Scores is missing the visible tribe cell %q", want)
			}
		})
	}
}
