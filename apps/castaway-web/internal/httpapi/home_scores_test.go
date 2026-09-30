package httpapi

import (
	"bytes"
	"strings"
	"testing"
)

func TestHomeScoresTemplate(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []homeRow
		want string
	}{
		{
			name: "populated",
			rows: []homeRow{{Rank: 1, Name: "Player", Tribe: "Red", Total: 47, Draft: 99, Bonus: 13}},
			want: `<td class="total">47</td><td>13</td>`,
		},
		{name: "empty", want: `<td colspan="5"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rendered bytes.Buffer
			data := sitePageData{Allowed: true, User: &webSession{Username: "Player"}, Rows: tc.rows}
			if err := siteTemplates.ExecuteTemplate(&rendered, "home.html", data); err != nil {
				t.Fatal(err)
			}
			html := rendered.String()
			start := strings.Index(html, `<div class="table-wrap"`)
			if start < 0 {
				t.Fatal("Scores table not found")
			}
			relativeEnd := strings.Index(html[start:], `</table>`)
			if relativeEnd < 0 {
				t.Fatal("Scores table was not closed")
			}
			table := html[start : start+relativeEnd+len(`</table>`)]
			if count := strings.Count(table, `<th scope="col">`); count != 5 {
				t.Errorf("found %d Scores column headers, want 5", count)
			}
			for _, header := range []string{"#", "Player", "Tribe", "Total", "Bonus"} {
				if !strings.Contains(table, `<th scope="col">`+header+`</th>`) {
					t.Errorf("Scores is missing the %q column", header)
				}
			}
			if strings.Contains(table, `<th scope="col">Draft</th>`) {
				t.Error("Scores includes a Draft column")
			}
			if !strings.Contains(table, tc.want) {
				t.Errorf("Scores does not contain %q", tc.want)
			}
		})
	}
}
