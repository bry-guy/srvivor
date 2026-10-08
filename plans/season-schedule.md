# Season schedule automation

Status: `in-progress`

Supersedes the scheduler parts of [season-automation-planning.md](season-automation-planning.md) and
Checkpoint 2 of [weekly-season-execution.md](weekly-season-execution.md). The file it runs from is
[`seasons/51.yaml`](../seasons/51.yaml).

## Goal

Each week, the admin does at most two things: makes sure next week's game is set up (Probst says when), and
replies **yes**, or replaces the copy, in the scores DM. Everything else runs itself: games open and score,
episode results come in, standings are computed, the post is drafted and DMed for approval, then sent.

The merge switch and Pick Your Champion are out of scope (next plan). Until then, anything the automation
can't score safely holds and alerts instead of guessing.

## The weekly cycle (America/New_York, episode N airs Wednesday 8:00pm)

| When | What happens | Who |
| --- | --- | --- |
| Wed 8:00pm (ep N) | Game N opens (it was created ahead of time) | server |
| Wed 8:00pm (ep N) | **Readiness check for game N+1**: if it isn't configured and created, DM admins | reconciler |
| Thu–Tue, daily | Remind until game N+1 is ready; escalate the day before it opens | reconciler |
| Thu–Tue, every few hours | Import episode N's results from survivoR once complete; DM if still missing Tue night | reconciler |
| Next Wed 7:59pm | Game N closes and scores itself | server (exists) |
| Next Wed ~8:00pm | Once game N is scored and episode N results are in, draft the scores post and DM it for approval | reconciler |
| On "yes" | Posts at 8:00pm, or right away if approved later (within 3 hours) | bot (exists) |
| On a full-text reply | Replaces the copy and sends a fresh DM for approval | bot (exists) |

Missing episode results block only the scores post, not the next game.

## What exists today (Oct 7)

- **Games score themselves at close:** Press the Button, Spell It Out and scored Castawordle, via the
  server's per-minute resolver. Re-running a resolve does nothing new.
- **Approval by DM:** the post is held, every instance admin is DMed, "yes" approves that exact revision, a
  full-text reply replaces the copy and re-asks, and approval up to 3 hours late posts immediately.
- **`probst season plan` / `apply` / `post`:**
  - `apply` creates Press the Button and Spell It Out games. A repeat apply hasn't been proven idempotent.
    Castawordle is skipped.
  - `post --week N --yes` drafts and saves the scores post for approval.
- **`probst episode sync`:** fetches survivoR, records boots and tribe challenge wins. It's run by hand and
  is **not safe unattended**:
  - nonempty tables are treated as complete
  - ambiguous boots are skipped
  - writes happen one at a time, with no record of which data version was applied
- **Stored tribe colors:** `participant_groups.color`, also returned on the leaderboard.

## What's missing (this plan)

### 1. Reconciler: Probst on a schedule in the cluster

- Add a **`probst season reconcile FILE`** command, run by a Kubernetes CronJob **every minute**, so the
  scores draft lands at about 8:00–8:01pm. Each
  run looks at the file and live state and does whatever is due.
  - A single 8:01pm job isn't enough: a missed run, a restart or late results would mean nothing happens.
    Frequent runs catch up on their own.
  - Each run is safe to repeat. It acts only on missing work and handles each week independently, so one
    failure doesn't block the rest.
- **State lives on the server, not in files:** a small `automation_actions` table keyed by
  `(instance, action key)`, e.g. `w3:game-ready-alert`, `w2:scores-draft`, `ep3:results`. Each row records
  a status, attempts, last error, a lease and a result reference. The reconciler claims an action with a
  lease, so two overlapping runs can't double-act.
- **Image and identity:**
  - Publish a Probst image and add a CronJob manifest in `deploy/`.
  - Give it its own automation login: a scoped, revocable service credential for one instance-admin
    identity, created through the normal approved credential path. Never copy a personal session or pull
    tokens out of other Kubernetes secrets.
  - The season file is baked into the image, so a change to `seasons/51.yaml` takes effect on the next
    deploy. Secrets are never in it.
- **Delivery and recovery:**
  - Reminders and DMs are deduplicated by action key and retried.
  - A lease that outlives its worker expires; a stale worker's result is rejected.
  - An ambiguous Discord send is checked before any retry.
  - Each action runs on its own, so one bad game or post doesn't starve the rest.
  - **Operator alerts** DM admins when:
    - the reconciler hasn't succeeded in 30 minutes
    - a game resolve fails
    - an approval expires
    - the automation credential is rejected
