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
- `non-functional-requirements.md`: cross-cutting security, reliability, and operational requirements.
- `production-readiness-checklist.md`: explicit pre-production checklist across apps.
- `secrets-and-config.md`: shared 1Password/fnox/mise secret and config conventions.
- `selfhost-home-k3s-operators-guide.md`: practical operator guide for bootstrapping, deploying, and verifying the private home `k3s` Castaway environment.
- `selfhost-k3s-deployment-blueprint.md`: structural design blueprint for the first self-hosted Castaway deployment target.
- `versioning-and-releases.md`: semver rules, release heuristics, and GitHub release flow.

Repository-level implementation plans live under `/plans`, not under `docs/`.
Blueprints, roadmaps, and other design/reference documents belong under `docs/`.

Reusable operator/agent prompt packs may live under the repo-level `/prompts/` directory when they are operational artifacts rather than normative product documentation.

As active shared docs are added, place them at the top level under `docs/` unless they belong in a more specific subdirectory such as `docs/gameplay/`.
