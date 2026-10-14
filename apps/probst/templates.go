package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// Every player-facing word lives in a season's template directory (seasons/51-templates by default, or the
// file's `templates:`), never in code:
//
//	scores.md                 the weekly scores post
//	reminder.md               the game-day reminder
//	next-game.md              the "this week's game" line at the end of a scores post
//	games/<type>.md           a game's results section in the scores post ({{.GameScore}})
//	lines/<name>.txt          one-line variants, one per line, picked by week so none repeats in a season
//
// All are Go text/templates. `probst season sync` stores the set as one bundle (see bundle).
type templateSet map[string]string

const bundleMarker = "=== template: "

// templateDir is the season file's template directory.
func templateDir(file string, f seasonFile) string {
	if f.Templates != "" {
		return filepath.Join(filepath.Dir(file), f.Templates)
	}
	return strings.TrimSuffix(file, filepath.Ext(file)) + "-templates"
}

func loadTemplates(dir string) (templateSet, error) {
	set := templateSet{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path) // #nosec G304 -- the season's own template directory
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		set[filepath.ToSlash(rel)] = string(raw)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("templates %s: %w", dir, err)
	}
	if _, ok := set["scores.md"]; !ok {
		return nil, fmt.Errorf("templates %s: no scores.md", dir)
	}
	return set, nil
}

// bundle is the set as one text, for the synced season config.
func (t templateSet) bundle() string {
	names := make([]string, 0, len(t))
	for n := range t {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s%s ===\n%s", bundleMarker, n, t[n])
		if !strings.HasSuffix(t[n], "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func parseBundle(text string) (templateSet, error) {
	if !strings.HasPrefix(text, bundleMarker) {
		return nil, fmt.Errorf("the synced templates are from an older probst; re-run `probst season sync FILE --yes`")
	}
	set := templateSet{}
	for _, part := range strings.Split(text, "\n"+bundleMarker)[0:] {
		part = strings.TrimPrefix(part, bundleMarker)
		name, body, _ := strings.Cut(part, " ===\n")
		set[name] = body
	}
	return set, nil
}

// render fills template name with data. A missing template is an error: say what to add.
func (t templateSet) render(name string, data any) (string, error) {
	text, ok := t[name]
	if !ok {
		return "", fmt.Errorf("no %s template", name)
	}
	return renderText(name, text, data)
}

func renderText(name, text string, data any) (string, error) {
	tm, err := template.New(name).Option("missingkey=error").Funcs(template.FuncMap{"inc": func(i int) int { return i + 1 }}).Parse(text)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tm.Execute(&b, data); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(b.String()), nil
}

// lineData fills a one-line variant: Names is the player(s), N the points.
type lineData struct {
	Names string
	N     int
}

// line renders week's variant from lines/<name>.txt.
func (t templateSet) line(name string, week int, data lineData) (string, error) {
	var lines []string
	for _, l := range strings.Split(t["lines/"+name+".txt"], "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no lines/%s.txt template", name)
	}
	return renderText("lines/"+name+".txt", lines[(max(week, 1)-1)%len(lines)], data)
}

// shortTime is "8pm" or "7:59pm".
func shortTime(t time.Time) string { return strings.Replace(t.Format("3:04pm"), ":00", "", 1) }
