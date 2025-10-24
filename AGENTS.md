# Agent Guidelines for srvivor

## Build/Lint/Test Commands
- **Build**: `make build` or `go build -o bin/srvivor .`
- **Test all**: `make test` (runs `SRVVR_LOG_LEVEL=DEBUG go test -v ./internal/*`)
- **Test single**: `go test -run TestName ./internal/scorer` (replace TestName with specific test)
- **Lint**: `golangci-lint run` (enabled: govet, errcheck, staticcheck, unused, gocritic, stylecheck, gosec, gofmt, goimports)

## Code Style Guidelines
- **Go version**: 1.24.0
- **Imports**: Group standard library, third-party, then local packages
- **Naming**: PascalCase for exported types/functions, camelCase for unexported
- **Error handling**: Return errors, avoid panics
- **Logging**: Use slog package
- **Testing**: Use testify/assert for assertions
- **Avoid**: Unnecessary destructuring, else statements, try/catch, any types, let statements
- **Variables**: Prefer single word names where possible
- **Comments**: Add for complex functions explaining purpose
- **Formatting**: Follow gofmt/goimports standards
- **Commits**: Use conventional commit format (e.g., feat:, fix:, docs:, etc.). **ALWAYS** ask for review and permission from the user before committing.
- **Patterns**: Refer to `wiki/code_patterns.md` for common patterns (e.g., interface-based dependency injection). Agents should consult this document when making any code changes to ensure consistency.

## Build/Lint/Test Commands

The app uses mise for task management. Available tasks:

- **Root level** (`mise tasks ls` in repo root):
  - `ci`: Run all monorepo CI tasks (runs `mise run //apps/...:ci`)

- **Apps level** (`mise tasks ls` in `./apps`):
  - `ci`: Run all monorepo CI tasks

- **CLI app level** (`mise tasks ls` in `./apps/cli`):
  - `lint`: Run golangci-lint
  - `test`: Run tests (depends on lint)
  - `run`: Run the app (depends on lint)
  - `clean`: Remove bin directory
  - `build`: Build the app (depends on clean)
  - `ci`: Run lint, test, build for CI

## Agent Memory and Wiki Usage

Agents should store their memory and session notes in the `wiki/` Obsidian vault for historical context and knowledge sharing.

### Note Naming Convention
- Use semantically meaningful names with datetime leaders in ISO 8601 format: `YYYYMMDDTHHMMSS_topic_description`
- Example: `20250501T190000_create_wiki`
- Datetime should reflect when the note was created or the action occurred

### When to Save Notes
- **ALWAYS save notes before committing changes**: Compact the session conversation into a note with examples, references, and key decisions, and include the note in the same commit as the code changes to avoid separate commits
- **Optionally save notes anytime**: For significant actions, discoveries, or context that might be useful later
- **Update existing notes**: If working on the same topic (even across commits), update the existing note rather than creating new ones
- **Create new notes**: For semantically different actions or topics

### Linking Notes
- After writing a note, grep the wiki for similar notes and add backlinks using `[[Note Name]]` syntax
- Focus on relevant connections, not exhaustive linking

### Reading Notes
- Agents can read notes anytime for historical or non-code context on the project
- When reading a note, follow useful links from it, but never traverse the entire wiki at once
- Use grep to find specific notes by topic or keyword
