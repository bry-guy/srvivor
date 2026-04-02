# Season 50 Tangerine immunity + Stir the Pot repair plan

Status: `done`
Owner: castaway-web + operations
Last updated: 2026-04-01

## Goal

Make the Season 50 Tangerine tribal pony immunity payout line up with the already-closed Stir the Pot round, while preserving correct contribution debits and avoiding duplicate scoring.

## Investigated current state

Live production findings from the Castaway PostgreSQL host:

1. Keith's manual contribution fix is present.
   - Keith contribution in the closed Stir the Pot round: `2`
   - Matching manual spend ledger row exists: `-2`
   - Keith current bonus balance after that debit: `3`

2. All recorded Stir the Pot contributions already have matching spend debits.
   - Tangerine contributors:
     - Adam `2`
     - Grant `3`
     - Kate `2`
     - Keith `2`
     - Kyle `2`
   - Tangerine total contribution: `11`
   - Leaf total contribution: `8`

3. The closed Stir the Pot round is currently targeted at **Episode 5**, not Episode 6.
   - Round public id: `1ff6817d-d03f-4463-a193-23cdd1b498db`
   - Round name: `Stir the Pot — Episode 5`
   - Metadata target episode: Episode 5
   - Metadata still contains the old final tier value `11 -> +4`

4. There is currently **no Episode 6 tribal pony occurrence** in production.
   - Existing tribal pony occurrences stop at Episode 5.

5. Generic occurrence resolution has a lifecycle bug.
   - `ResolveActivityOccurrence(...)` creates ledger entries but does **not** mark the resolved occurrence itself as `resolved`.
   - Example in prod: Episode 4 immunity still shows `recorded` even though tribal pony awards were already written.
   - This is misleading operationally and makes auditing harder.

## Important conclusions

1. **Contribution debits do not need to be applied again.**
   They are already present.

2. To give Tangerine the intended tribal pony payout for Episode 6, production needs a repair step.
   Specifically:
   - retarget the closed Stir the Pot round from Episode 5 to Episode 6
   - create or record the Episode 6 tribal pony immunity result for Tangerine
   - resolve that occurrence once

3. The expected Tangerine tribal pony payout is **5 total points per tribe member**.
   - base tribal pony immunity: `+1`
   - Stir the Pot add-on: `+4`
   - total awarded to each Tangerine member: `+5`

## Prepared code changes

### 1. Fix occurrence resolution lifecycle

Update `apps/castaway-web/internal/gameplay/resolver.go` so any resolved occurrence is also marked:
- `status = resolved`
- `ends_at = now`
- metadata preserved

This makes tribal pony, wordle, journey, manual adjustment, stir the pot, and individual pony resolutions all leave the occurrence in an operationally accurate state.

### 2. Add/adjust tests

Update resolver tests so they verify:
- resolved tribal pony occurrences are marked `resolved`
- Stir the Pot bonus resolution still resolves the underlying Stir the Pot round
- the triggering tribal pony occurrence is also marked `resolved`

## Proposed production repair sequence after code ships

1. Verify no additional Stir the Pot debits are needed.
2. Patch the closed Season 50 Stir the Pot round metadata to target Episode 6 instead of Episode 5.
3. Optionally normalize the round name to `Stir the Pot — Episode 6` for operator clarity.
4. Record an Episode 6 tribal pony immunity occurrence with Tangerine as the winner.
5. Resolve that occurrence once.
6. Verify each Tangerine member receives exactly one `+5` tribal pony award for that Episode 6 result.

## Safety checks before running the production repair

- confirm Tangerine does not already have an Episode 6 tribal pony payout
- confirm the closed Stir the Pot round still has the expected contribution totals
- confirm no second manual debit was written for Keith
- confirm the occurrence resolve operation will be executed exactly once

## Production repair completed

Completed on the live Castaway PostgreSQL host:

- verified all Stir the Pot contribution debits were already present
- retargeted the closed Season 50 Stir the Pot round to Episode 6
- normalized the round reward tier metadata to end at `10 -> +4`
- marked the closed Stir the Pot round resolved by tribal pony consumption
- created and resolved `Episode 6 Immunity` for Tangerine
- awarded exactly one `+5` public tribal pony bonus to each Tangerine member

Verified after repair:

- closed Stir the Pot round public id `1ff6817d-d03f-4463-a193-23cdd1b498db`
  - now named `Stir the Pot — Episode 6`
  - now `status = resolved`
- tribal pony occurrence public id `850047c5-4316-4bb3-81e2-a418ea790c0f`
  - name `Episode 6 Immunity`
  - `status = resolved`
- Tangerine tribal pony + Stir the Pot winners:
  - Adam `+5`
  - Grant `+5`
  - Kate `+5`
  - Keith `+5`
  - Kyle `+5`

## Acceptance criteria

- Tangerine members are **not** double-debited for their pot contributions
- each Tangerine member receives exactly one `+5` tribal pony payout for the intended immunity result
- the triggering immunity occurrence ends up marked `resolved`
- the closed Stir the Pot round is consumed by the matching tribal pony resolution