- **Kill switch:** `suspend: true` on the CronJob, plus `automation: {enabled: false}` in the file.

### 2. Game readiness and setup

- **Readiness is checked when the previous game opens.** Game N+1 must exist by the time game N opens (for
  example, Episode 4's game is due Oct 7 at 8pm, when Episode 3's opens). Each week's game is in one of
  these states:

  | State | Meaning | Action |
  | --- | --- | --- |
  | `missing` | the file says `TBD` | DM: "Week N+1 has no game. Pick one in seasons/51.yaml." |
  | `incomplete` | type set but no answer or phrase file | DM naming the missing file |
  | `invalid` | fails validation (word not in dictionary, phrase too long) | DM with the error |
  | `failed` | creation errored | DM with the error, retried next run |
  | `ready` | created, with matching opens/closes | nothing |
  | `none` | explicitly no game that week | nothing |

  - Reminders are deduplicated: once when due, then daily, then a final "opens in 24 hours" escalation.
  - No game type is chosen for the admin.
- **`apply` becomes part of the reconciler** and must be idempotent: re-applying the same file changes
  nothing, and a changed time updates the game in place. Fix the current repeat-apply behavior.
- **Secret setup (Castawordle answers, Spell It Out phrases):**
  - The files stay in a git-ignored `answers/` directory on your machine.
  - They reach the server through a one-time `probst season apply` run by you, which the readiness alert
    tells you to do.
  - Never in the image, the season file, logs, or a DM.
  - **Ready means the game exists on the server with matching settings**, not that a local file
    exists.

### 3. Episode results from survivoR

- **Completeness needs evidence:**
  - Rows existing isn't enough, since a missing challenge row looks the same as no challenge.
  - An episode counts as complete when survivoR's `episodes` table lists it **and** its boot order and
    challenge results are present.
  - Results are classified as `absent`, `incomplete`, `complete`, or `complete: no event` (e.g. no reward
    challenge).
- **Apply once, all or nothing:**
  - Each award has a stable key: `(instance, season, episode, survivoR event id, award kind)`.
  - The survivoR commit is stored alongside for provenance only. It's not part of the key, so a new
    upstream revision can't duplicate an award.
  - The whole episode is validated first, then applied in one transaction.
  - If upstream later disagrees with what was applied, that's a **conflict needing an admin**. Awarded
    points are never silently changed.
- **What gets scored:** boots, plus tribe immunity and reward wins (+2/+1) while `scoring.tribe` is active.
  - These are **held instead of guessed**:
    - individual immunity, and anything at or after `merge_episode`
    - a medevac, quit or no-vote boot, or two boots in one episode
    - a challenge with tied winners or an unknown tribe
  - A hold means a DM listing what needs a decision.
  - Today's `episode sync --yes` just skips holds, so it isn't the fix. Add explicit commands, e.g.
    `probst episode resolve-hold ep3:boot --contestant NAME`, that record the reviewed decision; the
    episode is complete only once every hold is resolved.
- **Tuesday 11:59pm:** if results are still missing, DM: "Episode N results aren't in survivoR yet;
  enter them with `probst challenge` / `probst episode sync`, or the scores post waits."

### 4. Scores post: drafting, approval, sending

- **Drafted only when ready:** game N is resolved, its ledger awards are committed, and episode N results
  are complete. Otherwise the post waits, and the admin is told why by 8:15pm.
- **Approval timing** (fixes the current future-time-only rule): the scheduled time becomes a "not before"
  time. A post drafted after its 8:00pm time is still accepted. It sends at the later of its time or
  approval, and expires 3 hours after its time, same as today. Other announcements keep the strict
  future-time rule.
- **Admin edits always win, and stale numbers never send:**
  - Once you've replaced the copy, automation never regenerates it.
  - If scoring changes after drafting (a correction or a late import), any unsent post goes back on hold,
    even if approved. Your edited text is kept, you get a DM with what changed, and it needs a fresh "yes".
  - An untouched draft is regenerated with the new numbers, then re-asked.
- **Facts are computed, not written by AI:**
  - standings, ranks and ties use the server's leaderboard: total, then draft points, then earliest
    draft
  - only players with drafts are listed
  - last week's totals come from a **finalized standings snapshot** saved when that week's scoring
    completes, not when a post is approved or sent, and not from a laptop file. It's used for gainer and
    slider numbers, and secret bonuses are excluded.
  - game results come from the actual award records: tied winners, best-three averages, award recipients,
    no-shows
  - tribe emoji come from stored colors, and links point to `/games`
  - optional AI flavor can only touch marked flavor lines, and the numbers and names are checked
    afterwards
