# Bonus Points Expansion Plan

Status: `planning`

## Goal

Extend `castaway-web` beyond draft placement scoring so it can persist, resolve, and expose bonus gameplay.

## Planning note

This document intentionally includes both the **schema draft** and the **implementation plan**.

That keeps the work inside the normal `plans/` structure instead of creating a bespoke documentation category for schema design.

## Primary references

- `../../../docs/castaway-bonus-points-plan.md`
- `../../../docs/castaway-manual-gameplay-logs.md`
- `../../../docs/gameplay/journey-tribal-diplomancy.md`
- `../functional-requirements.md`

## Confirmed product decisions

The current planning baseline is:

- bonus points remain separate from draft points
- public leaderboard totals are `draft_points + visible bonus_points`
- secret bonus points remain internal during active play
- future authenticated views should let a participant see their own secret points
- remaining secret bonus points convert into normal bonus points at end-of-game
- spending bonus points should be modeled as negative rows in the same bonus ledger
- corrections take effect when entered, not retroactively
- tribe membership can change over time
- activity mappings/configuration change only at explicit episode boundaries
- each instance owns its own copied episode schedule
- `tribal_pony` is a long-lived optional activity and can simply be omitted from an instance where pony scoring is disabled
- journeys are participant-centric activities that may still create tribe impacts
- bot scope for the first slice is limited to updating existing `score` / `scores` output

## Planning assumptions still to confirm

1. whether the first implementation only needs admin/public write support, not Discord-driven writes
2. whether any future bonus mechanic needs recipients other than participants or participant groups

## Implementation shape

The current recommendation is to implement bonus gameplay in `castaway-web` using:

- an instance-specific episode schedule
- generic participant groups with time-bound memberships
- generic instance activities with time-bound assignments
- activity occurrences for concrete scoring moments
- a participant-level bonus point ledger

Rule resolution should happen in Go application code, not SQL.

SQL should handle:

- persistent state
- time-aware lookups
- bonus aggregation
- provenance queries

## Schema draft

### Schema conventions

The draft should follow existing `castaway-web` database conventions:

- primary keys use `BIGSERIAL`
- externally referencable resources use `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- timestamps use `TIMESTAMPTZ`
- `created_at` defaults to `NOW()`
- mutable resources also get `updated_at`
- flexible mechanic-specific detail lives in `JSONB`

### 1. `instance_episodes`

Purpose:
- define the time checkpoints for a specific instance

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `episode_number INTEGER NOT NULL CHECK (episode_number >= 0)`
- `label TEXT NOT NULL`
- `airs_at TIMESTAMPTZ NOT NULL`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `UNIQUE (instance_id, episode_number)`
- `UNIQUE (instance_id, airs_at)`
- index on `(instance_id, airs_at)`

Notes:
- Episode `0` is the pre-air baseline.
- Every instance should receive its own copied schedule at creation time.
- Initial real-time schedule defaults can come from season-backed app config or seeded data; a separate schedule-template table is not required in the first slice.

### 2. `participant_groups`

Purpose:
- represent reusable collections such as tribes

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `name TEXT NOT NULL`
- `kind TEXT NOT NULL`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `UNIQUE (instance_id, kind, name)`
- index on `(instance_id, kind)`

Notes:
- `kind` examples: `tribe`, `alliance`, `ad_hoc`
- this table is generic and not bonus-specific

### 3. `participant_group_membership_periods`

Purpose:
- track who belongs to which group over time

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `role TEXT NOT NULL DEFAULT 'member'`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `CHECK (ends_at IS NULL OR ends_at > starts_at)`
- `UNIQUE (participant_group_id, participant_id, role, starts_at)`
- index on `(participant_group_id, starts_at)`
- index on `(participant_id, starts_at)`

Notes:
- this is historical state, not a ledger
- membership changes should be recorded at explicit episode boundaries
- single-tribe-at-a-time behavior can be enforced in application code first

### 4. `instance_activities`

Purpose:
- represent configured gameplay mechanics running inside an instance

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `activity_type TEXT NOT NULL`
- `name TEXT NOT NULL`
- `status TEXT NOT NULL CHECK (status IN ('planned', 'active', 'completed', 'cancelled')) DEFAULT 'active'`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `CHECK (ends_at IS NULL OR ends_at > starts_at)`
- index on `(instance_id, activity_type)`
- index on `(instance_id, starts_at)`

Notes:
- examples: `tribal_pony`, `tribe_wordle`, `journey`, `manual_adjustment`
- an instance can omit `tribal_pony` entirely if that mechanic is disabled for that run
- activities are long-lived configuration containers; they are not the same thing as scoring moments

### 5. `activity_group_assignments`

Purpose:
- attach participant groups to activities over time

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `role TEXT NOT NULL`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `configuration JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `CHECK (ends_at IS NULL OR ends_at > starts_at)`
- `UNIQUE (activity_id, participant_group_id, role, starts_at)`
- index on `(activity_id, starts_at)`
- index on `(participant_group_id, starts_at)`

