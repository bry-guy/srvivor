# castaway bonus points blueprint

## Goal

This document is shared design/reference documentation. Executable implementation work belongs in app-local or repository plans.

Add bonus points to Castaway while keeping them:

- separate from draft points
- additive to participant totals
- traceable to explicit gameplay sources
- flexible enough for tribe mechanics, journeys, Wordle-style challenges, and manual corrections
- aware of game time so gameplay can be resolved in real time today and virtualized time later

## Summary recommendation

Use a **temporal activity model** plus a **bonus point ledger**.

The shared document captures cross-app design and product semantics. The concrete `castaway-web` schema draft and migration-oriented implementation plan live in `../apps/castaway-web/plans/bonus-points-planning.md`.

In practice, that means:

1. keep draft scoring logic exactly as it works today
2. add per-instance episode scheduling so each game has its own notion of time
3. represent bonus mechanics as instance activities that can span time
4. represent concrete scoring moments inside those activities as activity occurrences
5. write participant-level bonus awards into a ledger
6. compute bonus totals by summing ledger rows
7. keep rule evaluation in application code, with SQL handling persistence and aggregation

## Teaching note: what a ledger means here

A ledger is **not** a single `bonus_points` number stored on a participant.

A ledger is a list of point transactions.

Instead of storing:

- `participant.bonus_points = 5`

we store rows like:

| participant | source | points | effective_at |
| --- | --- | ---: | --- |
| Bryan | Episode 1 pony immunity | 1 | 2026-03-05T21:00:00-05:00 |
| Bryan | Week 1 tribe Wordle win | 1 | 2026-03-10T18:00:00-05:00 |
| Bryan | Journey attendance | 1 | 2026-03-12T20:15:00-05:00 |
| Bryan | Manual correction | -1 | 2026-03-13T10:00:00-05:00 |

Then:

- `bonus_points = sum(points)`
- here, `1 + 1 + 1 - 1 = 2`

### Why this is useful

A ledger lets the system answer:

- why does this player have 2 bonus points?
- which activity created them?
- when did they become effective in game time?
- who participated in the source activity?
- what correction changed the total later?

It also makes corrections much safer.

If an award was wrong, the system does **not** need to overwrite history. It can add a compensating row such as `-1`.

### Why the ledger should be participant-level

When an activity produces a tribe-level effect, I recommend resolving that effect into **one ledger row per eligible participant**.

Important nuance:
- the activity rule decides who the eligible recipients are
- that may be all current members of a tribe
- or it may be a narrower participant set derived from the activity structure

So if Lotus earns a tribe-derived award, the system should first resolve the intended participant recipients for that activity, then write participant-level rows for those recipients.

That is better than storing only a tribe-level balance because:

- participant totals become trivial to compute
- historical totals stay stable if tribe membership changes later
- the bot can explain point provenance per participant
- corrections stay localized and auditable

## Teaching note: state history is different from a points ledger

There are really two kinds of historical data here:

### 1. State-over-time

Examples:

- which participants were in Lotus during Episode 2?
- which Survivor tribe was Lotus mapped to at that time?
- which activities were active between Episode 1 and Episode 2?

This data is best modeled with **effective time windows**, such as `starts_at` and `ends_at`.

### 2. Point transactions

Examples:

- Bryan earned `+1` from pony immunity
- Riley earned `+1` from journey attendance
- Kate lost `-1` because of a correction

This data is best modeled as a **ledger**.

So:

- group memberships are historical state
- activity assignments are historical state
- bonus awards are ledger entries

They are related, but they are not the same thing.

## Time model recommendation

### Why time needs to be explicit

Your Episode framing is the right model.

The system needs to know both:

- the ordered episode progression of the game, and
- the exact game-time when activities and awards happen

because activities can happen:

- before an episode airs
- exactly when an episode airs
- between episodes
- across a span of time

### Recommended approach

Make time **instance-specific**.

That supports both:

- **real-time play**: episode air times follow the real Survivor schedule in Eastern Time
- **virtualized play**: the same season can be replayed with a compressed or custom schedule

