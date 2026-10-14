⚠️ **Episode {{.Episode}} needs a decision before it can be scored:**
{{range .Holds}}• {{.}}
{{end}}
Record those by hand (`probst boot` / `probst challenge`), then run `probst episode import {{.Episode}} --file seasons/{{.Season}}.yaml --resolve-holds --yes` to record the rest. The Week {{.Week}} scores post waits until then.
