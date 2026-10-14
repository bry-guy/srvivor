**Season {{.Season}}: Individual Pony Auction**
- Bonus points available: {{.Bonus}}
- Loan due: {{.LoanDue}}
{{if .NoLots}}
No open auction lots.{{else}}
**Open lots**
{{range .Lots}}- {{.ContestantName}}{{if gt .MyBidPoints 0}} — your bid: {{.MyBidPoints}}{{end}}
{{end}}{{end}}{{with .Ponies}}
**Your ponies**
{{range .}}- {{.ContestantName}}
{{end}}{{end}}
