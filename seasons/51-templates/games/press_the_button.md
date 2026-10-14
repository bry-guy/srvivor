🔴 **Press the Button**: {{range $i, $t := .Tribes}}{{if $i}} · {{end}}{{$t.Emoji}} {{$t.Name}} {{printf "%.0f" $t.Value}} presses{{end}}
{{range .Rules.most}}👑 Most presses: {{.Names}}, {{.Presses}} ({{printf "%+d" .Points}})
{{end}}{{range .Rules.second}}🥈 Second most: {{.Names}}, {{.Presses}} ({{printf "%+d" .Points}}). So close, so costly.
{{end}}{{range .Rules.tie}}🤝 Tied at {{.Presses}}: {{.Names}} ({{printf "%+d" .Points}} each)
{{end}}{{range .Rules.least}}🐢 Fewest presses: {{.Names}}, {{.Presses}} ({{printf "%+d" .Points}})
{{end}}{{range .Rules.volume}}💪 {{.Presses}} presses: {{.Names}} ({{printf "%+d" .Points}} volume bonus)
{{end}}{{range .Rules.streak}}🔥 {{.Days}}-day streak: {{.Names}} ({{printf "%+d" .Points}}{{if gt .Count 1}} each{{end}})
{{end}}{{range .Rules.first}}⚡ First to press: {{.Names}} ({{printf "%+d" .Points}})
{{end}}{{range .Rules.last}}⏰ Last to press: {{.Names}} ({{printf "%+d" .Points}})
{{end}}
