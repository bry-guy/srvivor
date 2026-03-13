# castaway bonus points plan

## Goal

Add bonus points to Castaway while keeping them:

- separate from draft points
- additive to participant totals
- traceable to explicit source events
- flexible enough for ponies, Wordle challenges, journeys, and manual corrections

## Scope

### In scope

- `castaway-web` data model and API updates
- leaderboard scoring changes so `draft + bonus = total`
- provenance tracking for bonus awards
- participant/group/event modeling needed for manual Discord-driven gameplay
- `castaway-discord-bot` read/display updates

### Out of scope

- CLI changes
- a fully generic bonus-rules engine in v1
- Discord write workflows for bonus administration

## Current state summary

Today:

- `castaway-web` stores:
  - instances
  - contestants
  - participants
  - draft picks
  - outcome positions
- draft score is computed from draft picks + outcome positions
- the leaderboard returns:
  - `score`
  - `points_available`
- bonus points are explicitly deferred
- the Discord bot assumes `score` is draft score only

Relevant current references:

- `apps/castaway-web/internal/scoring/scoring.go`
- `apps/castaway-web/internal/httpapi/server.go`
- `apps/castaway-web/typespec/main.tsp`
- `apps/castaway-discord-bot/internal/castaway/client.go`
- `apps/castaway-discord-bot/internal/format/format.go`
- `docs/castaway-web-future-work.md`
- `docs/castaway-manual-gameplay-logs.md`

## Recommendation summary

Use a **manual-first, event-driven ledger model**.

That means:

1. keep draft scoring exactly as it is today
2. add first-class bonus events and bonus ledger entries
3. materialize bonus awards per participant in a ledger
4. compute totals as `draft_points + bonus_points`
5. model tribes/groups and event participants so provenance is queryable
6. use event type + metadata for bespoke mechanics instead of building a full rules engine now

This is the lowest-risk design because the current process is already manual in Discord and the journey mechanics are bespoke.

## Functional requirements update

### Scoring

1. The system must track **draft points** and **bonus points** separately.
2. The participant total used for standings must be `draft_points + bonus_points`.
3. Existing draft scoring logic must remain unchanged and independently testable.
4. Bonus points must support positive and negative values so corrections or penalties are possible.
5. `points_available` should remain a **draft-only** concept unless and until bonus-point potential becomes formally predictable.

### Provenance and auditability

6. Every bonus award must be linked to a named source event.
7. The system must record where a bonus award came from, not just the current total.
8. The system must be able to answer:
   - which event created these points?
   - what kind of event was it?
   - who participated?
   - which tribe or participant received points?
   - why were the points awarded?
9. Historical bonus awards must remain stable even if tribe membership changes later.

### Gameplay modeling

10. The system must support tribe-based bonus mechanics.
11. The system must support individual-participant bonus mechanics.
12. The system must support long-lived mappings such as a Castaway tribe’s pony assignment.
13. The system must support bespoke journey events without requiring schema changes per journey game.
14. The system must support manual adjustments as a first-class event type.

### API and bot behavior

15. `castaway-web` must expose bonus-aware leaderboard data.
16. `castaway-web` must expose bonus provenance data for participant-level inspection.
17. The Discord bot must interpret leaderboard rows as `draft + bonus = total`.
18. The Discord bot must display bonus points separately from draft points in `score` and `scores` output.
19. CLI behavior must remain unchanged.

## Proposed data design

### Design principles

- keep draft and bonus scoring independent
- use append-only-ish ledger rows for bonus awards
- model source events explicitly
- expand group awards into participant ledger rows at award time
- prefer typed metadata over over-engineered rule tables in v1

## Proposed tables

### 1. `participant_groups`

Purpose:
- represent Castaway tribes or other participant collections used by bonus gameplay

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `name TEXT NOT NULL`
- `kind TEXT NOT NULL` — e.g. `tribe`, `ad_hoc`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- unique key on `(instance_id, kind, name)`

Notes:
- start with `tribe` as the primary kind
- `metadata` can carry display color, short code, etc.

### 2. `participant_group_memberships`

Purpose:
- record which participants belong to which Castaway tribe/group

Suggested columns:
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `role TEXT NOT NULL DEFAULT 'member'`
- `starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `ends_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- primary key on `(participant_group_id, participant_id, starts_at)`

Notes:
- effective dating is worth adding now if tribe swaps are even remotely possible
- historical awards should still be materialized into the ledger and not re-derived later

### 3. `participant_group_targets`

