🔤 **Castawordle**, best three average: {{range $i, $t := .Tribes}}{{if $i}} · {{end}}{{$t.Emoji}} {{$t.Name}} {{printf "%.1f" $t.Value}}{{end}}
{{range $i, $p := .Players}}{{if $p.Solved}}{{if $i}} · {{end}}{{with $p.TribeEmoji}}{{.}} {{end}}{{$p.Name}} {{$p.Guesses}}{{end}}{{end}}
