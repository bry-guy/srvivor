# Activity Details and Participant History Plan

Status: `done`
Owner: `castaway-discord-bot`
Last updated: 2026-03-27

## Goal

Expand the Discord bot from simple activity/occurrence listing into a read-oriented gameplay explainer that helps a participant answer:

- what happened in this activity?
- what points were awarded, to whom, and why?
- what actions or recorded results led to those awards?
- for me, what happened in the activities I participated in?

## Why now

The current `/castaway activities` and `/castaway occurrences` slice proves that the bot can resolve instances, list activities, and show occurrence summaries. The next useful step is to expose the details already represented in the Castaway data model:

- occurrence participants and groups
- activity participant/group assignments
- bonus ledger entries created by those occurrences
- participant-centric history across activities

Right now the bot output is too shallow for retrospective gameplay review. Users can see that an occurrence exists, but not the meaningful explanation of who did what and how points moved.

## User-facing outcomes

After this plan ships, a Discord user should be able to:

1. list activities for an instance
2. inspect an activity's occurrences with richer detail
3. inspect a single occurrence and see:
   - occurrence type/status/time
   - recorded participants and groups
   - notable recorded actions/results
   - bonus points awarded/spent/revealed, grouped by participant
4. ask for a participant-centric gameplay history and see:
   - activities the participant was involved in
   - occurrences they directly appeared in
   - occurrences that affected their tribe/group
   - points that were awarded, spent, converted, or revealed for them

## Non-goals

- no Discord-driven write/edit flows
- no free-form natural language summarization pipeline outside the existing app logic
- no spoiler-bypass behavior beyond whatever the underlying API already exposes
- no replacement of the leaderboard or draft workflows
- no major data-model redesign in `castaway-web`

## Proposed command surface

Keep the existing commands and extend them conservatively.

### Existing commands to enrich

- `/castaway activities [instance] [season]`
  - keep as a compact list view
  - optionally add a `verbose` flag later if message size permits

- `/castaway occurrences activity:<name> [instance] [season]`
  - upgrade output from terse occurrence list to compact summaries that include:
    - type
    - status
    - effective time
    - participants/groups involved count
    - total public points moved
    - short "awards" line when available

### New commands

- `/castaway occurrence activity:<name> occurrence:<name> [instance] [season]`
  - show one occurrence in detail
  - include:
    - occurrence metadata summary
    - participant results/actions
    - group results/actions
    - ledger impact summary

- `/castaway activity activity:<name> [instance] [season]`
  - show one activity with:
    - activity type/status/window
    - relevant assignments (delegates, tribe mappings, etc.)
    - recent occurrences
    - optional aggregate points summary for that activity

- `/castaway history participant:<name> [instance] [season]`
  - participant-centric history across activities
  - answer the practical question: "what happened in the activities I participated in?"

## Data/API needs

The current API supports activity and occurrence list endpoints, but richer bot output needs more detail than the current list payloads provide.

### Likely API additions

#### Activity details
- `GET /activities/{activityID}`
  - activity metadata
  - participant assignments
  - group assignments

#### Occurrence details
- `GET /occurrences/{occurrenceID}`
  - occurrence metadata
  - participant rows
  - group rows
  - related bonus ledger entries

#### Participant-centric gameplay history
- one of:
  - `GET /instances/{instanceID}/participants/{participantID}/activity-history`
  - or a smaller combination of endpoints the bot can join client-side

Preferred shape for first implementation:
- add explicit read endpoints in `castaway-web`
- avoid forcing the bot to issue a large fan-out of list calls and infer too much client-side

## Response design principles

### 1. Explain impact, not just existence
For each occurrence, the bot should emphasize:
- what was recorded
- what points changed
- who was affected

### 2. Separate recorded actions from score consequences
Example sections:
- **Recorded**
- **Awards**
- **Secret/Public impact**

This keeps "Adam chose STEAL" separate from "Lotus received +1 public".

### 3. Stay within Discord message limits
Use a compact, layered design:
- list commands show summaries
- detail commands show one entity deeply
- truncate with clear "and N more" messaging where necessary

### 4. Preserve secrecy rules
If the API distinguishes `public`, `secret`, and `revealed` bonus visibility, Discord formatting must respect that. The bot should not invent disclosure that the API does not already authorize.

## Formatting examples

### Occurrence summary

```text
**Journey 1 Tribal Diplomacy**
- type: journey_resolution
- status: resolved
- effective: 2026-03-14 01:00 UTC
- recorded: Adam=STEAL, Mooney=STEAL, Katie=SHARE
- awards: Lotus +1 public each
```

### Occurrence detail

```text
**Episode 2 Immunity**
Activity: Tribal Pony
Type: immunity_result
Status: resolved
Effective: 2026-03-12 00:00 UTC

**Recorded**
- winning tribes: Leaf, Lotus

**Awards**
- Amanda +1 public
- Bryan +1 public
- Lauren +1 public
- Mooney +1 public
- Riley +1 public
- Yacob +1 public
- Katie +1 public
- Keeling +1 public
- Kenny +1 public
- Marv +1 public
- Sarah +1 public
```

### Participant history

```text
**Mooney — Activity History**
Season 50 — Season 50

**Journey 1**
- Lost for Words — Mooney
  - recorded: risk attempt, guess_count=3
  - impact: secret bonus change

**Tribal Pony**
- Episode 1 Immunity
  - impact: +1 public
- Episode 2 Immunity
  - impact: +1 public
```

## Implementation plan

1. **Define detailed bot UX and payload shapes**
   - confirm exact command names
   - confirm message section layout
   - decide whether detail commands use names or autocomplete-only IDs under the hood

2. **Add castaway-web read endpoints**
   - activity detail endpoint
   - occurrence detail endpoint
   - participant activity-history endpoint or equivalent
   - preserve read-only behavior

3. **Add castaway-discord-bot client methods**
   - fetch activity detail
   - fetch occurrence detail
   - fetch participant history

4. **Add formatter helpers**
   - occurrence summary formatter
   - occurrence detail formatter
   - activity detail formatter
   - participant history formatter
   - include message-length-aware truncation helpers

5. **Add Discord commands/handlers**
   - enrich `/castaway occurrences`
   - add `/castaway occurrence`
   - add `/castaway activity`
   - add `/castaway history participant:<name>`
   - add autocomplete where needed

6. **Testing**
   - bot client tests for new endpoints
   - formatter tests for representative outputs
   - handler tests for command routing, resolution, and failure cases
   - web integration tests for the new read endpoints where feasible

7. **Docs**
   - update `apps/castaway-discord-bot/README.md`
   - update functional requirements if the feature becomes committed scope
   - add representative examples for Season 50 without violating spoiler expectations

## Risks

- Discord message length limits may make rich occurrence detail noisy or truncated
- participant-centric history can become expensive if the bot must assemble it from many list endpoints
- secret bonus handling may become confusing if formatting is too terse
- activity/occurrence names may not be unique enough for name-only lookup in all future seasons

## Mitigations

- prefer dedicated detail endpoints over client-side fan-out joins
- provide compact summaries in list views and reserve depth for detail commands
- use autocomplete aggressively for activity and occurrence selection
- structure formatter output to distinguish public vs secret vs revealed impact

## Exit criteria

- a user can inspect one occurrence and understand both the recorded action and the point impact
- a user can ask for participant-centric activity history and get a useful answer
- `/castaway occurrences` is materially more informative than today's list-only output
- tests cover representative journey, tribal pony, wordle, and manual-adjustment cases
- docs reflect the expanded read-only command surface
