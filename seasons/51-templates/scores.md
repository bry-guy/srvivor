## Castaway Season {{.Season}}: Week {{.Week}} Scores

{{.Intro}}
{{if .Boots}}
{{.Boots}}
{{end}}
👑 {{.Leader}}
{{- if .Gainer}}
📈 {{.Gainer}}{{end}}
{{- if .Slider}}
📉 {{.Slider}}{{end}}

{{range $i, $r := .Top}}{{if eq $i 0}}🥇{{else if eq $i 1}}🥈{{else}}🥉{{end}} {{$r.Mention}}: {{$r.Total}}
{{end -}}
🔦 {{.Last.Mention}}: {{.Last.Total}}
{{- if .GameScore}}

{{.GameScore}}{{end}}

Full scores: <{{.Site}}>
{{- if .NextGame}}

{{.NextGame}}{{end}}