Purpose:
- store long-lived mappings from a Castaway group to an external gameplay target such as a pony Survivor tribe

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `target_type TEXT NOT NULL` — e.g. `survivor_tribe`
- `target_key TEXT NOT NULL`
- `target_name TEXT NOT NULL`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `ends_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this cleanly models ponies without forcing contestant-tribe history into v1
- if contestant-tribe tracking becomes necessary later, this can be replaced or supplemented by stronger references

### 4. `bonus_events`

Purpose:
- represent the source event that can create one or more bonus awards

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `event_type TEXT NOT NULL` — e.g. `pony_immunity`, `wordle`, `journey`, `manual_adjustment`
- `name TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'completed'` — e.g. `planned`, `open`, `completed`, `cancelled`
- `occurred_at TIMESTAMPTZ NOT NULL`
- `source_ref TEXT` — Discord message link/id or other provenance pointer
- `description TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- `metadata` carries event-specific details like `journey_game = tribal_diplomacy`
- this table is the durable answer to “what source events exist?”

### 5. `bonus_event_group_participants`

Purpose:
- record group-level participation in a bonus event

Suggested columns:
- `bonus_event_id BIGINT NOT NULL REFERENCES bonus_events(id) ON DELETE CASCADE`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `role TEXT NOT NULL` — e.g. `competing_group`, `winning_group`, `recipient_group`
- `result TEXT NOT NULL DEFAULT ''` — e.g. `won`, `lost`, `selected`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- primary key on `(bonus_event_id, participant_group_id, role)`

Notes:
- useful for Wordle and tribe-awarded events
- lets the system explain which tribes were involved even if awards are eventually expanded to participants

### 6. `bonus_event_individual_participants`

Purpose:
- record individual participation in a bonus event

Suggested columns:
- `bonus_event_id BIGINT NOT NULL REFERENCES bonus_events(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `role TEXT NOT NULL` — e.g. `delegate`, `winner`, `loser`, `recipient`
- `result TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- primary key on `(bonus_event_id, participant_id, role)`

Notes:
- this is the main journey-participation table
- `metadata` can hold event-specific outcomes if a journey has custom states

### 7. `bonus_ledger_entries`

Purpose:
- store the actual participant-level bonus points that count toward standings

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `bonus_event_id BIGINT NOT NULL REFERENCES bonus_events(id) ON DELETE CASCADE`
- `source_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `points INTEGER NOT NULL CHECK (points <> 0)`
- `reason TEXT NOT NULL`
- `award_key TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- optional unique index on `(bonus_event_id, participant_id, award_key)` when `award_key` is not null

Notes:
- this is the source of truth for `bonus_points`
- group-awarded points should be expanded into one row per recipient participant
- a correction should be a new ledger row, not an in-place overwrite

## Why this design fits the current gameplay

### Pony immunity

Model:
- Castaway tribes live in `participant_groups`
- tribe membership lives in `participant_group_memberships`
- pony assignment lives in `participant_group_targets`
- each immunity result is a `bonus_event` with `event_type = pony_immunity`
- participant awards are stored in `bonus_ledger_entries`

Flow:
1. create or update pony assignment for each tribe
2. create a `pony_immunity` event with the winning Survivor tribe in metadata
3. find Castaway groups whose active pony matches that winner
4. expand the group award into participant ledger rows

### Wordle

Model:
- the Wordle round is a `bonus_event`
- tribes involved are stored in `bonus_event_group_participants`
- winner tribes are identified by `role/result`
- awards are expanded into participant ledger rows

### Journey / Tribal Diplomacy

Model:
- the journey is a `bonus_event` with `event_type = journey`
- `metadata` includes `journey_game = tribal_diplomacy`
- the selected 3 players are stored in `bonus_event_individual_participants`
- awards are stored in `bonus_ledger_entries`

Why this is important:
- it records both the people who played and the people who got points
- it avoids baking one journey game’s rules directly into the database schema

### Manual adjustments

Model:
- represent a manual correction as its own `bonus_event` with `event_type = manual_adjustment`
- insert one or more positive/negative `bonus_ledger_entries`

Why this matters:
- every point change still has provenance
- the audit trail stays consistent

## Scoring model update

### Current

Leaderboard score today is effectively:

- `score = draft_points`

### Proposed

Leaderboard should become:

- `draft_points` = existing computed draft score
- `bonus_points` = sum of `bonus_ledger_entries.points` for the participant
- `total_points` = `draft_points + bonus_points`
- `draft_points_available` = existing `points_available` concept

### Backward-compatibility recommendation

Add new fields first and keep `score` temporarily as an alias for `total_points` during rollout.

Suggested response shape:

```json
{
  "participant_id": "...",
  "participant_name": "Bryan",
  "score": 26,
  "draft_points": 21,
  "bonus_points": 5,
  "total_points": 26,
  "points_available": 46
}
```

Notes:
- `points_available` should be documented as draft-only
- once clients are migrated, `score` can eventually be deprecated in favor of `total_points`

## API update plan

### Minimal read API changes

### Extend leaderboard rows

Update `GET /instances/:instanceID/leaderboard` to include:
- `draft_points`
- `bonus_points`
- `total_points`
- existing `points_available`

### Add bonus provenance endpoint

Recommended new read route:

- `GET /instances/:instanceID/participants/:participantID/bonus-ledger`

Suggested response contents:
- total bonus points for the participant
- event-linked ledger rows
- event type, event name, occurred_at
- source group if applicable
- reason text
- source ref if present

