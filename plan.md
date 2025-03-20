# Plan

Date: 2025-03-19

I have a few goals for this application:

- Code: Organize to be more modular, reusable, and testable
- Code: Simplify logger
- Code: Reorganize "finals" into "drafts" as a "final draft"
- Feature: Validate draft file meets format
- Feature: Validate draft meets roster
- Feature: Automatically clean draft input to meet roster
- Feature: Points Avaiable is correct
- Rename: `srvivor-cli`

Some of the above may move to the following, more major goal - to build an web app. 

`srvivor`, the web server for persistent hosting and tracking of survivor drafts:
- Feature: Discord integration
- Feature: Live drafting
- Feature: Score draft
- Feature: Draft/season history

## Current Plan

### Organize Code

I see a few primary functionalities:
- draft i/o and processing
- scoring
- cobra cmds

I want cobra cmds to follow conventions, which are typically like:
```
app/
├── cmd/
│   ├── root.go
│   ├── command1.go
│   ├── command2.go
│   └── subcommands/
│       ├── subcommand1.go
│       └── subcommand2.go
├── internal/
│   └── pkg/
│       ├── service1/
│       └── service2/
└── main.go
```

Notably, commands should be `$ appname verb noun --adjective` or `$ appname command arg --flag`.

In my case, this would look like:
```
srvivor-cli/
├── cmd/
│   ├── root.go
│   ├── score.go
│   ├── validate.go
│   └── subcommands/
│       └── parse.go
├── internal/
│   └── pkg/
│       └── draft/
└── main.go
```
So, let's go about it.

