**Season {{.Season}}: {{.Name}}**
- Type: {{.Type}}
- Status: {{.Status}}{{with .Starts}}
- Starts: {{.}}{{end}}{{with .Ends}}
- Ends: {{.}}{{end}}{{with .Assignments}}

**Assignments**{{range .}}
- {{.}}{{end}}{{end}}{{with .Occurrences}}

**Occurrences**{{range .}}
{{.Name}}
- Type: {{.Type}}
- Status: {{.Status}}{{with .Date}}
- Date: {{.}}{{end}}{{end}}{{end}}
