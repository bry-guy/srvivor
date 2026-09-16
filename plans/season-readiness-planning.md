# Season Readiness Rehabilitation

Status: `in-progress`
Owner: castaway-web + castaway-discord-bot
Last updated: 2026-09-16

## Goal

Repair the obviously broken gameplay paths and establish a small, executable baseline for the mechanics used last season. This work prepares a later operator CLI and deterministic season scripting effort; it does not implement either one.

## Keep

- Narrow HTTP clock injection and fixed test fixtures.
- Discord entity-routing and season-option corrections.
- Episode API contract corrections.
- Focused PostgreSQL integration coverage for gameplay flows.
- Small pure validators where they enforce a concrete schedule invariant.

## Fix now

1. Make Stir the Pot close use one database transaction and preserve the public-UUID/internal-BIGINT boundary.
2. Keep secret ledger spends secret at close; reveal secret points only when a spend actually requires it.
3. Add PostgreSQL coverage for the close result, ledger visibility, unchanged balances, and failure rollback.
4. Build a compact mechanics matrix for the last-season rules actually used:
   - Stir the Pot
   - Tribal Pony
   - pony auction and ownership
   - secret-point accounting
5. Resolve material discrepancies against the gameplay logs and documents, and add one representative executable scenario for each settled mechanic.

## Last-season baseline

This is a compact inventory, not a redesign. A mechanic is considered covered when the canonical path and a representative regression agree with the documented or observed rule.

| Mechanic | Canonical path | Regression coverage | Disposition |
| --- | --- | --- | --- |
| Stir the Pot | `httpapi/merge_gameplay.go`, `gameplay/resolver.go` | `TestMergeGameplayVerificationFlow`, Stir the Pot resolver tests | Repair close transaction and secret visibility; retain |
| Tribal Pony | `gameplay/resolver.go` | `TestResolveActivityOccurrenceTribalPony*` | Retain |
| Pony auction and ownership | `httpapi/merge_gameplay.go` | `TestRecordMergeAuctionResults_*`, merge gameplay flow | Retain; keep public-first spending |
| Secret-point accounting | `httpapi/merge_gameplay.go`, `db/query/bonus_ledger.sql` | secret-risk, contribution, and auction integration tests | Repair only confirmed discrepancies |
| Loan Shark | `httpapi/merge_gameplay.go` | merge gameplay flow | Defer unless a concrete broken behavior is found |

Observed rules and historical evidence remain in `docs/gameplay/` and `docs/castaway-manual-gameplay-logs.md`. Material discrepancies must be resolved before adding new behavior.

## Defer

- A season-operator CLI and scripting DSL.
- Upcoming-season schedule/configuration machinery.
- Event sourcing or a broad service-layer extraction.
- Large inventory/preflight packages and containerized operational wrappers.
- Backup, credential, and deployment automation already owned by the infrastructure repository.
- Loan Shark changes unless a concrete broken behavior is found.

## Acceptance criteria

- The affected PostgreSQL integration tests run against a disposable database rather than being silently skipped.
- Closing Stir the Pot cannot leave a closed round partially written.
- The regression baseline documents which last-season rules are encoded and which remain unresolved.
- Future CLI work can call the existing canonical gameplay operations instead of duplicating rules.
