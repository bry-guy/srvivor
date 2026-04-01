# Stir the Pot Tribe Totals Plan

Status: `planning`
Owner: castaway-web + castaway-discord-bot
Last updated: 2026-03-28

## Goal

Add an admin-friendly slash command so Castaway can show the current total for a tribe's Stir the Pot contributions during an open round:

- `/castaway pot show tribe:<tribe>`

This should answer questions like:

- how many points are currently in Lotus's pot?
- what reward tier has Tangerine reached?
- how far is Leaf from the next threshold?

## Current state

Today, Stir the Pot contributions are already recorded per participant **and** tagged with the contributor's active tribe at contribution time.

Relevant implementation points:

- `apps/castaway-web/internal/httpapi/merge_gameplay.go`
  - `addStirThePotContribution(...)` records each contribution with `ParticipantGroupID`
  - `getStirThePotStatus(...)` only returns the caller's personal contribution and balance
- `apps/castaway-web/internal/gameplay/resolver.go`
  - `resolveStirThePot(...)` aggregates contributions by `ParticipantGroupID`
  - tribal pony resolution also computes Stir the Pot bonuses by tribe/group
- `apps/castaway-discord-bot/internal/discord/commands.go`
  - current bot commands are only:
    - `/castaway pot status`
    - `/castaway pot add`
    - `/castaway pot start`
- `apps/castaway-discord-bot/internal/format/merge.go`
  - current formatting only supports personal status and contribution confirmations

So the behavior is already **per-tribe**, but there is **no current user-facing API or slash command** to show the aggregate tribe total.

## Product clarification to lock

Before implementation, decide whether `/castaway pot show tribe:<tribe>` should be:

1. **admin-only**
   - safest with the original "blind contributions" rules
   - lets admins inspect live totals without changing player information visibility

2. **public for all players**
   - still keeps individual contributions hidden
   - but changes the game from fully blind totals to publicly visible tribe totals

Recommendation: make the first version **admin-only**. If product later wants public tribe totals, loosen that intentionally and update the gameplay docs to match.

## Desired command behavior

### Slash command

Add:

- `/castaway pot show tribe:<tribe> [instance]`

Recommended options:

- `tribe` required
  - use the same tribe/group naming already used in the instance, e.g. `Lotus`, `Tangerine`, `Leaf`
- `instance` optional
  - defaults to the user's or guild's default instance as usual

### Response shape

If an open round exists and the tribe is valid, return:

- round name
- tribe name
- current contributed total
- current reached reward bonus
- next reward tier target, if any
- points remaining to next tier, if any
- whether the tribe has maxed the ladder

Example:

```text
**Season 50: Stir the Pot**
- Round: Stir the Pot — Episode 6
- Tribe: Lotus
- Current pot: 5
- Current bonus: +2
- Next tier: 8→+3
- Points to next tier: 3
```

If no round is open:

```text
**Season 50: Stir the Pot**
Stir the Pot is not currently open.
```

If the tribe name is invalid:

- return a clear validation error listing valid tribe names for the current round/instance

## API plan

Add a dedicated read endpoint in `apps/castaway-web/internal/httpapi/merge_gameplay.go`.

Recommended route:

- `GET /instances/:instanceID/stir-the-pot/tribes/:groupID`

Alternative if name-based lookup is preferred at HTTP level:

- `GET /instances/:instanceID/stir-the-pot/tribes/by-name/:tribeName`

Recommended JSON response:

```json
{
  "open": true,
  "round": { "id": "...", "name": "Stir the Pot — Episode 6" },
  "tribe": { "id": "...", "name": "Lotus" },
  "contribution_points": 5,
  "reward_bonus": 2,
  "next_reward_tier": { "contributions": 8, "bonus": 3 },
  "points_to_next_tier": 3,
  "reward_tiers": [
    { "contributions": 2, "bonus": 1 },
    { "contributions": 5, "bonus": 2 },
    { "contributions": 8, "bonus": 3 },
    { "contributions": 11, "bonus": 4 }
  ]
}
```

## Data/query plan

No schema migration should be needed.

Use existing data:

- open round from `activity_occurrences`
- contributor rows from `activity_occurrence_participants`
- tribe identity from `participant_group_id`
- reward tiers from round metadata

Implementation options:

1. **small SQL aggregation query**
   - add a query that sums `(metadata->>'contribution')::int` for one occurrence + one tribe/group
   - preferred for a clean API implementation

2. **reuse existing participant listing query and sum in Go**
   - acceptable for first slice
   - less efficient but probably still fine for current scale

Recommendation: add a dedicated sqlc query for clarity.

Suggested query responsibilities:

- resolve the open Stir the Pot round for the instance
- validate the target tribe/group belongs to the same instance
- sum contribution metadata for contributor rows in that round and tribe

## Bot plan

### Command registration

Update `apps/castaway-discord-bot/internal/discord/commands.go` to add:

- `/castaway pot show`

Options:

- `tribe` required
- `instance` optional

### Handler wiring

Update:

- `apps/castaway-discord-bot/internal/discord/handlers.go`
- `apps/castaway-discord-bot/internal/discord/merge_gameplay.go`

Add a new handler that:

- resolves the instance the same way other Castaway commands do
- validates the caller is an instance admin, if admin-only
- calls the new web API endpoint
- formats the response into a compact Discord message

### Formatting

Update:

- `apps/castaway-discord-bot/internal/format/merge.go`

Add something like:

- `StirThePotTribeStatus(instance, response)`

## Permission plan

Recommended first version:

- admin-only

Reason:

- existing gameplay copy says contributions are blind
- public tribe totals materially change the information players have during the round
- admin-only gives you the operational visibility you asked for without silently changing gameplay

If product wants public totals later:

- remove the admin guard
- update `docs/gameplay/stir-the-pot.md`
- update bot/app requirements docs to clarify that totals are public but individual contributions stay hidden

## Validation plan

### Web tests

Update/add tests in:

- `apps/castaway-web/internal/httpapi/server_integration_test.go`

Cover:

- open round + tribe with contributions
- open round + tribe with zero contributions
- no open round
- invalid tribe
- admin permission enforcement, if admin-only

### Bot tests

Update/add tests in:

- `apps/castaway-discord-bot/internal/discord/handlers_test.go`
- `apps/castaway-discord-bot/internal/castaway/client_test.go`
- `apps/castaway-discord-bot/internal/format/format_test.go` or `internal/format` merge tests

Cover:

- command dispatch for `/castaway pot show`
- formatting of current total / current tier / next tier
- permission failure behavior

### Manual verification

- open a Stir the Pot round in a test instance
- submit contributions from multiple players across multiple tribes
- confirm `/castaway pot show tribe:Lotus` matches aggregated DB state
- confirm the displayed tier matches resolver behavior at tribal pony resolution

## Documentation updates

If implemented, update:

- `apps/castaway-discord-bot/README.md`
- `apps/castaway-discord-bot/functional-requirements.md`
- `apps/castaway-web/README.md`
- `apps/castaway-web/functional-requirements.md`
- `docs/gameplay/stir-the-pot.md` if visibility rules change

## Recommended implementation order

1. add web aggregation/query support
2. add web route + response model
3. add bot client method
4. add `/castaway pot show tribe:<tribe>` command + formatter
5. add tests
6. update docs to match the chosen visibility rules
