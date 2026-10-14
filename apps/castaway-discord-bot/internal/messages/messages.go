// Package messages renders user-facing Discord text from the templates in templates/.
//
// A file named like "approval-prompt.md" is one template; a .txt file holds several {{define}} blocks,
// each rendered by its block name. Names are unique across files.
package messages

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed templates
var files embed.FS

var set = parse()

func parse() *template.Template {
	root := template.New("").Option("missingkey=error")
	entries, err := fs.ReadDir(files, "templates")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		body, err := files.ReadFile("templates/" + e.Name())
		if err != nil {
			panic(err)
		}
		// Drop the file's final newline so Fragment output is exact.
		if _, err := root.New(e.Name()).Parse(strings.TrimSuffix(string(body), "\n")); err != nil {
			panic(fmt.Sprintf("parse template %s: %v", e.Name(), err))
		}
	}
	return root
}

// Render returns the named template's text with surrounding whitespace trimmed.
func Render(name string, data any) string {
	return strings.TrimSpace(Fragment(name, data))
}

// Fragment returns the named template's text exactly, for text appended to other text.
func Fragment(name string, data any) string {
	var b bytes.Buffer
	if err := execute(&b, name, data); err != nil {
		panic(fmt.Sprintf("message %s: %v", name, err)) // templates are embedded and checked by tests
	}
	return b.String()
}

// Err returns the rendered text as an error.
func Err(name string, data any) error {
	return errors.New(Fragment(name, data))
}

func execute(w *bytes.Buffer, name string, data any) error {
	return set.ExecuteTemplate(w, name, data)
}
