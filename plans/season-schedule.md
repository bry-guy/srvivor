# Declarative season schedule

Status: in-progress

## Goal

One file per season (`seasons/51.yaml`) shows the whole season at a glance and runs it: episodes, draft
open/close, weekly game open/close/award, scores posts, and one-off messages. You edit the file, run one
command, and the season runs itself. Only two things stay with you each week: the episode results (until we
fetch them) and approving each scores post.

## What history says (Seasons 48–51, #survivor)

- **48–49 (the rhythm to keep):** one scores post per week, usually Wednesday before or around the episode.
  Spoiler-hidden boots, then a joke. Often late ("apologies for the delay", Week 3 posted three times, finale
  12 days late). One message per week.
- **50 (the "crazy" one):** three tribes plus Monty Hall doors, Stir the Pot, a pot auction, secret bonus
  points, a merge auction run on Tally with three bid rounds, Loan Shark Scroll and FIRE finale bingo. Host
  posts went from about 8 a month to 33 a month. There were corrections ("I had the scores incorrect"),
  skipped weeks (job, work trip), and deadlines moved by hand.
- **51 so far:** you promised "a lot simpler than last season". Draft buffs, two tribes, Tribal Pony, and
  one game per week (Castawordle first).

So the file should keep 51 to **one game per week, one scores post per week, on fixed times**. Anything
not yet decided stays a visible placeholder instead of an ad-hoc post.

## Design

- **File:** YAML in the repo, Eastern times, with weekly defaults plus per-week overrides (see
  `seasons/51.yaml`).
- **Placeholders:** `TBD` / `tbd: true` do nothing. The timeline shows them as ⚠, and you're nagged 3 days
  before they're due.
- **`probst season plan FILE`:** prints the full timeline (past ✓, upcoming, ⚠ unset or unapproved, ✗
  missing results) and the diff against the database. Read-only.
- **`probst season apply FILE`:**
  - creates or updates episodes, draft windows, game definitions and scheduled posts. Changing the file and
    re-applying moves things; nothing is duplicated.
  - only reports things that exist in the database but not in the file; it never deletes them.
  - is idempotent and keyed by `week/kind`.
- **Runner:** an in-cluster ticker in the existing bot or web process (no new service). Every minute it:
  - opens games
  - closes and resolves games at their cutoff
  - sends approved posts
  - DMs you about held posts or missing results
- **Scores posts:**
  - At send time the runner fills `{scoreboard}`, `{boots}`, `{movers}` and `{next_game}` into your
    approved prose.
  - Your prose is written ahead of time, from Tuesday's results.
  - If the prose isn't approved, or results are missing, it holds and pings you instead of guessing.
- **Approval:** `probst season approve 51 week 3`, after `probst season preview 51 week 3` shows the exact
  rendered text. This keeps the "exact copy, then explicit approval" rule.

## Decisions (Oct 1)

- **Weekly scores post:**
  - Mentions only the **top 3 and last place**; the full board lives on the website. No `@castaway` role ping.
  - Every week it **praises the leader**, **praises the biggest gainer**, and **gently ribs the biggest
    slider**.
  - The format stays the same, but the wording varies week to week, drawn from a phrase pool with no
    repeats within the season.
  - Pronouns come from the player record (see the `castaway-pronouns` skill).
- **Language:** players belong to a **Tribe** and score when **their Tribe** wins immunity (+2) or reward
  (+1). Don't say "pony" anywhere player-facing.
- **Merge:** Tribe scoring ends.
  - **Pick Your Champion** opens: each player picks one castaway, who earns them +3 for an immunity win and
    +1 for a reward win.
  - Any castaway can be picked, and picks can be shared; the post doesn't need to mention that.
  - The merge episode is a placeholder until it's known.

## Steps

0. Champion picks: a pick flow (web and/or Discord) plus scoring on top of the existing ownership ledger. Rename
   player-facing "pony" copy to Tribe/Champion. Needed before the merge.
1. ✅ Schema and parser with a `plan` timeline (read-only), validated against the live Season 51 state
   (`probst season plan`).
2. `apply` for episodes, draft and games (Castawordle first). Unknown game types must stay placeholders.
3. Scheduled posts with approval and template fill, using the existing announcement store plus bot sending.
4. Runner: auto-resolve at close, then send, then nags.
5. Later: fetch episode results from survivoR into `weekly.results`.

## Open questions

- **Scores time:** the file says Wednesday noon, with the game closing at 11:59am. That's what you asked
  for on Sep 29. Do you want a small buffer, e.g. close at 11:00?
- **Merge:** what happens to Tribal Pony at the merge? It's left as a placeholder.
- **Finale:** final scores on the following Wednesday noon, or the night of the finale?