Optional broader route:
- `GET /instances/:instanceID/bonus-events`
- optional filters by `event_type`, `participant_id`, `group_id`

### Minimal write API changes

Because `castaway-web` is the source of truth, something needs to persist bonus data even if Discord remains read-only.

Recommended admin/write routes:

- `POST /instances/:instanceID/participant-groups`
- `PUT /instances/:instanceID/participant-groups/:groupID/memberships`
- `PUT /instances/:instanceID/participant-groups/:groupID/targets`
- `POST /instances/:instanceID/bonus-events`
- `POST /instances/:instanceID/bonus-events/:eventID/group-participants`
- `POST /instances/:instanceID/bonus-events/:eventID/individual-participants`
- `POST /instances/:instanceID/bonus-events/:eventID/awards`

If that surface feels too large for v1, the fallback is:
- implement the tables and internal service layer now
- expose only the read endpoints publicly
- let early writes happen through seed/admin tooling until a tighter API is finalized

## Discord bot update plan

### Client model changes

Update `apps/castaway-discord-bot/internal/castaway/client.go` to parse:
- `draft_points`
- `bonus_points`
- `total_points`

### Formatting changes

Update existing commands:

### `/castaway score`

Current style:
- `Bryan — 21 points (points available: 46)`

Proposed style:
- `Bryan — 26 total (draft: 21, bonus: 5, draft points available: 46)`

### `/castaway scores`

Current style:
- `1. Bryan — 21 (points available: 46)`

Proposed style:
- `1. Bryan — 26 total (draft: 21, bonus: 5, available: 46)`

## Optional future bot enhancement

If provenance needs to be visible in Discord, add a dedicated command such as:

- `/castaway bonus participant:<name> [instance] [season]`

That command would list the participant’s bonus source events and point rows.

## Implementation plan

### Phase 0: rules confirmation

1. Add the manual gameplay doc.
2. Use `docs/gameplay/journey-tribal-diplomancy.md` as the canonical journey reference and fill any missing resolution details there.
3. Confirm tribe-award semantics:
   - does a tribe earning `+1` mean every member gets `+1`?
4. Confirm whether negative bonus outcomes are possible.
5. Confirm whether Castaway tribe membership can change mid-season.

### Phase 1: data foundation in `castaway-web`

1. Add DB migrations for:
   - `participant_groups`
   - `participant_group_memberships`
   - `participant_group_targets`
   - `bonus_events`
   - `bonus_event_group_participants`
   - `bonus_event_individual_participants`
   - `bonus_ledger_entries`
2. Add `sqlc` queries and generated models.
3. Add repository/service helpers for:
   - active group membership lookup
   - pony resolution
   - bonus ledger aggregation
   - participant bonus history lookup

### Phase 2: scoring and API

1. Refactor scoring so draft scoring remains isolated.
2. Add bonus aggregation into leaderboard generation.
3. Extend TypeSpec models and regenerate OpenAPI.
4. Add read endpoints for bonus-aware leaderboard + participant bonus ledger.
5. Optionally add write/admin endpoints for groups, events, participants, and awards.

### Phase 3: Discord bot

1. Update castaway client models.
2. Update `score` and `scores` formatting.
3. Add tests for bonus-aware formatting.
4. Optionally add a dedicated bonus-breakdown command.

### Phase 4: validation and examples

1. Add migration tests / integration tests for bonus tables.
2. Add scoring tests covering:
   - no bonus points
   - direct individual award
   - tribe-wide award expanded to members
   - manual correction row
   - journey participation + award provenance
3. Add API tests for leaderboard and bonus-ledger responses.
4. Add bot formatter tests for total/draft/bonus rendering.

## Key implementation decisions

### Decision 1: do not build `bonus_rules` first

Even though the earlier future-work sketch mentioned `bonus_rules`, the better v1 is:
- event type + metadata
- explicit participant/group participation rows
- explicit bonus ledger rows

That is simpler, more auditable, and better matched to manual Discord play.

### Decision 2: materialize participant awards

If a tribe wins bonus points, write one ledger row per participant recipient.

Do **not** only store a tribe total and derive participant totals on the fly.

Why:
- historical totals stay stable
- corrections are easier
- provenance is clearer
- leaderboard aggregation stays simple

### Decision 3: keep `points_available` draft-only

Bonus points are currently event-driven and sometimes bespoke.

Until bonus opportunity math becomes formalized, keep `points_available` tied to draft scoring only and label it clearly in the bot/UI.

## Open questions

1. Does a tribe-earned bonus point become `+1` for each current member of that tribe?
2. Can a journey award points to a tribe, an individual, or both?
3. Can journeys or other mechanics subtract points?
4. Can Castaway tribe membership change after the initial assignment?
5. Should the bot expose a dedicated bonus-breakdown command, or is updating `score`/`scores` enough for now?
6. Does `docs/gameplay/journey-tribal-diplomancy.md` need more detail before it can drive metadata examples and tests?
