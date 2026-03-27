# Activities + Occurrences API Plan

Status: done
Owner: castaway-web
Last updated: 2026-03-27

## Goal

Allow operators/clients to create gameplay activities, record occurrence results, and resolve those results into bonus ledger entries through HTTP APIs, without changing the current database schema.

## Why now

- The data model already supports activities, occurrences, occurrence participants/groups, and bonus ledger entries.
- Seed/import code and gameplay resolver logic already exercise these pathways internally.
- There is currently read-only API coverage for bonus ledger totals, but no write API for activities/results.

## Non-goals

- No underlying data model redesign.
- No generic rule engine redesign.
- No Discord-driven write workflows in this plan; bot scope is read-only fetch/display for activities and occurrences.

## Current capabilities (already implemented in code)

- DB queries exist for:
  - `instance_activities`
  - `activity_occurrences`
  - `activity_occurrence_participants`
  - `activity_occurrence_groups`
  - `bonus_point_ledger_entries`
- Resolver exists:
  - `internal/gameplay/resolver.go` (`ResolveActivityOccurrence`)
- Seed flow already uses these APIs internally:
  - creates activity + occurrence
  - records participants
  - optionally resolves to ledger entries

## Proposed API surface (minimal, sufficient)

These routes serve two consumers:
- operators/admin tooling that create and resolve activities
- the Discord bot, which needs read access to list activities and occurrences for an instance

### Activities

- `POST /instances/{instanceID}/activities`
  - Create activity (`activity_type`, `name`, `status`, `starts_at`, optional `ends_at`, optional `metadata`)
- `GET /instances/{instanceID}/activities`
  - List activities for instance

### Occurrences

- `POST /activities/{activityID}/occurrences`
  - Create occurrence (`occurrence_type`, `name`, `effective_at`, optional `starts_at`, optional `ends_at`, `status`, optional `source_ref`, optional `metadata`)
- `GET /activities/{activityID}/occurrences`
  - List occurrences for activity

### Occurrence results

- `POST /occurrences/{occurrenceID}/participants`
  - Record participant result (`participant_id`, optional `participant_group_id`, `role`, optional `result`, optional `metadata`)
- `POST /occurrences/{occurrenceID}/groups`
  - Record group result (`participant_group_id`, `role`, optional `result`, optional `metadata`)

### Resolve

- `POST /occurrences/{occurrenceID}/resolve`
  - Invoke gameplay resolver and create bonus ledger entries
  - Response includes created ledger entries and count

## Implementation plan

1. **TypeSpec/OpenAPI updates**
   - Add request/response models and routes in `apps/castaway-web/typespec/main.tsp`.
   - Regenerate `apps/castaway-web/openapi/openapi.yaml`.

2. **HTTP handlers**
   - Add routes + handlers in `apps/castaway-web/internal/httpapi/server.go`.
   - Reuse existing generated `db` query methods.
   - Use `gameplay.NewService(...).ResolveActivityOccurrence(...)` for resolve endpoint.

3. **Validation and error mapping**
   - Validate UUID path params and required fields.
   - Preserve current DB constraint behavior and map:
     - not found -> 404
     - uniqueness/constraint violations -> 409 or 400 as appropriate
     - unexpected -> 500

4. **Transaction boundaries**
   - For complex write operations (especially resolve), run inside transaction.
   - Ensure partial writes do not occur on failure.

5. **Idempotency behavior**
   - Duplicate resolve attempts should be explicit:
     - either reject as conflict if already resolved,
     - or tolerate duplicates by returning conflict details from award-key constraints.
   - Document expected behavior in OpenAPI.

6. **Tests**
   - Add/extend integration tests in `internal/httpapi/server_integration_test.go`:
     - create activity
     - create occurrence
     - record participant result metadata
     - resolve occurrence
     - assert bonus ledger + leaderboard effects
     - duplicate resolve behavior

7. **Discord bot read support**
   - Add `castaway-discord-bot` client methods for:
     - list activities for an instance
     - list occurrences for an activity
   - Add Discord handlers/commands for read-only fetches, reusing existing instance-resolution behavior.
   - Keep the first slice intentionally narrow:
     - one command to list activities for the active/selected instance
     - one command to list occurrences for a selected activity
   - Format output for Discord message limits and include key fields only:
     - activity/occurrence name
     - type
     - status
     - effective time

8. **Docs and examples**
   - Update `apps/castaway-web/README.md` API list with new endpoints.
   - Add curl examples for:
     - manual adjustment occurrence
     - secret risk result occurrence
     - resolve call
   - Document bot read commands in `apps/castaway-discord-bot/README.md` once implemented.

9. **Verification gates**
   - Run:
     - `mise run //apps/castaway-web:lint`
     - `mise run //apps/castaway-web:test`
     - `mise run //apps/castaway-web:build`
     - `mise run //apps/castaway-web:openapi-check`
     - `mise run //apps/castaway-discord-bot:lint`
     - `mise run //apps/castaway-discord-bot:test`
     - `mise run //apps/castaway-discord-bot:build`

## Rollout sequence

- Phase 1: Create/list activities and occurrences, record participant/group results.
- Phase 2: Add resolve endpoint for controlled automated ledger entry creation.
- Phase 3: Add Discord bot read support for listing activities and occurrences.
- Phase 4: Optional admin tooling/UX improvements beyond the bot read slice.

## Risks

- Duplicate resolution attempts can conflict on `(activity_occurrence_id, participant_id, award_key)` uniqueness.
- Occurrence metadata shape is activity-type-specific; poor validation may allow malformed payloads.

## Mitigations

- Add resolver-specific validation before write when feasible.
- Return structured conflict responses that indicate duplicate award keys.
- Add integration tests per supported activity type path used today (`manual_adjustment`, `journey`).
