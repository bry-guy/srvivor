# Changelog

All notable changes to `castaway-discord-bot` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Restrict `/castaway` to `score [player]`, `scores`, and `draft [player]`, using API channel bindings, ephemeral public-only replies, and explicit BrainLand/Podracing guild registration.
- Restrict registration and dispatch to the configured multi-guild allowlist without clearing globals or falling back to global registration.
- Render `/castaway score` and `/castaway scores` with total points plus draft and visible bonus breakdowns from the leaderboard API.

## [0.1.0] - 2026-03-06

### Added
- Standalone Discord bot app with slash commands for scores, drafts, and instance context.
- Local stack integration through Docker Compose and root `mise run start` / `stop`.
- fnox + 1Password integration for Discord secrets.
