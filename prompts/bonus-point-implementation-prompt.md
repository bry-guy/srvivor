# Bonus Point Implementation Prompt Pack

This prompt pack is for implementing the Castaway bonus-points system using `pi-ez-worktree` and multiple pi sessions.

## Instruction summary

Use the implementation plan as the execution source of truth:

- `apps/castaway-web/plans/bonus-points-planning.md`

Use these shared references for product semantics and gameplay context:

- `docs/castaway-bonus-points-plan.md`
- `docs/castaway-manual-gameplay-logs.md`
- `docs/gameplay/journey-tribal-diplomancy.md`

## Recommended execution order

1. **Agent 1** implements the persistence foundation in its own ez-worktree and merges first.
2. **Agent 2** starts from the updated `main` and implements the `castaway-web` resolver and read API layers.
3. **Agent 3** starts from the updated `main` and implements the Discord bot read-model update.

Recommended merge order:

1. Agent 1
2. Agent 2
3. Agent 3

Agent 3 should wait until Agent 2 has merged, or at least until the leaderboard/API response shape is stable on `main`.

---

## Agent 1 prompt — persistence foundation

```text
Create a fresh ez-worktree and implement the Castaway bonus-points persistence foundation only.

Read first:
- docs/castaway-bonus-points-plan.md
- apps/castaway-web/plans/bonus-points-planning.md
- docs/castaway-manual-gameplay-logs.md
- docs/gameplay/journey-tribal-diplomancy.md
- apps/castaway-web/functional-requirements.md

Implement only these slices from apps/castaway-web/plans/bonus-points-planning.md:
- Slice 1: episode schedule foundation
- Slice 2: group and activity state
- Slice 3: occurrences and bonus ledger persistence

Requirements:
- follow the schema draft in the plan
- use `entry_kind` and `visibility` in the bonus ledger
- support negative ledger rows for spending and corrections
- keep secret bonus points internal
- keep activity config changes at explicit episode boundaries
- do not implement public leaderboard/bot formatting changes yet unless required for tests
- keep rule logic out of SQL

Execution rules:
- follow the “Agent execution slices” section in the plan
- keep changes limited to the persistence foundation slice
- regenerate derived artifacts in the same slice that changes their sources
- commit complete thoughts during the work
- do not modify docs/castaway-manual-gameplay-logs.md unless absolutely necessary

Validation:
- run all validation commands listed for slices 1-3 in the plan
- before finishing, run:
  - mise run //apps/castaway-web:ci

When finished:
- summarize schema, migration, query, and service changes
- note any assumptions made
- finish the worktree
```

---

## Agent 2 prompt — web domain, resolvers, and read API

```text
Create a fresh ez-worktree from the latest main and implement the Castaway bonus-points web domain/application work, but do not update the Discord bot.

Read first:
- docs/castaway-bonus-points-plan.md
- apps/castaway-web/plans/bonus-points-planning.md
- docs/castaway-manual-gameplay-logs.md
- docs/gameplay/journey-tribal-diplomancy.md
- apps/castaway-web/functional-requirements.md

Assume Agent 1 has already merged slices 1-3. Implement:
- Slice 4: activity resolvers in Go
- Slice 5: public read API and contract updates
- Slice 6 only for the castaway-web bonus-ledger read API side, not the bot

Requirements:
- keep resolution logic in Go
- implement initial resolvers for:
  - `tribal_pony`
  - `tribe_wordle`
  - `journey`
  - `manual_adjustment`
- public leaderboard returns visible bonus only
- keep secret points internal
- preserve `score` as a compatibility alias for `total_points`
- update TypeSpec/OpenAPI as needed
- do not implement Discord bot changes in this worktree

Execution rules:
- follow the “Agent execution slices” section in the plan
- keep changes limited to web resolvers + public read API
- regenerate derived artifacts in the same slice that changes their sources
- commit complete thoughts during the work

Validation:
- run the validation commands listed for the relevant slices in the plan
- before finishing, run:
  - mise run //apps/castaway-web:ci

When finished:
- summarize resolver, leaderboard, and bonus-ledger API changes
- call out any remaining follow-up work for the bot
- finish the worktree
```

---

## Agent 3 prompt — Discord bot read-model update

```text
Create a fresh ez-worktree from the latest main and implement the Discord bot read-model changes for Castaway bonus points.

Read first:
- docs/castaway-bonus-points-plan.md
- apps/castaway-web/plans/bonus-points-planning.md
- apps/castaway-discord-bot/README.md
- apps/castaway-discord-bot/internal/castaway/client.go
- apps/castaway-discord-bot/internal/format/format.go
- apps/castaway-discord-bot/internal/format/format_test.go

Assume Agent 2 has already merged the public leaderboard/API response changes. Implement only the bot-facing work described in the Castaway bonus-points plan:
- parse `draft_points`, `bonus_points`, `total_points`
- update existing `score` and `scores` output
- do not add a dedicated bonus command
- assume the public API exposes visible bonus only

Requirements:
- keep bot behavior read-only
- preserve the current command surface
- update tests for total + draft + visible bonus rendering

Execution rules:
- keep changes limited to the Discord bot app
- do not change castaway-web contracts in this worktree unless absolutely required and called out explicitly
- commit complete thoughts during the work

Validation:
- run:
  - mise run //apps/castaway-discord-bot:ci

When finished:
- summarize client, formatting, and test changes
- finish the worktree
```
