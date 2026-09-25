# Changelog

All notable changes to `castaway-web` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Season 51 episode schedule (CBS Wednesdays 8pm ET), admin tribe assignment over time (`/instances/{id}/tribes`), tribe immunity/reward awards (`/tribe-challenges`, +2/+1), and opt-in Wordle scoring: +2 best individual, +1 to the tribe with the best submitter average.
- API-owned Discord channel bindings, restricted first-admin bootstrap, and transactional player-link administration for Probst.
- Repeatable Season 43 Hurl rehearsal using pinned survivoR data, fake drafts and public Wordle awards, with explicit opt-in accelerated BrainLand delivery.
- Admin-operated Wordle rounds for legacy instances, with explicit submission windows, public bonus awards, atomic resolution, and retry-safe creation/closure/resolution.

## [0.1.0] - 2026-03-06

### Added
- Persistent Castaway web API with instances, participants, drafts, outcomes, and leaderboard endpoints.
- Bot-friendly instance, participant, and leaderboard filters.
- OpenAPI generation and route parity checks to prevent API drift.
