# Season automation plan

Status: `planning`
Owner: castaway-web + castaway-admin
Last updated: 2026-04-16

## Problem

Running a Castaway season currently requires the admin to:
- manually create instances, contestants, participants, episodes, and activities
- manually link Discord users to participants
- manually advance episodes as they air
- manually record eliminations after each episode
- manually trigger immunity/reward resolution for individual ponies
- manually run Stir the Pot / Merge Auction flows
- manually correct scoring errors via DB surgery
- know implementation details of every gameplay mechanic

This means the admin cannot play as a regular player, because operating the game exposes secrets and requires constant hands-on intervention. The game is also fragile — a missed step or a bug means incorrect scores that require vibe coding to fix.

## Goal

Make it possible to:
1. **Define a season declaratively** as a config file
2. **Bootstrap an entire season** from that config in one operation
3. **Auto-advance the game** as episodes air, with minimal admin intervention
4. **Script test seasons** for deterministic end-to-end validation
5. **Eventually let the admin play as a player** while Jeff Probst bot runs the game autonomously

## Design principles

- **Config-driven, not code-driven.** New seasons should not require new Go code unless they introduce a new gameplay mechanic.
- **Idempotent operations.** Re-running a season step that already completed should be safe.
- **Observable.** The admin should always be able to see what the automation did, what it's about to do, and what it's waiting on.
- **Override-friendly.** The admin can intervene at any point using the Admin CLI without breaking automation.
- **Testable.** A season config can run as a test with scripted player actions and deterministic assertions.

## Season config format

A season config is a YAML file that declares everything needed to run a season:

```yaml
name: "Season 50"
season: 50

contestants:
  - Aubry
  - Charlie
  - Chrissy
  - Christian
  # ... all 24

episodes:
  - number: 0
    label: Preseason
    airs_at: "2026-02-26T01:00:00Z"
  - number: 1
    label: Episode 1
    airs_at: "2026-03-05T01:00:00Z"
  # ... through finale

participants:
  - name: Bryan
    discord_user_id: "198239741265707008"
  - name: Kyle
    discord_user_id: "235246238382030849"
  # ... all players

tribes:
  - name: Tangerine
    kind: tribe
    members: [Adam, Grant, Kate, Keith, Kyle]
  - name: Leaf
    kind: tribe
    members: [Bryan, Lauren, Mooney, Riley, Yacob]
  - name: Lotus
    kind: tribe
    members: [Amanda, Katie, Keeling, Kenny, Marv, Sarah]

activities:
  - type: tribal_pony
    name: "Tribal Pony"
    starts_at: episode 1
    ends_at: episode 5
    rules:
      points_per_win: 1

  - type: stir_the_pot
    name: "Stir the Pot"
    starts_at: episode 3
    ends_at: episode 6
    rules:
      tiers: [{contributions: 3, bonus: 1}, {contributions: 5, bonus: 2}, {contributions: 10, bonus: 4}]
      hidden_final_tier: true

  - type: tribe_wordle
    name: "Tribe Wordle"
    starts_at: episode 2
    ends_at: episode 5

  - type: merge_auction
    name: "Merge Auction"
    starts_at: episode 7
    mode: three_round_blind_fallthrough

  - type: individual_pony
    name: "Individual Pony"
    starts_at: episode 7
    rules:
      immunity_points: 3
      reward_points: 1

  - type: loan_shark
    name: "Loan Shark"
    starts_at: episode 7

# Optional: pre-planned occurrences per episode
# These represent known events that happen at specific episode boundaries.
episode_events:
  - episode: 1
    events:
      - type: tribal_pony
        action: record_immunity
        # winner determined at runtime by admin input or external data

  - episode: 7
    events:
      - type: merge_auction
        action: import_results
        # source: external form CSV
```

## Lifecycle model

### Phase 1: Season bootstrap

`castaway-admin import-season season50.yaml`

This creates:
- the instance
- all contestants and instance_contestants joins
- all participants with Discord links
- all episodes
- all tribes with membership periods
- all activities

After bootstrap, the game is ready for drafts and play.

### Phase 2: Episode-driven progression

Each episode boundary triggers a set of game operations. Some can be automated, some require admin input.

**Automatable (no human input needed):**
- advance current episode
- check if any activities start or end at this episode boundary

**Requires admin input:**
- who was eliminated (could be scraped from a data source later)
- who won immunity / reward (admin input or data source)
- Stir the Pot open/close timing
- Merge Auction form results

**Hybrid (admin triggers, server resolves):**
- tribal pony immunity resolution (admin says "Tangerine won", server awards points)
- individual pony resolution (admin says "Joe won immunity + reward", server awards to pony owner)

### Phase 3: Admin-as-player separation