The key idea is:

- an instance owns its own episode schedule
- each instance should get its own copied schedule at creation time
- the current gameplay state is derived from that schedule plus activity timestamps

## Proposed shared data design

### 1. `instance_episodes`

Purpose:
- define the timeline checkpoints for a specific instance

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `episode_number INTEGER NOT NULL CHECK (episode_number >= 0)`
- `label TEXT NOT NULL`
- `airs_at TIMESTAMPTZ NOT NULL`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- unique key on `(instance_id, episode_number)`
- unique key on `(instance_id, airs_at)`

Notes:
- Episode `0` is the pre-air baseline.
- “The instance is at Episode N” means `Episode N.airs_at <= now` for that instance schedule.
- For virtualized play later, just create a different `airs_at` schedule for the same season structure.

### 2. `participant_groups`

Purpose:
- represent reusable participant collections inside an instance, such as tribes

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `name TEXT NOT NULL`
- `kind TEXT NOT NULL` — e.g. `tribe`, `alliance`, `ad_hoc`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- unique key on `(instance_id, kind, name)`

Notes:
- this remains generic and is not bonus-specific

### 3. `participant_group_membership_periods`

Purpose:
- record who belongs to a group over time

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `role TEXT NOT NULL DEFAULT 'member'`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this is historical state, not a points ledger
- use this table to answer who was in a tribe at a given time
- if a participant can only be in one tribe at a time, enforce that rule in code and later with stronger DB constraints if needed

### 4. `instance_activities`

Purpose:
- represent a configured gameplay mechanic running within an instance

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `activity_type TEXT NOT NULL` — e.g. `tribal_pony`, `tribe_wordle`, `journey`, `manual_adjustment`
- `name TEXT NOT NULL`
- `status TEXT NOT NULL DEFAULT 'active'` — e.g. `planned`, `active`, `completed`, `cancelled`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this replaces the earlier `bonus_events` idea
- an activity can span multiple episodes
- an activity does not need to award points every time it exists
- reuse across instances comes from reusing the same `activity_type` on many instance rows

### 5. `activity_group_assignments`

