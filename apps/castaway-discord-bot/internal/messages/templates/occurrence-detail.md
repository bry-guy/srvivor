**{{.Name}}**
- Activity: {{.Activity}}
- Type: {{.Type}}
- Status: {{.Status}}
- Date: {{.Date}}{{with .Recorded}}

**Recorded**{{range .}}
- {{.}}{{end}}{{end}}{{with .Impact}}

**Impact**{{range .}}
- {{.}}{{end}}{{end}}
