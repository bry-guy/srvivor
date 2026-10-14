package httpapi

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// Every Discord message the server writes (draft buffs, admin alerts) is a template in messages/.
//
//go:embed messages
var messageFS embed.FS

var messages = template.Must(template.New("").Option("missingkey=error").ParseFS(messageFS, "messages/*"))

// message renders messages/<name> with data.
func message(name string, data any) string {
	var b bytes.Buffer
	if err := messages.ExecuteTemplate(&b, name, data); err != nil {
		panic(fmt.Sprintf("message %s: %v", name, err)) // templates are embedded and checked by tests
	}
	return strings.TrimSpace(b.String())
}

// messageLines is messages/<name> (one variant per line), each filled with data.
func messageLines(name string, data any) []string {
	var out []string
	for _, l := range strings.Split(message(name, data), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func easternTZ() *time.Location {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		return loc
	}
	return time.UTC
}
