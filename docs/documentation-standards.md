# Documentation Standards

This document defines the minimum documentation layout for Castaway globally and for each app under `apps/`.

## Required structure

Castaway (global), and each app, should include _at least_ the following structure and documentation:

- `README.md`: human-facing documentation that focuses on how to bring up, interact with, use, and administer the app or repository scope.
- `plans/`: directory containing specific, well-named implementation plans with statuses of `planning`, `in-progress`, or `done`.
- `non-functional-requirements.md`: security, reliability, and operational requirements.
- `functional-requirements.md`: feature specifications, including inputs and outputs.
- `production-readiness-checklist.md`: production requirements.

## Additional documentation

- `CHANGELOG.md` should be present for each app and for any scope that is intentionally versioned or released.
- `docs/README.md` remains the shared documentation map for the repository-level `docs/` directory.
- If docs are added for a separate concern, give them their own well-named file.
- Shared cross-app documentation belongs in the repo-level `docs/` directory unless it is clearly app-local.
- Use a plan for executable implementation work; use a blueprint or roadmap for design, structure, or future-oriented reference documentation.
- Use a guide for human-oriented instructions that explain how to accomplish a concrete task with the repository, app, or deployed environment.
- Reusable operator or agent prompt packs may live under the repo-level `prompts/` directory when they are operational artifacts rather than normative product documentation.

## Placement rules

- App-specific minimum documentation should live at the root of each app directory.
- App-specific planning documents should live under `apps/<app>/plans/`.
- Shared repository implementation plans should live under the repo-level `plans/` directory.
- Shared repository guides should live under `docs/guides/`.
- Shared repository blueprints, roadmaps, and other design/reference documents should live under the repo-level `docs/` directory unless they belong in a more specific subdirectory.
- Keep `README.md` focused on developer and operator entrypoints; use dedicated documents for deeper concerns.
