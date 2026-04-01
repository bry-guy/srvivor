# Merge Gameplay Activities Plan

Status: `done`
Owner: castaway-web + castaway-discord-bot
Last updated: 2026-03-28

## Goal

Support the merge-related gameplay documented in:

- `docs/gameplay/stir-the-pot.md`
- `docs/gameplay/individual-pony-auction.md`
- `docs/gameplay/loan-shark.md`
- `docs/gameplay/loan-shark-advantage-scroll.md`

This plan covers:

- Discord bot UX for players and admins
- API additions in `apps/castaway-web`
- data model changes needed to support hidden player actions, auction resolution, pony ownership, and loan accounting

Assumption from product/admin:

- each player will be linked to the correct in-game participant via Discord ID, so player actions can be inferred from `X-Discord-User-ID` / `/castaway link`

## Completion summary

This plan has now been implemented as a practical merge-gameplay slice:

- legacy `loan_shark` tribe-contribution naming was normalized to `stir_the_pot`
- `castaway-web` now supports Stir the Pot player actions, auction lot open/close, blind bidding, Loan Shark borrow/repay, pony ownership, and individual pony immunity payouts
- `castaway-discord-bot` now exposes player/admin slash command UX for these flows
- local verification coverage now seeds and executes a contrived merge gameplay scenario end-to-end
- season 50 local seed data was updated so Angelina is recorded at position 19 while Mike remains at position 20

## Current state

The repo already has useful foundations:

- Discord identity linking exists:
  - `apps/castaway-web/db/migrations/009_participant_discord_user_id.sql`
  - `apps/castaway-web/db/migrations/010_instance_admins.sql`
  - `apps/castaway-web/internal/httpapi/server.go`
  - `apps/castaway-discord-bot/internal/discord/handlers.go`
- activity / occurrence / bonus ledger primitives already exist:
  - `instance_activities`
  - `activity_occurrences`
  - `activity_occurrence_participants`
  - `bonus_point_ledger_entries`
  - `participant_advantages`
- the bot already has read-only activity inspection commands:
  - `/castaway activity`
  - `/castaway occurrences`
  - `/castaway occurrence`
  - `/castaway history`

## Important drift to fix first

There is current terminology and behavior drift:

1. The repo still contains a **legacy activity name reuse**:
   - a Stir-the-Pot-like tribe contribution mechanic currently exists under the old `loan_shark` name
   - core references include:
     - `apps/castaway-web/internal/gameplay/resolver.go`
     - `apps/castaway-web/internal/gameplay/resolver_test.go`
     - `apps/castaway-web/seeds/historical-seasons.json`

2. Product clarification is that this is intentional history drift:
   - Loan Shark previously existed
   - that implementation was later overridden
   - the old name was reused for what is now better understood as **Stir the Pot's predecessor / spiritual successor**

3. The repo should now be normalized so that:
   - the existing tribe contribution mechanic is named `stir_the_pot`
   - true `loan_shark` refers only to the merge-era loan mechanic documented in `docs/gameplay/loan-shark.md`
   - old `loan_shark` terminology is removed from current code, seeds, and active docs where it actually means Stir the Pot

4. Even after the rename, the current implementation still does **not** match `docs/gameplay/stir-the-pot.md` fully:
   - it resolves immediately instead of supporting a hidden submission window
   - it does not model blind contributions
   - it does not model “if your tribe loses, I keep the pot”
   - it does not model a later settlement step tied to a winning tribe or winning outcome

5. `docs/gameplay/stir-the-pot.md` itself contains a rules inconsistency that must be resolved before deeper implementation:
   - top summary says effectively “5 bonus points -> +1 point”
   - prompt says:
     - `2 -> +1`
     - `5 -> +2`
     - `8 -> +3`
     - `? -> +4`

This plan assumes we will explicitly reconcile that drift instead of preserving stale naming.

## Product decisions to lock before coding

These are the minimum rules that should be confirmed before implementation starts.

### Stir the Pot

1. Is the prompt ladder canonical, or is the summary canonical?
2. Does Stir the Pot create:
   - an **immediate reward** when the tribe wins the Stir the Pot event, or
   - a **modifier to a later pony payout** as implied by “increase the value of your pony win”?
3. Are contributions editable while the window is open?
4. After resolution, are individual contributions revealed publicly, revealed only to admins, or kept private to the contributor?

### Individual Pony Auction

