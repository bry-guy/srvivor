# Changelog

## Unreleased

- Every announcement now waits for an admin's DM "yes": `message`, `announcement send` and `announcement schedule` save a draft and request approval (no direct posting; `--reply-to`/`--silent` removed, `schedule --yes` send-now removed).

- Press the Button scores section calls out every rule that fired (most, second, ties, fewest, volume, streaks, first, last).

- Admin DMs from the scheduled job (results missing, holds, import proposals, conflicts, scores waiting/expired, game not ready, daily nudge) now come from the season's `messages/*.md` templates; multi-person eliminations use `lines/boot-many.txt`.

- `probst season sync FILE --fix-episodes` sets the live episode schedule to the file's first (dry run shows each change); `probst season post --preview` renders a dry run before the week's game is scored.

- All post wording now lives in the season's template directory (`seasons/51-templates/`: `scores.md`, `reminder.md`, `next-game.md`, `games/<type>.md`, `lines/*.txt`); no message text in code. `probst season sync` stores the whole set. The scores post gains a per-game results section (`{{.GameScore}}`), and `weekly.reminder.at` adds a game-day reminder drafted for admin approval when the game opens (no pings).

- Pick Your Champion: `probst season merge FILE N` ends Tribe scoring at merge Episode N and opens picks (website `/games/champion`, or reply to the bot's "Pick Your Champion" post) until Episode N+1 airs; then picks are fixed and shown to all. Post-merge challenges import from survivoR and award pickers of the winners (+3 immunity, +1 reward). The merge episode's own challenges are held for an admin.

- Failure alert: each successful scheduled `season reconcile --instance` run stamps a heartbeat; if none lands for 30 minutes, admins get one DM per stall.

- Season config lives in the database: `probst season sync FILE` saves the season file and scores template (revision-checked, refuses on episode mismatches); the scheduled `season reconcile --instance ID` runs the synced copy.

- Episode imports that need judgment (medevac/quit, two people leaving) are DMed to admins as an exact proposal; replying **yes** records it (`POST /admin-alerts/:id/approve`, revision-bound). `probst episode import N --file F` replaces `season import`/`episode sync`.

- Spell It Out is now called **Island Scramble** (display name only; `spell_it_out`, URLs and data unchanged).
- Tests use their own season files (`testdata/`), so `seasons/*.yaml` can change freely; added a non-Wednesday/DST/skip-week schedule test.

- `season reconcile FILE` (the scheduled job): next-game readiness alerts, all-or-nothing survivoR episode import with holds for anything ambiguous, and scores posts drafted for admin approval once the week's game is scored and results are in. `season check FILE` runs only the readiness check; `season import FILE --episode N` runs the import by hand. Season files gain `automation: {enabled, from_week, guild}`.
- Daily reminder DM (`automation.nudge_at`, default 10am ET) collecting every still-open issue.
- `boot PLACE CONTESTANT` records a finishing place by hand (e.g. a medevac the import held).
- Scores posts: when everyone moved by the same amount, say so instead of "nobody moved"; game links point to `/games`.
- Plain HTTP is allowed only to loopback or the exact URL in `PROBST_ALLOW_HTTP_SERVER`.

- Infer a single unmatched whole name word from a complete draft's sole unused contestant, without adding artificial nicknames or changing fuzzy thresholds. Weak candidates remain confirmation-only suggestions.
- Add offline Keeling and Mooney original-message coverage, including all ranks, pending submission credit, and replay safety.

- `contestant rename CONTESTANT NEW_NAME`.

- `login`/`logout` (Discord login for the public `/api`), `admin add`, and `access list|approve|deny`.
- `draft watch THREAD_URL` and `draft reject PLAYER` for bot-read draft threads.
- `recap --episode N` prints the weekly scores post (spoilered boots, biggest gainer/loser, leaderboard with tribes) and diffs against last week's local snapshot.
- Wordle rounds now run 1pm ET on an episode's air day to noon on the next one's.
- `episode sync --season S --episode E` pulls survivoR data for an episode and records boots and tribal challenge wins.
- `draft open --tribes` / `draft close` run draft-submission events (+2/+1, balanced random tribes, pinged posts).
- `announcement mark-sent` records announcements you posted yourself.
- `scores` hides players without a draft.
- `--notify` on `message` and `announcement save`/`send` lets `<@user>` mentions ping.
- Config `aliases` give instances and channels friendly names (`--instance podracing`); with an instance alias, `CHANNEL` defaults to its channel for `message` and `announcement save`/`send`.
- Add draft announcements: `announcement save`/`show`/`edit`/`schedule`/`unschedule`/`delete` by name, with a readable `list`.
- Add `message` for one-off, unsaved bot posts (optionally as a reply).
- `wordle import --file` reads hand-scored results.
- Add the weekly loop: `tribes set`/`show`, `challenge immunity|reward`, and `wordle open`/`import`/`submit`/`resolve`.
- Add `announcement send`/`list`: queue verbatim Discord announcements (now or `--at` a scheduled time) that the Castaway bot posts with mentions disabled.
- Add optional owner-only local JSON configuration for Probst API and Discord credentials, with environment and flag overrides.
- Add season setup (`instance create`, `contestant list`, `player add`) and `draft import` from Discord threads or text files, with chat-tolerant parsing and dry-run review.

- Add the Probst HTTP operator client for instance inspection, first-admin bootstrap, channel bindings, player links, drafts, and public scores.
- Add destructive-action confirmation, structured output, and disposable API integration coverage.
