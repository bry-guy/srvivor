**{{.Activity}}: Occurrences**
{{range $i, $o := .Occurrences}}{{if $i}}
{{end}}{{$o.Name}}
- Status: {{$o.Status}}{{with $o.Date}}
- Date: {{.}}{{end}}{{range $o.Impacts}}
- Impact: {{.}}{{end}}
{{end}}