1. Does every remaining player in the merge become an auction lot?
2. If only one valid bid exists for a pony, does the winner pay `0`, `1`, or some reserve price?
3. Can players overcommit across multiple blind bids, or must the system reserve enough points for all open bids?
4. When a player wins multiple ponies, do all ownerships become active immediately at auction close?

### Loan Shark

1. Is borrowing available only while the Individual Pony Auction is open, or throughout the merge window?
2. Is borrowed value public or private on the leaderboard while the loan is outstanding?
3. Does “lose all of their remaining bonus points” at default mean:
   - visible bonus only, or
   - visible + secret bonus?
4. Does the Loan Shark Advantage apply per participant after the merge, even if it originated from a tribe reward pre-merge?

## Recommended product interpretation

Unless product wants different behavior, the implementation should use these interpretations.

### Stir the Pot

- Treat Stir the Pot as a **hidden contribution window** tied to the next tribal pony result
- Player contributions should **immediately debit** bonus points using hidden/private ledger entries so other players cannot infer the amount publicly
- The tribal pony resolver should automatically enhance the winning tribe's tribal pony payout based on the active Stir the Pot round
- If the tribe loses, they simply lose the contributed points and receive no Stir the Pot boost
- Prefer a configurable reward ladder in activity metadata rather than hardcoding one formula

### Individual Pony Auction

- Treat the auction as a set of **blind second-price lots**
- Admin should explicitly open and close lots per player via Discord bot UX such as `/castaway auction start|stop <player>`
- Opening a lot should bind it to the **next scheduled episode**
- Submitted bids should **immediately debit** the bidder's bonus balance using hidden/private ledger entries while the lot is open
- If a hidden bid spend consumes secret bonus points, those points should be converted into revealed/public-safe ledger rows first
- Updating a bid should apply only the delta:
  - increasing a bid debits more points immediately
  - lowering a bid refunds the difference immediately
- Resolution should charge winners the second-highest valid bid, not their own bid
- Winning a lot should create a time-bound ownership record for that pony

### Loan Shark

- Treat Loan Shark as a **real debt contract**, not as an activity occurrence that immediately resolves to award/spend rows
- Loan issuance should increase the borrower’s spendable balance
- Repayment obligation should be tracked separately from current bonus balance
- Outstanding debt should be visible to the borrower and admins even if public leaderboard treatment stays limited
- Existing `participant_advantages` can carry the “+1 extra point, interest free” modifier; no new advantage table is required

## Discord bot UX plan

### UX principles

- All player write commands should be **ephemeral**
- Public informational commands can stay non-ephemeral
- Player commands should infer the acting participant from the linked Discord account
- Admin management can initially remain API/admin-tool driven if that keeps the first slice smaller

### Stir the Pot commands

Recommended player-facing commands:

- `/castaway pot status`
  - ephemeral
  - shows:
    - whether a Stir the Pot window is open for the player’s current tribe
    - the rules / payout ladder
    - the player’s own committed amount
    - the close time
- `/castaway pot add <points>`
  - ephemeral
  - increments the player’s hidden contribution
  - validates they have enough unreserved bonus points
  - responds with new personal committed total

Optional follow-up command if editing is needed:

- `/castaway pot set <points>`
  - ephemeral
  - replaces current committed total instead of incrementing

Public informational commands:

- `/castaway activity Stir the Pot`
  - public/read-only
  - shows window status and rules
  - does **not** reveal hidden contributions

Admin flow:

- create/open/close/resolve Stir the Pot via API/admin tooling first
- later, optionally add admin bot commands once player flow is stable

### Individual Pony Auction commands

Recommended player-facing commands:

- `/castaway auction status`
  - ephemeral
  - shows:
    - whether the auction is open
    - open lots
    - player’s current bonus balance after any hidden bid debits
    - any active loan balance / remaining loan capacity
- `/castaway bid <player> <points>`
  - ephemeral
  - creates or replaces the player’s hidden bid for that pony
  - validates that any increase can be paid immediately from current bonus balance
  - confirms only the actor’s own bid
- `/castaway bids`
  - ephemeral
  - shows the player’s current bids and current hidden amounts already committed into those bids
- `/castaway ponies`
  - ephemeral by default when showing only your ownerships
  - optionally public when showing resolved auction ownerships after close

Public informational commands:

- `/castaway activity "Individual Pony Auction"`
  - public/read-only
  - shows lots, open/closed state, and resolved outcomes
  - does not reveal live bids

### Loan Shark commands

Recommended player-facing commands:

