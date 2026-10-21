🎯 **Inside Track**: best score +3 · second +2 · everyone else who played +1 (score = cups, plus balls left once all three are in)
{{range .Players}}{{with .TribeEmoji}}{{.}} {{end}}{{.Name}} (+{{.Points}}) · {{.Score}}{{if .BallsLeft}} ({{.Cups}} cups + {{.BallsLeft}} balls left){{else}} ({{.Cups}} {{if eq .Cups 1}}cup{{else}}cups{{end}}){{end}}
{{end -}}
