# Seed First-Class Bonus Activities

Status: done
Owner: castaway-web
Last updated: 2026-03-27

## Goal

Upgrade historical season seeding so Season 50 bonus gameplay is seeded as first-class gameplay data (`participant_groups`, memberships, `tribal_pony`, `tribe_wordle`, and `journey` occurrences) rather than relying on manual adjustment backfill entries for pony/wordle outcomes.

## Why

Current seed data captures correct scores, but it encodes some gameplay results as manual corrections. This loses gameplay provenance and prevents replaying resolver logic directly from seeded structures.

## Scope

In scope:
- Extend seed schema/types to support participant groups and group memberships.
- Extend seeding pipeline to create groups/memberships before activity seeding.
- Seed `tribal_pony` activity + group assignments and immunity occurrences.
- Seed `tribe_wordle` activity + participant/group result rows and resolve through gameplay resolver.
- Keep journey/Lost for Words seeded as occurrence-driven data.

Out of scope:
- API design changes for admin write endpoints (tracked separately).
- New scoring mechanics beyond currently implemented resolver behavior.

## Proposed data model additions in seed JSON

Add optional top-level season fields:
- `participant_groups[]`
  - `name`
  - `kind` (e.g. `tribe`)
  - `metadata`
  - `memberships[]` with `participant_name`, `role`, `starts_at`, optional `ends_at`, optional `metadata`
- `activity_group_assignments[]` under each activity
  - `participant_group_name`
  - `role`
  - `starts_at`, optional `ends_at`
  - `configuration`

Keep existing `activities[].occurrences[].participants[]` for occurrence-level results.

## Implementation plan

1. **Seed type extensions**
   - Update `internal/seeddata/seeddata.go` structs for groups and activity group assignments.
   - Keep backward compatibility for historical seasons that do not include new fields.

2. **Seed application pipeline**
   - In `internal/app/seed.go`, add a `seedParticipantGroups(...)` phase before `seedActivityHistory(...)`.
   - Resolve participant/group names case-insensitively with clear errors on missing references.

3. **Activity assignment seeding**
   - During activity seeding, apply `activity_group_assignments` rows.
   - Validate assignment time boundaries against episode boundaries using gameplay service helpers.

4. **Resolver-driven bonus generation**
   - For `tribal_pony` and `tribe_wordle`, seed occurrences with `resolve: true` and let resolver create ledger entries.
   - Eliminate manual backfill occurrences that represent pony/wordle outcomes.

5. **Season 50 seed migration**
   - Replace manual pony/wordle backfill entries with:
     - `participant_groups`: Tangerine/Leaf/Lotus + memberships
     - `tribal_pony` activity with pony mapping assignments and ep1/ep2/ep3 occurrences
     - `tribe_wordle` activity with week-2 occurrence participant results
   - Keep manual journey attendance/diplomacy adjustments only if still required for historical parity.

6. **Tests**
   - Update `internal/seeddata/seeddata_test.go` expectations for Season 50 shape.
   - Add integration coverage in `internal/app/seed_integration_test.go` (gated by test DB) to verify:
     - groups/memberships created
     - resolver creates expected pony and wordle ledger entries
     - leaderboard bonus totals match expected values

7. **Documentation**
   - Update `apps/castaway-web/README.md` seed notes to describe first-class bonus activity seeding.
   - Update planning docs if manual backfill assumptions are removed.

## Risks

- Name-based mapping drift between participant names and seed references.
- Time boundary mismatches for group memberships/assignments.
- Duplicate ledger entries if `resolve` is re-run without idempotent constraints in seed logic.

## Mitigations

- Normalize and validate participant/group references before insert.
- Reuse existing episode-boundary checks from gameplay service.
- Continue relying on award-key uniqueness constraints and deterministic seed award keys.

## Exit criteria

- Season 50 seeds reproduce current intended leaderboard and ledger behavior.
- Pony and wordle points are generated from resolver-backed activities/occurrences, not manual snapshot backfills.
- `mise run //apps/castaway-web:lint`, `test`, and `build` pass with updated seed tests.