- `/castaway loan status`
  - ephemeral
  - shows:
    - current principal borrowed
    - interest owed
    - total due
    - repayment deadline
    - remaining borrowing capacity
    - whether Loan Shark Advantage modifies the terms
- `/castaway loan request <points>`
  - ephemeral
  - validates against remaining borrowing capacity
  - if accepted, immediately increases spendable balance
- `/castaway loan repay <points>`
  - ephemeral
  - reduces outstanding debt and current bonus balance accordingly

Admin informational support:

- admin API/report for outstanding loans and defaults due before finale

## API plan

The current API is service-to-service authenticated, with player context passed via `X-Discord-User-ID`. That model is enough for this feature set.

### Authorization model

For new player-action routes:

- require service auth as today
- require `X-Discord-User-ID`
- resolve the acting participant through `participants.discord_user_id`
- reject actions when the caller is not linked in the chosen instance
- keep admin-only routes gated by `instance_admins`

### New read routes

Recommended additions:

- `GET /instances/:instanceID/participants/me/spendable-balance`
  - optional follow-on route if a dedicated balance view is still useful
  - if implemented, it should return:
    - visible bonus
    - secret bonus visible to self
    - hidden bid / contribution debits already applied
    - active loan credit already granted
    - current usable balance

- `GET /activities/:activityID/pot/me`
  - player’s own Stir the Pot state

- `GET /activities/:activityID/auction/me`
  - player’s own auction state
  - current bids
  - current hidden debits already committed into those bids
  - ownerships if already resolved

- `GET /activities/:activityID/loan/me`
  - player’s own loan state
  - outstanding principal, interest, deadline, repayments

### New write routes for player actions

Recommended additions:

- `PUT /activities/:activityID/pot/me`
  - set or increment hidden pot contribution

- `PUT /activities/:activityID/auction/lots/:lotID/bid/me`
  - create or replace hidden bid

- `POST /activities/:activityID/loan/me/requests`
  - request/accept a new loan increment

- `POST /activities/:activityID/loan/me/repayments`
  - repay some or all outstanding debt

These routes should be actor-scoped. The caller should never provide an arbitrary `participant_id` for player commands.

### Admin routes / admin-tool routes

Recommended additions for admin or internal tooling:

- create/open/close/resolve Stir the Pot windows
- create/open/close/resolve auction lots
- resolve auction winners and payment amounts
- list outstanding loans and mark defaults
- grant Loan Shark Advantage to participants

It is fine for the first slice to implement these as API/admin-tool routes before adding bot admin commands.

## Data model plan

### Why the current model is not enough by itself

Existing `activity_occurrences` + `bonus_point_ledger_entries` are good for **resolved historical facts**.

They are not sufficient for this new work because we now need:

- hidden player submissions while an activity is still open
- immediate hidden debits while bids/contributions remain blind
- second-price auction resolution across many hidden bids
- long-lived debt obligations with partial repayments and a final default rule
- ownership of ponies as a time-bound participant-to-participant relationship

### Recommended additions

#### 1. Hidden player submissions

Add dedicated mutable or semi-mutable state for hidden player actions.

Recommended tables:

- `stir_the_pot_contributions`
  - `id`, `public_id`
  - `instance_id`, `activity_id`
  - `participant_id`
  - `participant_group_id`
  - `points`
  - `status` (`open`, `locked`, `resolved`, `cancelled`)
  - `submitted_at`, `updated_at`
  - optional `resolved_occurrence_id`
  - optional `metadata`

- `auction_lots`
  - `id`, `public_id`
  - `instance_id`, `activity_id`
  - `pony_participant_id`
  - `status` (`open`, `closed`, `resolved`, `cancelled`)
  - `opens_at`, `closes_at`, `resolved_at`
  - optional `metadata`

- `auction_bids`
  - `id`, `public_id`
  - `lot_id`
  - `bidder_participant_id`
  - `bid_points`
  - `status` (`active`, `replaced`, `withdrawn`, `resolved`, `invalid`)
  - `submitted_at`, `updated_at`
  - unique active bid per `(lot_id, bidder_participant_id)`

These should be treated as **private operational state**, not as ledger rows.

#### 2. Pony ownership

Add a first-class ownership table rather than encoding this only in JSON assignment config.

Recommended table:

- `participant_pony_ownerships`
  - `id`, `public_id`
  - `instance_id`
  - `owner_participant_id`
  - `pony_participant_id`
  - `source_activity_id`
  - `source_auction_lot_id`
  - `starts_at`
  - `ends_at`
  - `status` (`active`, `ended`, `revoked`)
  - `metadata`
  - `created_at`, `updated_at`

