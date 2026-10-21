Alright players, you know in the Open Era anything can happen. {{if eq .Kind "merge"}}It's time for the merge!{{else if eq .Kind "assignment"}}It's time for new tribes!{{else}}It's time for a tribe swap!{{end}} Everyone gets a new buff. Check out the new tribes:

{{range .Tribes}}{{.Emoji}} **{{.Name}}**: {{range $i, $m := .Members}}{{if $i}} {{end}}{{$m}}{{end}}
{{end}}{{if .Awards}}
This time, we're giving points to the new tribes retroactively:

{{range .Awards}}{{.Emoji}} **{{.Name}}** won {{if .Immunity}}immunity (+2){{if .Reward}} and reward (+1){{end}}{{else}}reward (+1){{end}}: +{{.Points}} each!
{{end}}{{end}}
