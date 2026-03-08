# Versioning and releases

Castaway versions deployable apps independently.

## Apps

Current semver-managed apps:

- `castaway-web`
- `castaway-discord-bot`

Each app starts at `0.1.0` and is released independently with its own changelog and release tag.

## Why semver still makes sense for a web app

Yes, even for a web service, semver is useful.

Use the version to describe the deployable artifact and its operational/API contract, not just a user-visible UI.

For `castaway-web`, the contract includes:
- HTTP endpoints
- request/response shapes
- authentication requirements
- required runtime configuration

For `castaway-discord-bot`, the contract includes:
- slash command names and options
- bot runtime configuration
- bot behavior that users or operators depend on

## Bump rules

### Patch
Use a patch release for backward-compatible fixes.

Examples:
- bug fix with no contract break
- formatting/logging improvements
- non-breaking Discord UX polish
- internal performance improvements
- documentation-only release notes or operational fixes

### Minor
Use a minor release for backward-compatible features.

Examples:
- add a new endpoint
- add an optional query parameter
- add a new slash command or subcommand
- add optional config with safe defaults
- additive response fields that existing clients can ignore

### Major
Use a major release for breaking changes.

Examples:
- rename or remove an endpoint
- change response shape incompatibly
- make auth required where it was previously open
- rename slash commands or required options incompatibly
- rename or remove required config/env vars
- change behavior in a way existing consumers/operators must adapt to

## During the 0.x phase

While the apps are pre-1.0:
- still follow semver semantics
- but expect faster iteration
- treat breaking changes seriously even if the version is still `0.x`

In practice:
- `0.x.patch` = safe fixes
- `0.x.minor` = features or breaking changes during early development

If you want stricter signaling for pre-1.0, call out breaking changes clearly in release notes.

## GitHub release flow

This repo uses:
- squash merges
- semantic PR titles
- release-please

Release mapping:
- `fix:` => patch
- `feat:` => minor
- `feat!:` or `BREAKING CHANGE:` => major

Release-please is configured for the two app paths and will manage:
- release PRs
- version bumps
- changelog updates
- GitHub releases

## Important GitHub settings

Repository files prepare the release flow, but they do **not** automatically change GitHub repository settings.

Recommended GitHub settings:
- allow squash merge
- prefer disabling merge commits and rebase merges if you want one semantic commit on `main`
- protect `main` with required CI and semantic PR checks

## Build metadata

Both apps expose build metadata through `--version` and startup logs.

Fields:
- version
- commit
- build date

Release builds can inject those values through Go ldflags.
