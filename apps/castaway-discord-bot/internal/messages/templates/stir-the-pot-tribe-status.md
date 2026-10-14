**Season {{.Season}}: Stir the Pot**
- Tribe: {{.Tribe}}
{{if .Open}}- Round: {{.Round}}
- Current contribution: {{.Contribution}}
- Bonus if resolved now: +{{.BonusIfNow}}{{if .HasNext}}
- Next tier: {{.NextLabel}} ({{.NextNeeded}} more){{end}}{{with .Tiers}}
- Reward tiers: {{.}}{{end}}{{else}}Stir the Pot is not currently open.{{end}}