- **Pings:** top 3 and last place only, never the role, as configured.

### 5. Season file

- `weekly.game.closes` is 7:59pm and `weekly.scores_post.at` is Wednesday 8:00pm (done).
- Add `automation: {enabled: true, admins_dm: all}` and a `none` game type.
- **`merge_episode` stays a manual setting in the file**, and nothing implements it in this phase. From
  that episode on (or for any individual win), scoring holds and alerts. Detecting the merge and the
  Champion flow are the next plan.
- **Times:** everything is `America/New_York`, with daylight saving time handled. That covers skipped or
  rescheduled episodes via `episodes.skip`, the finale (no next game; final scores the following
  Wednesday), and late catch-up within the 3-hour approval window.
- **Clean up stale claims in the file:**
  - "PROPOSAL — not yet read by anything" is now false
  - "Posts never send unless `approved: true`" is replaced by DM approval
  - "nagged 3 days before" becomes the readiness rule above
  - the `results` comment should say they come from survivoR
- **Fix `season plan`:**
  - show actually-sent posts as ✓ (the Week 2 post shows as unapproved)
  - show readiness states and holds

## Verification

- **Disposable-DB tests:**
  - reconciler idempotency: duplicate and overlapping runs, a restart mid-action
  - each readiness state
  - incomplete, ambiguous and corrected survivoR data
  - late drafting and late approval
  - an admin edit preserved across reruns
  - a stale revision rejected
  - draft accuracy, against tied winners and today's Episode 2 data as fixtures
  - partial imports, corrected upstream results, a no-game week, standings ties, secret bonuses excluded,
    and original draft order preserved
- **Accelerated BrainLand rehearsal with you:** a compressed week (minutes, not days):
  - a missing-game alert, then set up
  - the game opens and closes and scores
  - results are imported
  - the draft DM arrives, you edit by reply, approve, and it posts
  - kill the job mid-week and confirm it catches up
- **Activation:** after the BrainLand rehearsal passes, Podracing is turned on only with your explicit
  approval. Rollback means suspending the CronJob; the manual commands keep working.

## Milestones

1. Server: `automation_actions` table and API, not-before/expiry approval, per-week standings snapshots.
2. Probst: `season reconcile`, idempotent `apply`, readiness alerts, operator alerts.
3. Safe results import: completeness evidence, stable keys, all-or-nothing apply, holds with
   resolve commands, conflicts.
4. Accurate scores draft from server data.
5. Image, CronJob and automation credential; BrainLand rehearsal; phased Podracing activation.
6. Docs and changelogs for this and the Oct 7 changes.

## Status (Oct 8)

Built and running for Podracing (`seasons/51.yaml`) since Oct 7, after a compressed BrainLand rehearsal:

- `probst season reconcile` CronJob every 5 minutes, on the bot's token (dedicated token still to do).
- Next-game readiness alerts, plus one daily reminder DM coalescing every open issue.
- survivoR import: pinned commit, completeness evidence, holds, all-or-nothing server apply, legacy-key reuse,
  conflicts alert instead of overwriting.
- Scores drafts: snapshot + fingerprint; stale posts can't be approved or sent; admin edits kept; drafting
  up to 3 hours late; expiry and waiting alerts.

Not yet: `automation_actions` leases (each step is idempotent on the server instead), reconciler-health and
credential-rejected operator alerts, idempotent `season apply` in the reconciler, readiness `incomplete` /
`invalid` states, `season plan` showing sent/holds, the stale comments in the season file.

## Decisions (Oct 7)

- **First automated piece:** when a game opens, if the following week has no game set, DM the admins.
  Build this first (milestones 1–2 scope), then the rest.
- **Automation login:** a scheduled k3s pod configured from env like the bot. It uses a **dedicated
  service token** added to castaway-web's `SERVICE_AUTH_BEARER_TOKENS` (not the bot's token), stored in
  1Password and synced to a Kubernetes secret like the other credentials. It acts as an instance admin via
  `PROBST_DISCORD_USER_ID`. Revoke it by removing the token from the list.
- **Activation:** Podracing is turned on only after a successful BrainLand test.
- **Week 4:** the admin's prepared puzzle; set up separately.

## Earlier decisions (kept)

- **Scores post:**
  - top 3 and last place only, no `@castaway`
  - praise the leader and the biggest gainer, gently rib the biggest slider
  - consistent format with varied wording
- **Wording:** "your Tribe", never "pony".
- **Merge:** Pick Your Champion, +3 immunity and +1 reward, shareable picks. Next plan.
