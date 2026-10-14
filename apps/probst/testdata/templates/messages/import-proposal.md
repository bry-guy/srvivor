🗳️ **Episode {{.Episode}} results need your OK** (survivoR {{.Revision}}):
{{range .Review}}• {{.}}
{{end}}
This will record:
{{range .Boots}}• {{.Name}}: place {{.Position}} ({{.Result}})
{{end}}{{range .Challenges}}• {{.Tribes}} won {{.Kind}}: {{.Points}} each
{{end}}{{range .Champions}}• {{.Winners}} won {{.Kind}}: their Champion pickers score
{{end}}
The Week {{.Week}} scores post waits until then.
