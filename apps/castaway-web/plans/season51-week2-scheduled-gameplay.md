# Season 51 Week 2: scheduled gameplay and tribe colors

Status: planning

## Confirmed requirements

This is a plan, not authorization to schedule posts or mutate production. Target the existing Podracing Season 51 instance `ee0a0884-37ad-47bb-820b-cf111bec3a39`; do not recreate or change its mode.

- September 30, 2026, 7pm EDT: one pre-episode scores announcement, with a website link, tribe colors, puzzle link/window, and a single-sentence +2 immunity / +1 reward reminder.
- September 30, 2026, 8pm EDT: open one six-letter Castawordle for linked instance players. Use the operator-provided answer through a private, untracked input/reference, not public configuration or announcement text.
- October 7, 2026, 11:59am EDT: reject further guesses and finalize the round. Award its points afterward alongside Episode 2 draft updates and tribal-pony bonuses for the next scores update. Do not award these bonuses tonight or retroactively for Episode 1.
- App scheduling only, not a Discord Scheduled Event. Use America/New_York for human-facing times and DST-aware conversion.
- Website Scores: a small tribe-colored dot beside each player name, retaining accessible tribe text and the existing five-column table.
- Declarative preparation must validate/diff before applying, use stable keys, and be safe to reapply without duplicate games, announcements, or ledger awards.

## Verified current state

- Code is merged; the previous rollout is healthy. No new production mutations were made during this planning effort.
- The latest Season 51 announcement read contains only sent entries; no pending scores post was found. Treat this as preparation of a new unsent announcement unless another existing job is identified.
- Live scores currently show Marv 3, Keeling 2, and the other six listed players 1 each. Refresh before approving the final snapshot rather than treating these as the eventual 7pm scores.
- Sent buff announcements identify Savu as purple and Toka as yellow. Color is not a stored/public leaderboard field today; existing draft/bot formatters use hardcoded badges.
- `docs/castaway-manual-gameplay-logs.md` preserves actual Season 50 Weeks 1–3 score posts: a brief Survivor recap with spoiler-hidden eliminations, ranked mentions/tribe emoji, Total (Draft+Bonus), short competitive commentary, then the next game introduction. These are archived posts, not a fresh Discord channel-history read. No general history CLI or connected Discord MCP was found.
- `internal/scenario` YAML is a disposable-database rehearsal compiler, not a production reconciler. Historical seed JSON can delete/recreate matching instances and must not be used for this rollout.
- The existing bot already delivers due saved announcements. Episode-linked Castawordle currently rejects explicit opening/cutoff timestamps and derives 1pm/noon windows; custom-time games without episode numbers are admin-only.
- Castawordle plays persist but remain unscored. Season 51's +2/+1 tribe resolver uses `tribe_challenge`, distinct from the older `tribal_pony` resolver. Live activity activation/configuration has not yet been verified.

## Implementation plan, after approval

1. Inspect the live instance's activities, assignments, eligibility windows, and existing awards through authenticated read-only tooling. Verify the Season 51 tribe challenge is the authoritative tribal-pony mechanism; do not activate both payout paths. Eligibility begins with Episode 2. Confirm the existing Wordle resolution/closure worker capabilities before adding any job.
2. Add a narrow production-safe weekly declaration and dry-run/apply entrypoint using existing API operations. Bind it to the existing instance/episode; reference approved announcement text without rewriting sent posts; configure tribe appearance, explicit puzzle window, and a private answer reference. Preserve prior trials/progress and forbid destructive reseeding. Prefer existing workers/time gates over a new scheduler service.
3. Allow validated admin-controlled windows for player-visible episode-linked puzzles, while retaining episode uniqueness, linked-instance authorization, answer secrecy, private trials, and server-side opening/cutoff enforcement.
4. Connect saved server-validated results to the existing Wordle activity/round/ledger, with no awards before cutoff and retry-safe finalization after cutoff. Inspect/agree award rules and abandoned/all-failed behavior before activation. Verify Episode 2 tribe/draft updates complete before publishing next week's refreshed scores; hold/report incomplete inputs rather than silently publishing stale scores.
5. Provide one authoritative tribe appearance mapping to leaderboard API, Discord scores/recap formatting, and accessible website dots. Preserve Discord Draft+Bonus breakdown and the website's removal of the Draft column.
6. Run CI and disposable PostgreSQL/mobile/desktop checks: exact time boundaries, answer secrecy, player/admin isolation, colors, repeated declaration application, concurrent finalization, no early awards, and exactly one award per source. Seek permission before modifying conflicting regression tests.
7. Review copy separately, dry-run the declaration, obtain explicit approval, deploy the implementation, then apply/read back the one puzzle and announcement. Verify destination, 7pm send time, 8pm opening, 11:59am cutoff, live activity eligibility, and next-week scoring path without test posts/guesses in Podracing.

## Remaining approvals / evidence

- Existing Season 51 opt-in Wordle scoring awards +2 to best individual guess count and +1 to every member of the best-average tribe; ties share awards and non-submitters are excluded from averages. Confirm this is the intended public-round policy, including how failed and abandoned games count and whether an all-failed round awards anything.
- The standing external-writing policy forbids authored announcement drafts/publication. This document supplies requirements and structure, not message copy. User-authored copy must be reviewed under `.agents/skills/castaway-announcements/SKILL.md` before scheduling. Preserve any existing non-empty body.
- Additional past-season style examples require representative posts/export or suitable authenticated history tooling; do not look for credentials or bypass the historical-instance authorization denial.
