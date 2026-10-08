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

🥇 {{index .Top 0}}
{{- if gt (len .Top) 1}}
🥈 {{index .Top 1}}{{end}}
{{- if gt (len .Top) 2}}
🥉 {{index .Top 2}}{{end}}
🔦 {{.Last}}

Full scores: <{{.Site}}>
{{- if .NextGame}}

{{.NextGame}}{{end}}