The long-term goal is that the admin can play as a regular player. This requires:
- all admin operations can be performed by a non-player operator role, OR
- admin operations are scheduled/automated so no human sees secrets

For example:
- elimination results could be auto-scraped from a Survivor spoiler-free data API
- immunity/reward winners could be entered by a non-playing friend
- or the system could prompt the admin to enter results in a way that doesn't reveal other players' secret bonus points

This phase is aspirational and depends on how much external data integration is feasible.

## Scripted test seasons

A key benefit of declarative configs is testability. A test season config can include:

```yaml
test_script:
  - episode: 1
    actions:
      - player: Alice
        action: draft
        picks: [Joe, Ozzy, Cirie, ...]
      - admin:
        action: eliminate
        contestant: Jenna
      - admin:
        action: tribal_immunity
        winner_tribe: Tangerine

    assertions:
      - participant: Alice
        draft_points: 3
        bonus_points: 1
        total: 4
      - leaderboard_order: [Alice, Bob, Cara]

  - episode: 2
    actions:
      - player: Alice
        action: pot_contribute
        points: 2
      - admin:
        action: eliminate
        contestant: Kyle
    assertions:
      - participant: Alice
        bonus_points: -1  # contributed 2, earned 1 tribal
```

This enables:
- **Regression testing**: run a known season through the engine and assert scores match
- **Deterministic validation**: prove the scoring model is correct for any sequence of events
- **New mechanic testing**: add a new activity type and validate it doesn't break existing scoring
- **Season replay**: replay Season 50 from scratch and verify final scores match production

## Implementation roadmap

### Short-term (builds on Admin CLI)
1. **Season config YAML schema** — define the format
2. **`import-season` CLI command** — bootstrap from config
3. **`episode advance` + `eliminate` CLI commands** — manual but efficient weekly operation
4. **`pony immunity` + `pony reward` CLI commands** — replace manual curl/SQL

After this, weekly game operation looks like:
```bash
castaway-admin episode advance
castaway-admin eliminate Chrissy Coach
castaway-admin pony immunity Joe
castaway-admin pony reward Joe Tiffany
```

That's 4 commands per episode instead of SSH + psql + curl + port-forward.

### Medium-term (automation layer)
5. **Episode event scheduler** — define events per episode in config, server processes them at episode boundaries
6. **`castaway-admin episode run`** — execute all configured events for the current episode, prompting for required inputs
7. **Individual pony reward API endpoint** — so reward recording works through normal app mechanics
8. **Admin manual bonus endpoint** — so corrections don't require DB surgery

### Long-term (autonomous game engine)
9. **External data integration** — scrape elimination/immunity/reward results from a Survivor data source
10. **Jeff Probst bot auto-announcements** — bot announces results without admin triggering
11. **Scripted test runner** — execute test season configs and assert outcomes
12. **Admin-as-player mode** — separate admin identity from player identity so the admin can play blind

## Relationship to Admin CLI

The Admin CLI (separate plan) provides the **operator primitives**. This plan builds on top of those primitives by:
- composing them into higher-level workflows
- scheduling them based on episode air dates
- scripting them for testing

The CLI is the foundation; automation is the composition layer.

## Key new concepts to build

### 1. Season config schema
A YAML format that can express the full shape of a season: players, contestants, episodes, tribes, activities, and their rules.

### 2. Episode lifecycle hooks
A way to declare "when episode N becomes current, do X" — where X can be automated or require admin input.

### 3. Scripted player actions
For testing: a way to express "player Alice contributes 3 to the pot in episode 4" as test data that the engine processes.

### 4. Assertion framework
For testing: a way to express "after episode 4, Alice's score should be 12" and have the test runner verify it.

## What this makes possible

### Today (manual, fragile)
```
Admin watches episode
Admin SSHes into prod
Admin writes SQL to record elimination
Admin curls API for pony immunity
Admin writes more SQL for reward
Admin checks Discord to see if scores updated
Admin vibe-codes with AI to fix any bugs
```

### After Admin CLI (manual, reliable)
```
Admin watches episode
castaway-admin eliminate Chrissy Coach
castaway-admin pony immunity Joe
castaway-admin pony reward Joe Tiffany
Scores update. Done.
```

### After season automation (semi-automated)
```
Admin watches episode
castaway-admin episode run
> Episode 8: who was eliminated? Chrissy, Coach
> Episode 8: who won immunity? Joe
> Episode 8: who won reward? Joe, Tiffany
> Recording... done. Jeff Probst announcing results in #castaway.
```

### Long-term (fully automated)
```
Episode airs at 8pm
System detects new episode aired
System scrapes results from data source
System records eliminations, immunity, reward
Jeff Probst announces results
Admin plays as a player and finds out with everyone else
```
