# Castaway Docs

This index maps the shared documentation that lives under `docs/`.
Documentation requirements and placement rules live in `documentation-standards.md`.

## Structure

- `archive-wiki/`: historical project notes and legacy wiki content from the `srvivor` phase.
- `castaway-bonus-points-blueprint.md`: cross-app bonus-points design and data-model blueprint.
- `castaway-discord-bot-blueprint.md`: architecture and structure blueprint for the Discord bot app.
- `castaway-manual-gameplay-logs.md`: manual gameplay logs and operational notes used to backfill bonus-point requirements and mechanics.
- `castaway-web-future-work.md`: deferred and future `castaway-web` ideas that are intentionally out of scope today.
- `documentation-standards.md`: minimum required documentation and placement rules for the repo and each app.
- `gameplay/`: gameplay mechanic documentation and player-facing prompts for journeys, twists, and bonus point systems.
- `guides/`: shared human-oriented guides for operating, deploying, maintaining, or otherwise accomplishing concrete tasks with Castaway.
- `non-functional-requirements.md`: cross-cutting security, reliability, and operational requirements.
- `production-readiness-checklist.md`: explicit pre-production checklist across apps.
- `secrets-and-config.md`: shared 1Password/fnox/mise secret and config conventions.
- `selfhost-home-k3s-deployment-readiness-checklist.md`: deployment readiness checklist for the self-hosted `home-k3s` target.
- `selfhost-k3s-deployment-blueprint.md`: structural design blueprint for the first self-hosted Castaway deployment target.
- `versioning-and-releases.md`: semver rules, release heuristics, and GitHub release flow.

Repository-level implementation plans live under `/plans`, not under `docs/`.
Repository-level guides live under `docs/guides/`.
Blueprints, roadmaps, and other design/reference documents belong under `docs/` unless they fit a more specific subdirectory.

Reusable operator/agent prompt packs may live under the repo-level `/prompts/` directory when they are operational artifacts rather than normative product documentation.

As active shared docs are added, place them in the most specific fitting location under `docs/`, such as `docs/guides/` for instructional documents or `docs/gameplay/` for gameplay-focused content.
