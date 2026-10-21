🎯 **Inside Track**: +1 per cup · +1 per ball left once all three are in
{{range .Players}}{{with .TribeEmoji}}{{.}} {{end}}{{.Name}} (+{{.Points}}) · {{.Score}}{{if .BallsLeft}} ({{.Cups}} cups + {{.BallsLeft}} balls left){{else}} ({{.Cups}} {{if eq .Cups 1}}cup{{else}}cups{{end}}){{end}}
{{end -}}
