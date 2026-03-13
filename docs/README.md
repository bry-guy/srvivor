# Castaway Docs

This index maps the shared documentation that lives under `docs/`.
Documentation requirements and placement rules live in `documentation-standards.md`.

## Structure

- `archive-wiki/`: historical project notes and legacy wiki content from the `srvivor` phase.
- `castaway-discord-bot-plan.md`: architecture and implementation plan for the Discord bot app.
- `castaway-manual-gameplay-logs.md`: logs from manually-run gameplay, useful for backfilling requirements and game mechanics.
- `castaway-web-future-work.md`: deferred and future `castaway-web` ideas that are intentionally out of scope today.
- `documentation-standards.md`: minimum required documentation and placement rules for the repo and each app.
- `gameplay/`: gameplay mechanic documentation and player-facing prompts for journeys, twists, and bonus point systems.
- `non-functional-requirements.md`: cross-cutting security, reliability, and operational requirements.
- `production-readiness-checklist.md`: explicit pre-production checklist across apps.
- `secrets-and-config.md`: shared 1Password/fnox/mise secret and config conventions.
- `versioning-and-releases.md`: semver rules, release heuristics, and GitHub release flow.

Repository-level plans now live under `/plans`, not under `docs/`.

As active shared docs are added, place them at the top level under `docs/` unless they belong in a more specific subdirectory such as `docs/gameplay/`.