Notes:
- this is the generic home for activity-specific mappings
- example: a `tribal_pony` activity can assign a tribe with configuration such as `{ "pony_survivor_tribe": "vatu" }`
- configuration changes should happen at explicit episode boundaries

### 6. `activity_participant_assignments`

Purpose:
- attach individual participants to activities over time

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `role TEXT NOT NULL`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `configuration JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `CHECK (ends_at IS NULL OR ends_at > starts_at)`
- `UNIQUE (activity_id, participant_id, role, starts_at)`
- index on `(activity_id, starts_at)`
- index on `(participant_id, starts_at)`

Notes:
- this is useful for journey delegates, opt-ins, risks, and other participant-first mechanics

### 7. `activity_occurrences`

Purpose:
- represent concrete moments or phases inside an activity

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE`
- `occurrence_type TEXT NOT NULL`
- `name TEXT NOT NULL`
- `effective_at TIMESTAMPTZ NOT NULL`
- `starts_at TIMESTAMPTZ`
- `ends_at TIMESTAMPTZ`
- `status TEXT NOT NULL CHECK (status IN ('recorded', 'resolved', 'cancelled')) DEFAULT 'recorded'`
- `source_ref TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)`
- index on `(activity_id, effective_at)`
- index on `(activity_id, occurrence_type)`

Notes:
- examples: `immunity_result`, `challenge_result`, `journey_attendance`, `journey_resolution`, `secret_risk_result`, `manual_correction`
- this is the time anchor for bonus resolution

### 8. `activity_occurrence_groups`

Purpose:
- record group participation/results for a specific occurrence

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `role TEXT NOT NULL`
- `result TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `UNIQUE (activity_occurrence_id, participant_group_id, role)`
- index on `(activity_occurrence_id)`
- index on `(participant_group_id)`

Notes:
- examples: `competing_group`, `winning_group`, `recipient_group`

### 9. `activity_occurrence_participants`

Purpose:
- record individual participation/results for a specific occurrence

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `role TEXT NOT NULL`
- `result TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- `UNIQUE (activity_occurrence_id, participant_id, role)`
- index on `(activity_occurrence_id)`
- index on `(participant_id)`

Notes:
- examples: `delegate`, `winner`, `sharer`, `stealer`, `risk_taker`
- this is where Wordle guess counts and Tribal Diplomacy choices should live

### 10. `bonus_point_ledger_entries`

Purpose:
- store participant-level bonus point changes

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `source_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `entry_kind TEXT NOT NULL CHECK (entry_kind IN ('award', 'correction', 'spend', 'conversion', 'reveal'))`
- `points INTEGER NOT NULL CHECK (points <> 0)`
- `visibility TEXT NOT NULL CHECK (visibility IN ('public', 'secret', 'revealed')) DEFAULT 'public'`
- `reason TEXT NOT NULL`
- `effective_at TIMESTAMPTZ NOT NULL`
- `award_key TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Suggested constraints/indexes:
- partial unique index on `(activity_occurrence_id, participant_id, award_key)` where `award_key IS NOT NULL`
- index on `(instance_id, participant_id, effective_at)`
- index on `(instance_id, visibility, effective_at)`
- index on `(activity_occurrence_id)`

