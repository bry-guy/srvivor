🔴 **Press the Button**: {{range $i, $t := .Tribes}}{{if $i}} · {{end}}{{$t.Emoji}} {{$t.Name}} {{printf "%.0f" $t.Value}} presses{{end}}
{{range $i, $p := .Players}}{{if lt $i 3}}{{with $p.TribeEmoji}}{{.}} {{end}}{{$p.Name}}: {{$p.Presses}} presses ({{printf "%+d" $p.Points}})
{{end}}{{end -}}
{{range .Players}}{{if .First}}First to press: {{.Name}}. {{end}}{{end}}{{range .Players}}{{if .Last}}Last to press: {{.Name}}.{{end}}{{end}}
