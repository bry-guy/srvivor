# Changelog

## Unreleased

- `season reconcile FILE` (the scheduled job): next-game readiness alerts, all-or-nothing survivoR episode import with holds for anything ambiguous, and scores posts drafted for admin approval once the week's game is scored and results are in. `season check FILE` runs only the readiness check; `season import FILE --episode N` runs the import by hand. Season files gain `automation: {enabled, from_week, guild}`.
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