Why a dedicated table is worth it:

- ownership is a core gameplay concept after the merge
- winners can own multiple ponies
- future immunity resolution needs a direct lookup from winning player -> fantasy owners
- it avoids awkward participant-to-participant references hidden inside JSON blobs

#### 3. Loan contracts

Add a first-class loan table.

Recommended table:

- `participant_loans`
  - `id`, `public_id`
  - `instance_id`
  - `participant_id`
  - `activity_id`
  - `status` (`active`, `repaid`, `defaulted`, `cancelled`)
  - `principal_points`
  - `interest_points`
  - `principal_repaid_points`
  - `interest_repaid_points`
  - `granted_at`
  - `due_at`
  - `settled_at`
  - `metadata`
  - `created_at`, `updated_at`

Recommended usage:

- loan issuance writes a bonus ledger award row linked by metadata to the loan record
- repayments write bonus ledger spend rows linked by metadata to the loan record
- the loan table remains the source of truth for obligations and status

#### 4. Balance concept

Current implementation direction should prefer **immediate hidden debits** over a separate reservation ledger.

That means:

- Stir the Pot contributions are written immediately as hidden spends
- auction bid increases are written immediately as hidden spends
- auction bid decreases and losing bids are returned through hidden correction/refund rows
- loan issuance is written immediately as hidden awards
- loan repayment is written immediately as hidden spends
- when a hidden spend consumes secret bonus points, those points should first be converted into visible/revealed ledger rows so the secret points become public on use

A dedicated balance route may still be useful, but it can usually be derived directly from the existing visible + secret bonus ledger totals rather than from a separate reservation table.

#### 5. Advantage usage

Reuse the existing `participant_advantages` table for both concepts, but split the names cleanly.

Recommended change:

- rename the old tribe contribution modifier away from `advantage_type = "loan_shark"`
- use an explicit Stir the Pot type for the historical/current tribe contribution mechanic, such as:
  - `stir_the_pot_advantage`
- use a distinct participant-scoped merge-loan type for the true Loan Shark mechanic, such as:
  - `loan_shark_bonus_credit`
  - or `loan_shark_interest_waiver`

Recommended metadata shape for true Loan Shark:

- `extra_principal_points: 1`
- `interest_discount_points: 1`

This preserves flexibility without new schema.

## Resolver / domain behavior plan

### Stir the Pot resolution

Implement Stir the Pot so that player contributions are recorded during an open round, the round is bound to the next scheduled episode, and then it is automatically consumed by the matching tribal pony resolution for that episode.

During contribution:

1. load the current open Stir the Pot round
2. upsert the player's hidden contribution total for that round
3. if the spend uses secret bonus points, convert those secret points into revealed/public-safe ledger rows first
4. immediately write a hidden spend row for the added points

When tribal pony resolves:

1. load any open Stir the Pot round(s) that apply to the current episode
2. determine which tribe(s) won the relevant tribal pony outcome
3. compute reward using a configurable ladder from activity/occurrence metadata
4. keep loser contributions spent
5. increase the winning tribe's tribal pony payout automatically
6. mark the Stir the Pot round resolved

### Individual Pony Auction resolution

Implement auction resolution as an admin-triggered operation.

At resolve time it should, per lot:

1. collect active valid bids
2. rank them by bid amount, then deterministic tie-break rules
3. pick the winning bidder
4. compute the second-highest valid bid amount as the final price
5. refund all losing bids through hidden correction/refund rows
6. refund any winner overbid amount through a hidden correction/refund row
7. create `participant_pony_ownerships` for the winner
8. record the lot result for audit/history

Tie-break rule should be explicit in metadata or product rules, not implicit in SQL ordering.

### Individual pony immunity payouts

Add a resolver or extend an immunity resolver to support:

- activity type: `individual_pony`
- occurrence type: `immunity_result`
- lookup: winning survivor participant -> active fantasy owners
- payout: `+3` to each owner for each immunity win

This should be separate from auction resolution. The auction only creates ownership; immunity occurrences create the points later.

### Loan Shark behavior

Loan behavior should be transaction-based, not occurrence-only.

When a player borrows:

1. validate linked participant and active auction/loan window
2. compute remaining allowed principal:
   - base max `3`
   - `4` if advantage applies
3. compute interest owed:
   - base `1`
   - `0` if advantage removes interest
