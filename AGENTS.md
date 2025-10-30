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

## Testing Rules
Agents MUST adhere to the following when handling tests:
- **Regression Tests:** NEVER alter or remove existing regression tests without explicit user confirmation. This includes changes to test expectations, assertions, logic, or data in functions named with "Regression" (e.g., `TestSeason48Regression`). If a user explicitly creates a regression (e.g., "this is a regression test"), ask for confirmation before any modification. Agents may suggest changes but MUST NOT execute them without approval.
- **Unit Tests:** Can be modified as needed during refactoring, bug fixes, or improvements (e.g., `TestCalculateCurrentScore_Isolated`).
- **New Tests:** Can be added freely (e.g., new regression or unit tests).
- **Uncertainty Protocol:** If unsure if a test is regression or unit, classify conservatively as regression and ask for user permission before any change.
- **Logging:** Agents MUST log test actions, e.g., "Proposed change to regression test X: [details]. Awaiting user approval."
- **Examples:**
  - Allowed: Update `TestUnitFunction` assertions; add `TestNewRegression`.
  - Forbidden: Change `TestHistoricalRegression` without user OK.

Violations of these rules must be reported and reverted.

## Monorepo Tasks via Mise

The app uses mise for task management using Mise Monorepo tasks.

1. Run `mise tasks ls --all` in repo root to find available tasks.
2. Run tasks e.g.:
    a. `mise run //apps/...:*`: Run all tasks for all apps
    b. `mise run //apps/cli:*`: Run all tasks for apps/cli app
    c. `mise run //apps/cli:lint`: Run lint task for apps/cli app

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

## Pull Request Workflow

### Constraints
- ALL GitHub interactions MUST use the `@github` agent exclusively. Do NOT use external CLI tools like `gh`, `git`, or any bash commands for GitHub operations.
- Stick STRICTLY to the defined workflows. Do NOT perform actions outside of the listed steps (e.g., no merging, no creating or deleting branches, no direct pushes).
- In plan mode (read-only), ONLY observe, analyze, and plan. Do NOT execute changes.
- If an action is not explicitly defined in the workflow, do NOT perform it. Seek user confirmation for any deviations.

### Conflict Resolution
- **Autonomous Handling**: If conflicts can be resolved without altering main's existing behavior (e.g., adding new imports, non-logic changes), agents may proceed autonomously. Log the resolution for transparency.
- **User Involvement**: If resolution requires modifying main's behavior (e.g., logic changes, overwriting code), agents MUST NOT proceed. Instead, propose changes to the user as a series of accept/deny options, e.g.:
  - "Accept: Keep main's version of [file:line]?"
  - "Deny: Use incoming version?"
  - "Custom: Suggest merge with [specific edit]?"
  - Wait for user response before continuing.
- **Safety**: Abort operations if uncertain; never overwrite without approval. Conflicts reported by the hosting platform (e.g., GitHub) should trigger this protocol.

### Argument Validation

* PR identifier must be either:
  - A valid integer (e.g., "4")
  - A GitHub PR URL (e.g., "https://github.com/bry-guy/srvivor/pull/4")
* If $ARGUMENTS is empty or invalid
  - Search for existing pulls for the current feature branch via `@github`
    - If PR found, use that PR
    - If none found, assume you are creating a new PR at origin

Note: GitHub username is bry-guy.

### Workflow: New PR

0. **Name**: Based on the changes you've made, define a name (referred to as `$NAME`). If the branch is already named, defer to that.
1. **Setup**: If you are on main, make a feature branch named `bry-guy/$NAME`. If you are on a feature branch, stay on that branch.
2. **Commit:** Commit your changes using /commit.
3. **Push:** Use `@github` to push all commits to PR branch.

### Workflow: Existing PR

0. **PR Number Extraction:** If $ARGUMENTS is a URL, extract the PR number using regex pattern `/pull/(\d+)`
1. **Fetch:** Use `@github` agent to get PR branch name and review comments. 
2. **Setup**: Checkout and sync the PR branch, if needed. Check for and resolve git conflicts per Conflict Resolution guidelines. 

#### Code 

1. **Planning:** Create a todo list to address comments.
2. **Implementation:** Execute the todos.
3. **Self-Review:** Think deeply about your changes, and make improvements, if any.
4. **Commit:** Commit your changes using /commit.
5. **Push:** Use `@github` to push all commits to PR branch
6. **PR Update:** Use `@github` agent to mark resolved comments and update PR status

#### CI/CD

1. **Search:** Use `@github` to find CI issues.
2. **Planning:** Create a todo list to address the issues. If CI issues involve conflicts, follow Conflict Resolution protocols.
3. **Self-Review:** Think deeply about your changes, and make improvements, if any.
4. **Commit:** Commit your changes using /commit.
5. **Push:** Use `@github` to push all commits to PR branch

If no comments or issues, summarize status and await user instructions. Do NOT merge or close the PR.
