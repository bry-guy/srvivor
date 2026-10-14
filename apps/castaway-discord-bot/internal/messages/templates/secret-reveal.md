{{if and .ID .Name}}<@{{.ID}}> ({{.Name}}){{else if .ID}}<@{{.ID}}>{{else}}{{.Name}}{{end}} has revealed {{if eq .Count 1}}a secret bonus point{{else}}{{.Count}} secret bonus points{{end}}!
