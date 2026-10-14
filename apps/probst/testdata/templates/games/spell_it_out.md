🔠 **Island Scramble**
{{range .Players}}{{if .Place}}{{if eq .Place 1}}🥇{{else if eq .Place 2}}🥈{{else if eq .Place 3}}🥉{{else}}{{.Place}}.{{end}} {{with .TribeEmoji}}{{.}} {{end}}{{.Name}}{{if .Points}} (+{{.Points}}){{end}}
{{end}}{{end -}}
