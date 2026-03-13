# Castaway Functional Requirements

Castaway is the monorepo for Survivor fantasy draft tooling.

## Scope

The repository must support the following product surfaces:

- `apps/cli`: legacy local CLI workflows for scoring and draft normalization
- `apps/castaway-web`: persistent HTTP API and historical seed workflows
- `apps/castaway-discord-bot`: Discord-native workflows backed by `castaway-web`

## Repository-level requirements

- provide a reproducible developer workflow through shared tooling
- support local startup, seeding, validation, and CI from the monorepo root
- preserve compatibility between the web API and the Discord bot integration
- retain historical draft/roster data needed for seeds and legacy CLI workflows
- organize new work as app-specific or shared documentation with clear ownership

## Inputs and outputs

### Inputs
- source code and configuration in the monorepo
- local secrets injected through configured tooling
- historical draft and roster data
- HTTP and Discord interactions handled by the app surfaces

### Outputs
- runnable app artifacts
- reproducible local development workflows
- documented API, bot, and CLI behavior
- maintainable historical data and plans
