# Agent Guidelines for Castaway

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

## Change Rules

- ALWAYS ensure the app lints, tests, builds, and runs before committing or PRing
- Commit complete thoughts frequently; this repo squash merges PRs, so prefer smaller committed checkpoints over large uncommitted changesets
- NEVER remove or update a regression test without asking permission

## Documentation Rules

- Keep repository and app docs aligned with `docs/documentation-standards.md`
- Each app should keep `README.md`, `CHANGELOG.md`, `functional-requirements.md`, `non-functional-requirements.md`, `production-readiness-checklist.md`, and `plans/` present and in sync
- Plan documents should be well-named and carry a status of `planning`, `in-progress`, or `done`
