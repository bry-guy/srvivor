# Castaway Guides

This directory contains shared human-oriented guides for accomplishing concrete tasks with the Castaway codebase and its deployed environments.

Guides are instructional documents.

Use a guide when the goal is to help a person do something step by step, such as:

- bootstrap an environment
- operate a deployed system
- run a release flow
- perform a migration or recovery task
- complete a recurring maintenance action

## Placement

- Shared repository-level guides belong under `docs/guides/`.
- App-specific how-to guidance should stay with the app when it is only relevant to that app.
- Design/reference docs still belong at the top level under `docs/` unless they fit a more specific subdirectory such as `docs/gameplay/`.
- Executable implementation plans still belong under `/plans` or `apps/<app>/plans/`.

## Current guides

- `selfhost-home-k3s-operators-guide.md`: operator workflow for bootstrapping, deploying, and verifying the private home `k3s` environment.