Notes:
- this is the source of truth for participant bonus balances
- both earning and spending should use this same ledger
- spending uses negative rows
- corrections use new rows instead of mutating old ones
- conversions/reveals should also use explicit balancing rows instead of mutating older secret rows in place

## Mechanic mapping examples

### `tribal_pony`

- one long-lived optional `instance_activity`
- tribe-to-pony mappings live in `activity_group_assignments.configuration`
- each immunity result is an `activity_occurrence`
- winner tribes are resolved in Go code
- resulting participant awards are written into `bonus_point_ledger_entries`

### `tribe_wordle`

- one activity per recurring Wordle mechanic or per challenge series, depending on admin ergonomics
- each actual challenge result is an `activity_occurrence`
- participant results store raw `guess_count` in occurrence participant metadata
- the winning tribe is derived from the top-3 average logic in Go code
- awards are written into the participant ledger

### `journey`

- one activity for the journey arc or one per journey instance; either is acceptable, but one activity per journey instance is likely simpler operationally
- delegate selection is recorded through assignments or occurrence-participant rows
- attendance, diplomacy resolution, and secret risk resolution are separate occurrences
- diplomacy choices live in occurrence participant metadata/results
- tribe impacts are resolved from participant choices in Go code
- secret points and any later spends still flow through the same ledger

## Planned API changes

### Extend leaderboard rows

`GET /instances/:instanceID/leaderboard` should eventually expose:

- `draft_points`
- `bonus_points` (visible/public only)
- `total_points`
- existing `points_available`

For compatibility during rollout:

- keep `score` as an alias for `total_points`

### Add participant bonus provenance route

Recommended read route:

- `GET /instances/:instanceID/participants/:participantID/bonus-ledger`

Response should include:

- visible bonus total
- optionally internal secret total for authenticated/self views later
- ledger rows with occurrence provenance
- activity type
- occurrence type
- reason
- visibility

### Write surface recommendation

First implementation does not need every concept exposed through public CRUD.

A practical first slice is:

- persist state through internal/admin routes or admin tooling
- expose read-only public routes first
- keep Discord read-only

## Implementation plan

### Phase 0: migration and API contract prep

1. finalize public vs internal visibility rules in TypeSpec comments and plan notes
2. decide whether first write workflows are HTTP admin routes or internal tooling
3. add any missing season-backed episode schedule defaults needed for instance creation

### Phase 1: temporal foundations

1. add migration for `instance_episodes`
2. update instance creation flow to copy a season schedule into `instance_episodes`
3. add queries for:
   - list episodes by instance
   - resolve current episode by timestamp
   - resolve episode boundary windows
4. add tests around Episode `0` and current-episode lookup

### Phase 2: group and activity state

1. add migrations for:
   - `participant_groups`
   - `participant_group_membership_periods`
   - `instance_activities`
   - `activity_group_assignments`
   - `activity_participant_assignments`
2. add `sqlc` queries for create/list/as-of lookups
3. add service helpers for:
   - active group memberships at a timestamp
   - active activity group assignments at a timestamp
   - active activity participant assignments at a timestamp
4. add tests for explicit episode-boundary changes

### Phase 3: occurrences and ledger

1. add migrations for:
   - `activity_occurrences`
   - `activity_occurrence_groups`
   - `activity_occurrence_participants`
   - `bonus_point_ledger_entries`
2. add `sqlc` queries for create/list/provenance/aggregate paths
3. add service helpers for:
   - visible bonus total by participant
   - secret bonus total by participant
   - visible bonus total as-of a timestamp
   - available secret balance by participant
4. add tests for positive, negative, secret, and revealed rows

### Phase 4: rule resolution services in Go

