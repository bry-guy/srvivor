# Monorepo Restructuring

## Overview
Restructured the srvivor repository from a single CLI application to a monorepo foundation supporting multiple srvivor tools (CLI, web, bot, MCP server, etc.). This establishes the architectural skeleton for scalable development while maintaining current functionality.

## Changes Made

### Directory Structure
- **cli/**: CLI application with app-specific data
  - Contains `cmd/`, `main.go`, `go.mod`, `go.sum`
  - Data directories: `drafts/`, `finals/`, `rosters/`, `test_fixtures/`
- **shared/**: Extracted shared business logic packages
  - `config/`, `log/`, `matcher/`, `roster/`, `scorer/`
- **tools/ci/**: Prepared for CI tooling
- **wiki/**: Retained as documentation directory

### Tooling Updates
- **Mise**: Configured as primary task runner replacing Make
  - Root `mise.toml` with build/test/lint/run tasks
  - Tasks assume execution from project root
- **Go Module**: Single module at root covering all packages
- **CI**: Added basic GitHub Actions workflow for testing

### Code Refactoring
- **Import Updates**: Updated all CLI imports to reference `shared/` packages
- **Path Adjustments**: Fixed data loading paths in roster loader and tests
- **Test Fixes**: Updated test fixture paths to work with new structure

### Documentation
- **README.md**: Updated for monorepo context and Mise usage
- **AGENTS.md**: Already updated for conventional commits
- **Wiki Notes**: Preserved historical context in wiki/

## Migration Challenges Resolved
- **Import Paths**: Resolved circular dependencies and module boundaries
- **Data Paths**: Moved app-specific data to cli/ for self-containment
- **Test Paths**: Updated relative paths in test files
- **Build System**: Replaced Make with Mise for cross-platform task management

## App Self-Containment
Each application (cli/, future web/, bot/, etc.) is now self-contained with its own data and dependencies, while benefiting from shared tooling and packages. This allows individual apps to be developed, tested, and deployed independently.

## Future Extensions
This structure enables easy addition of:
- **web/**: Website application (Next.js, etc.)
- **bot/**: Discord bot (Python, Go)
- **mcp/**: MCP server implementation
- Additional shared packages as needed

## Development Workflow
- Install Mise: `mise install`
- Build: `mise run build`
- Test: `mise run test`
- Lint: `mise run lint`
- Run: `mise run run`

## Compatibility
- All existing CLI functionality preserved
- Tests updated and should pass
- Data formats unchanged
- API contracts maintained

## Related Notes
- [Current App State](20251016T150000_current_app_state.md)
- [Archived Docs](archived_docs.md)
- [Archived Thoughts](archived_thoughts.md)