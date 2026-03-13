# srvivor Non-Functional Requirements

## Reliability

- Scoring output must remain deterministic for a given draft, roster, and final placement set.
- Historical regression behavior must be preserved unless intentionally changed.
- Local workflows must remain runnable through `mise`.

## Quality

- Canonical roster validation must remain available for name normalization workflows.
- Changes should preserve backwards compatibility for existing local draft files where feasible.
- Existing regression coverage should remain intact.

## Operations

- The CLI should remain easy to run locally without service dependencies.
- Build, test, and lint workflows must remain documented and reproducible.
- The CLI is in maintenance/archive mode, so changes should be conservative and well-documented.

## Security

- The CLI should not require committed secrets for normal development or use.
- Local file modifications must stay explicit and user-invoked.