Implement a resolver layer that takes an occurrence plus current effective state and writes ledger rows.

Initial resolvers:

- `tribal_pony`
- `tribe_wordle`
- `journey`
- `manual_adjustment`

Expected behavior:

- each resolver decides recipient expansion logic
- each resolver writes participant-level ledger rows
- spend/correction/conversion behavior uses the same ledger with negative or offsetting rows as needed
- no resolver mutates historical ledger rows in place

### Phase 5: leaderboard integration

1. keep existing draft scoring isolated in `internal/scoring`
2. aggregate visible bonus totals in the leaderboard handler
3. expose `draft_points`, `bonus_points`, `total_points`
4. keep `points_available` draft-only
5. keep `score` as `total_points` during compatibility window
6. do not expose secret bonus points in public leaderboard responses

### Phase 6: bonus provenance read API

1. add participant bonus-ledger route
2. return occurrence/activity provenance for each ledger row
3. structure the handler so future auth can optionally include self-visible secret rows
4. keep unauthenticated/public responses limited to visible/public entries

### Phase 7: Discord bot update

1. update castaway client models in `apps/castaway-discord-bot`
2. update `score` formatting to show total, draft, and visible bonus
3. update `scores` formatting similarly
4. do not add a dedicated bonus command in the first slice

### Phase 8: testing and backfill support

Add tests for:

- instance schedule copy on creation
- group membership changes at episode boundaries
- pony mapping changes at episode boundaries
- `tribal_pony` immunity award resolution
- Wordle top-3-average winner calculation
- journey attendance award resolution
- Tribal Diplomacy tribe-impact resolution
- secret point issuance and hidden visibility
- spending secret/public points through negative ledger rows
- end-of-game secret-to-public conversion
- correction entry timing
- leaderboard aggregation with draft + visible bonus totals

## Agent execution slices

This section is intended to make the plan directly executable by coding agents.

Rules for execution:

- complete slices in order
- keep each slice independently reviewable and committable
- regenerate derived artifacts inside the same slice that changes their sources
- do not move to the next slice until validation for the current slice passes

### Slice 1: episode schedule foundation

Primary files:

- `apps/castaway-web/db/migrations/005_instance_episodes.sql`
- `apps/castaway-web/db/query/episodes.sql`
- `apps/castaway-web/internal/db/*` generated via `sqlc`
- `apps/castaway-web/internal/httpapi/server.go`
- `apps/castaway-web/internal/app/*`
- `apps/castaway-web/internal/seeddata/*` if schedule defaults live in seed-backed data
- `apps/castaway-web/internal/httpapi/*_test.go`

Deliverables:

- `instance_episodes` migration
- schedule copy on instance creation
- read/query helpers for episode lookups
- tests for Episode `0` and current-episode resolution

Validation:

- `cd apps/castaway-web && mise run sqlc`
- `cd apps/castaway-web && mise run lint`
- `cd apps/castaway-web && mise run test`
- `cd apps/castaway-web && mise run build`

### Slice 2: group and activity state

Primary files:

- `apps/castaway-web/db/migrations/006_groups_and_activities.sql`
- `apps/castaway-web/db/query/groups.sql`
- `apps/castaway-web/db/query/activities.sql`
- `apps/castaway-web/internal/db/*` generated via `sqlc`
- new or updated service files under `apps/castaway-web/internal/`
- relevant tests under `apps/castaway-web/internal/**/*_test.go`

Deliverables:

- participant groups
- membership periods
- instance activities
- activity group/participant assignments
- as-of lookup helpers for active state
- episode-boundary change tests

Validation:

- `cd apps/castaway-web && mise run sqlc`
- `cd apps/castaway-web && mise run lint`
- `cd apps/castaway-web && mise run test`
- `cd apps/castaway-web && mise run build`

### Slice 3: occurrences and bonus ledger persistence

Primary files:

- `apps/castaway-web/db/migrations/007_activity_occurrences_and_bonus_ledger.sql`
- `apps/castaway-web/db/query/activity_occurrences.sql`
- `apps/castaway-web/db/query/bonus_ledger.sql`
- `apps/castaway-web/internal/db/*` generated via `sqlc`
- new or updated aggregation/provenance helpers under `apps/castaway-web/internal/`
- relevant tests under `apps/castaway-web/internal/**/*_test.go`

Deliverables:

- occurrence persistence
- group/participant occurrence joins
- ledger persistence with `entry_kind`, `visibility`, and negative row support
- visible/secret aggregate queries
- tests for public, secret, spend, correction, and conversion rows

Validation:

- `cd apps/castaway-web && mise run sqlc`
- `cd apps/castaway-web && mise run lint`
- `cd apps/castaway-web && mise run test`
- `cd apps/castaway-web && mise run build`

### Slice 4: activity resolvers in Go

Primary files:

- new resolver/service files under `apps/castaway-web/internal/`
- `apps/castaway-web/internal/httpapi/server.go` or adjacent handlers if resolution endpoints/tooling are added
- resolver tests under `apps/castaway-web/internal/**/*_test.go`

Deliverables:

- resolver framework for occurrence -> ledger rows
- initial resolvers for:
  - `tribal_pony`
  - `tribe_wordle`
  - `journey`
  - `manual_adjustment`
- tests covering recipient expansion, tribe impacts, and negative-row behavior

Validation:

- `cd apps/castaway-web && mise run lint`
- `cd apps/castaway-web && mise run test`
- `cd apps/castaway-web && mise run build`

### Slice 5: public read API and contract updates

Primary files:

- `apps/castaway-web/typespec/main.tsp`
- `apps/castaway-web/openapi/openapi.yaml`
- `apps/castaway-web/internal/httpapi/server.go`
- `apps/castaway-web/internal/scoring/scoring.go`
- `apps/castaway-web/internal/httpapi/*_test.go`
- `apps/castaway-web/internal/scoring/*_test.go`

Deliverables:

- leaderboard rows with `draft_points`, `bonus_points`, `total_points`
- `score` retained as compatibility alias
- participant bonus-ledger read route
- public responses exclude secret entries
- TypeSpec/OpenAPI updated to match

Validation:

- `cd apps/castaway-web && mise run openapi`
- `cd apps/castaway-web && mise run openapi-check`
- `cd apps/castaway-web && mise run lint`
- `cd apps/castaway-web && mise run test`
- `cd apps/castaway-web && mise run build`

### Slice 6: Discord bot read-model update

Primary files:

- `apps/castaway-discord-bot/internal/castaway/client.go`
- `apps/castaway-discord-bot/internal/format/format.go`
- `apps/castaway-discord-bot/internal/format/format_test.go`
- any bot handler tests affected by leaderboard field changes

Deliverables:

- bot parses `draft_points`, `bonus_points`, `total_points`
- `score` and `scores` render total + draft + visible bonus
- no dedicated bonus command added yet

Validation:

- `cd apps/castaway-discord-bot && mise run lint`
- `cd apps/castaway-discord-bot && mise run test`
- `cd apps/castaway-discord-bot && mise run build`

### Slice 7: final integration validation

Validation:

- `cd apps/castaway-web && mise run lint && mise run test && mise run build`
- `cd apps/castaway-discord-bot && mise run lint && mise run test && mise run build`
- from repo root, run the relevant monorepo CI check before merge

## Suggested migration order

1. `instance_episodes`
2. `participant_groups`
3. `participant_group_membership_periods`
4. `instance_activities`
5. `activity_group_assignments`
6. `activity_participant_assignments`
7. `activity_occurrences`
8. `activity_occurrence_groups`
9. `activity_occurrence_participants`
10. `bonus_point_ledger_entries`

This keeps state foundations in place before point resolution tables.

## Non-goals for the first implementation slice

- a full user-facing rules engine
- generic public CRUD for every activity concept on day one
- Discord-native write administration
- exposing secret bonus points publicly
- solving all future spend mechanics beyond recording spend as ledger rows