4. create/update `participant_loans`
5. write a ledger award row for the borrowed points

When a player repays:

1. validate current spendable balance
2. apply repayment to interest first or principal first, per explicit rule
3. write a ledger spend row
4. update repayment fields on the loan
5. mark repaid when total due reaches zero

Before the finale:

- provide an admin report of all active unpaid loans
- optionally support an admin “settle defaults” operation

On default:

- write a penalty spend/correction row that zeroes the borrower’s remaining bonus balance per the confirmed rule
- mark the loan `defaulted`

## Activity taxonomy recommendation

Introduce or normalize these activity types for new work:

- `stir_the_pot`
- `individual_pony_auction`
- `individual_pony`
- `loan_shark`

Recommended occurrence types:

- Stir the Pot:
  - legacy/current immediate-resolution shape:
    - `stir_the_pot_result`
  - fuller future hidden-window shape:
    - `submission_window_opened`
    - `submission_window_closed`
    - `stir_the_pot_resolution`
- Individual Pony Auction:
  - `auction_opened`
  - `auction_closed`
  - `lot_resolved`
  - `auction_resolved`
- Individual Pony:
  - `immunity_result`
- Loan Shark:
  - `loan_window_opened`
  - `loan_issued`
  - `loan_repayment`
  - `loan_default`

Do **not** keep using `loan_shark` as the activity type for the Stir the Pot-like tribe contribution mechanic.

## Suggested implementation slices

### Slice 0: rule and naming cleanup

1. confirm the open product questions above
2. rename the existing Stir-the-Pot-like implementation from `loan_shark` to `stir_the_pot` in code, tests, seeds, and active docs
3. migrate old seed/test data to the new naming instead of preserving the legacy alias
4. keep true `loan_shark` reserved for the merge-era loan mechanic only

### Slice 1: player-action persistence and spendable balance

1. add tables for hidden contributions and auction bids/lots
2. add service methods for spendable balance and active reservations
3. add player-context API routes for self-scoped reads/writes
4. add tests for linked-player authorization and reserve validation

### Slice 2: Stir the Pot

1. implement hidden contribution writes
2. implement admin close/resolve flow
3. implement resolver-backed ledger settlement
4. expose Discord bot `pot` commands
5. add tests for blind state, winner/loser behavior, and ladder handling

### Slice 3: Auction lots, bids, and pony ownership

1. add auction lot and bid APIs
2. implement second-price resolution
3. add `participant_pony_ownerships`
4. add bot `auction status`, `bid`, and `bids`
5. add tests for reserve math, tie-breaks, and multiple ownerships

### Slice 4: Individual pony payout resolution

1. add `individual_pony` activity support
2. resolve immunity wins into `+3` owner payouts
3. surface ownership in activity/history endpoints and bot output
4. add tests for multiple owners and repeated immunity wins

### Slice 5: Loan Shark contracts and repayments

1. add `participant_loans`
2. implement borrow and repay routes
3. reuse `participant_advantages` for the advantage modifier
4. add bot `loan status`, `loan request`, and `loan repay`
5. add tests for cap, interest, repayment, and default

### Slice 6: history/read-model polish

1. include auction outcomes, pony ownership, and loan status in participant history where appropriate
2. ensure public read routes never leak active hidden bids or contributions
3. ensure self views show private context clearly

## Test plan

Add coverage for:

- player command authorization via linked Discord user
- hidden contribution persistence and non-leakage
- spendable balance with open reservations
- Stir the Pot winner vs loser settlement
- configurable Stir the Pot ladder handling
- auction reserve enforcement across multiple bids
- second-price winner payment
- deterministic tie-break behavior
- pony ownership creation and lookup
- +3 payout for owned pony immunity wins
- loan issuance cap 3 vs 4 with advantage
- interest 1 vs 0 with advantage
- partial repayment and full repayment
- default before finale
- participant history formatting for all new concepts

## Non-goals for first slice

- full admin bot UX for creating and resolving every activity
- public disclosure of live blind bids or live hidden contributions
- generic rules-engine abstraction for every future game mechanic
- retrofitting old historical seasons unless needed for consistency or seed stability

## Recommended first coding target

Start with **Slice 1 + Slice 2**:

- hidden player submissions
- spendable balance
- Stir the Pot end-to-end

Why:

- it forces the new actor-scoped write model into the API and bot
- it creates the reservation primitives the auction will also need
- it builds on a cleaned-up baseline where the existing tribe contribution mechanic is correctly named `stir_the_pot`
