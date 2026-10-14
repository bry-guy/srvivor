**Season {{.Season}}: {{.Name}} History**{{if .HasCurrent}} (currently {{.Current}}){{end}}{{if .Empty}}

No activity history found.{{end}}{{range .Groups}}

**{{.Label}}**{{if .Items}}
{{range $i, $item := .Items}}{{if $i}}
{{end}}{{$item.Activity}}
{{range $item.Details}}- {{.}}
{{end}}{{end}}{{else}}

n/a{{end}}{{end}}