Purpose:
- connect participant groups to an activity over time

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `role TEXT NOT NULL`
- `starts_at TIMESTAMPTZ NOT NULL`
- `ends_at TIMESTAMPTZ`
- `configuration JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this is where generic activity-specific mapping lives
- example: a `tribal_pony` activity can assign a Castaway tribe with configuration such as `{ "pony_survivor_tribe": "kalo" }`
- this avoids creating a pony-specific schema table
- activity configuration changes should happen at explicit episode boundaries, not arbitrary mid-episode timestamps

### 6. `activity_participant_assignments`

Purpose:
- connect individual participants to an activity over time

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

Notes:
- useful for journey delegates, eligible players, and similar mechanics
- this supports activities that involve only some members of a tribe

### 7. `activity_occurrences`

Purpose:
- represent concrete moments or phases inside an activity

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE`
- `occurrence_type TEXT NOT NULL` — e.g. `immunity_result`, `challenge_result`, `journey_attendance`, `journey_resolution`, `manual_correction`
- `name TEXT NOT NULL`
- `effective_at TIMESTAMPTZ NOT NULL`
- `starts_at TIMESTAMPTZ`
- `ends_at TIMESTAMPTZ`
- `status TEXT NOT NULL DEFAULT 'recorded'`
- `source_ref TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- an activity is the long-lived mechanic; an occurrence is the actual scoring moment or phase
- this is the right place to capture the time a result became effective in game state
- examples:
  - Episode 1 immunity result inside `tribal_pony`
  - Week 1 Wordle result inside `tribe_wordle`
  - Journey attendance bonus inside `journey`
  - Tribal Diplomacy resolution inside `journey`

### 8. `activity_occurrence_groups`

Purpose:
- record which groups participated in a specific occurrence and what happened to them

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE`
- `role TEXT NOT NULL`
- `result TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this answers which tribes were involved in a given occurrence
- examples: `winning_tribe`, `competing_tribe`, `recipient_group`

### 9. `activity_occurrence_participants`

Purpose:
- record which individuals participated in a specific occurrence and what happened to them

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `role TEXT NOT NULL`
- `result TEXT NOT NULL DEFAULT ''`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

Notes:
- this answers which players actually took part in a given journey or challenge phase
- examples: `delegate`, `winner`, `sharer`, `stealer`, `risk_taker`

### 10. `bonus_point_ledger_entries`

Purpose:
- store the participant-level bonus points that count toward standings

Suggested columns:
- `id BIGSERIAL PRIMARY KEY`
- `public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`
- `instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE`
- `participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE`
- `activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE`
- `source_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL`
- `entry_kind TEXT NOT NULL` — e.g. `award`, `correction`, `spend`, `conversion`, `reveal`
- `points INTEGER NOT NULL CHECK (points <> 0)`
- `visibility TEXT NOT NULL DEFAULT 'public'` — e.g. `public`, `secret`, `revealed`
- `reason TEXT NOT NULL`
- `effective_at TIMESTAMPTZ NOT NULL`
- `award_key TEXT`
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- optional unique index on `(activity_occurrence_id, participant_id, award_key)` when `award_key` is not null

Notes:
- this is the bonus points source of truth
- totals are computed from this table
- corrections should be new rows, not in-place edits
- spending should also be modeled in this same ledger using negative rows
- `effective_at` supports time-aware state queries
- `visibility` is included because the journey rules mention secret bonus points
- secret entries should remain internal and be excluded from public leaderboard totals until they are revealed or converted at end-of-game
- once authentication exists, a participant should be able to see their own secret entries without exposing them to other players
- if secret points are converted or revealed, prefer explicit balancing rows in the same ledger over mutating old rows in place

## How the model fits the current mechanics

### `tribal_pony`

Model:
- each Castaway tribe is a `participant_group`
- tribe membership lives in `participant_group_membership_periods`
- the `tribal_pony` mechanic is an optional `instance_activity`
- each tribe is attached through `activity_group_assignments`
- the pony mapping lives in `activity_group_assignments.configuration`
- each immunity result is an `activity_occurrence`
- resulting participant bonus awards are written to `bonus_point_ledger_entries`

Notes:
- this should usually be modeled as one long-lived `tribal_pony` activity per instance
- an instance can simply omit this activity entirely if pony scoring is turned off for that run

Example configuration:

- activity: `tribal_pony`
- group assignment for Lotus:
  - `role = tribe`
  - `configuration = { "pony_survivor_tribe": "vatu" }`

Example resolution flow:

1. record an immunity occurrence with metadata such as `{ "winning_survivor_tribes": ["vatu", "kalo"] }`
2. load activity group assignments active at the occurrence time
3. find assigned tribes whose configured pony matches a winning Survivor tribe
4. load the members of those Castaway tribes at the occurrence time
5. write one bonus ledger row per eligible participant

### `tribe_wordle`

Model:
- the challenge is an `instance_activity`
- the actual result is an `activity_occurrence`
- tribe participation and winners are recorded in `activity_occurrence_groups`
- individual submissions or top scorers can be recorded in `activity_occurrence_participants`
- awards are written to the participant ledger

Season 50 gameplay note:
- the challenge is currently scored by averaging the top 3 individual tribe results
- lower guess counts are better
- that means participant occurrence metadata likely needs at least `guess_count`, and occurrence/group metadata likely needs the derived tribe average used to pick winners

### `journey`

Model:
- the journey is an `instance_activity`
- selected delegates are assigned through `activity_participant_assignments`
- attendance, diplomacy resolution, and private risk outcomes can each be separate `activity_occurrences`
- participant choices and results live in `activity_occurrence_participants`
- tribe-level effects live in `activity_occurrence_groups`
- all actual bonus awards still land in `bonus_point_ledger_entries`

Season 50 gameplay note:
- delegates are first selected by tribe vote
- journeys are participant-centric activities
- journey attendance awards each delegate `+1` personal bonus point
- `tribal_diplomacy` is a private choice phase where each delegate chooses `SHARE` or `STEAL`
- the diplomacy outcome can create tribe impacts derived from participant choices
- the optional `lost_for_words` risk can mint `3` secret bonus points, then reduce them according to Wordle guess count, with secret points depleted first

This supports:

- individual bonus points
- tribe-derived bonus points
- multiple phases within one journey
- private and public point awards when needed

### `manual_adjustment`

Model:
- the correction is an `instance_activity` with `activity_type = manual_adjustment`
- the specific fix is an `activity_occurrence`
- the correction rows are written into the bonus ledger as positive or negative entries

Policy:
- corrections should take effect when they are entered
- they should not backdate public scoring history unless a future admin workflow explicitly adds that concept

## Scoring model update

### Current

Leaderboard score today is effectively:

- `score = draft_points`

### Proposed

Leaderboard should expose:

- `draft_points` = existing draft score
- `bonus_points` = sum of visible/public bonus ledger entries only
- `total_points` = `draft_points + bonus_points`
- `points_available` = existing draft-only concept

Secret bonus point policy:
- secret bonus points remain internal during active play
- they are excluded from public leaderboard output
- future authenticated views may expose a participant's own secret point state to that participant only
- secret points may later be consumed by activities or revealed into public bonus points
- at end-of-game, remaining secret bonus points convert into normal bonus points and count toward final scoring

Season 50 gameplay note:
- manual public score posts currently use the format `Jeff: Total (Draft+Bonus)`
- the bot and API should preserve that mental model even if they expose more structured fields

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

## Implementation guidance: code vs SQL

I recommend putting **activity resolution logic in application code**, not SQL.

Use SQL for:

- storing activities, occurrences, assignments, memberships, and ledger rows
- querying active state at a given time
- aggregating bonus totals
- returning provenance and breakdowns

Use Go code for:

- applying the rule for `tribal_pony`
- applying the rule for `tribe_wordle`
- applying the rule for `journey` / `tribal_diplomacy`
- turning a resolved occurrence into participant-level ledger rows

Why:

- the mechanics are bespoke and evolving
- conditional branching is easier to test in Go than in SQL
- it keeps schema generic while rules stay explicit and readable

## API direction

### Minimal read changes

Update `GET /instances/:instanceID/leaderboard` to include:

- `draft_points`
- `bonus_points`
- `total_points`
- existing `points_available`

Recommended new route:

- `GET /instances/:instanceID/participants/:participantID/bonus-ledger`

Suggested response:

- participant bonus total
- ledger rows
- activity type
- activity name
- occurrence type
- occurrence time
- reason
- source group if applicable
- visibility

Optional future route:

- `GET /instances/:instanceID/leaderboard?as_of=<timestamp>`

That would expose the temporal model directly once needed.

### Minimal write direction

Recommended initial persistence surface:

- create/update episode schedules
- create groups and membership periods
- create activities and assignments
- record occurrences
- resolve occurrences into ledger rows

Whether those arrive as public API routes or admin-only tooling can be decided separately.

## Resolved planning decisions

1. Secret bonus points remain hidden during active play and are excluded from public leaderboard totals.
2. Future authenticated views should allow a participant to see their own secret bonus points.
3. Secret bonus points may later be consumed by activities and may become revealed/public upon use.
4. Remaining secret bonus points convert to normal bonus points at end-of-game and count toward final scoring.
5. Corrections take effect when entered, not retroactively.
6. Activity configuration changes should happen at explicit episode boundaries.
7. Each instance should own a copied episode schedule at creation time.
8. `tribal_pony` should usually be modeled as one long-lived optional activity per instance, with many occurrences under it.
9. Spendable bonus points should be modeled as negative rows in the same bonus ledger.

## Remaining open questions

1. Is there any bonus mechanic that needs a recipient that is neither a participant nor a participant group?
