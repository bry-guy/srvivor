# castaway-web

`castaway-web` is a Gin + PostgreSQL web API for persistent Survivor fantasy drafts.

## Documentation

- Changelog: `CHANGELOG.md`
- Functional requirements: `functional-requirements.md`
- Non-functional requirements: `non-functional-requirements.md`
- Production readiness: `production-readiness-checklist.md`
- Shared future-work notes: `../../docs/castaway-web-future-work.md`

## Stack

- Gin HTTP server
- PostgreSQL 16
- SQL-first data access via `sqlc`
- `pgx` connection pool

## Local dev

From repo root:

```bash
mise run start
```

This starts:
- `castawaydb` on `localhost:5432`
- `castaway-web` on `localhost:8080`

Seed historical seasons:

```bash
mise run seed
```

Stop stack:

```bash
mise run stop
```

Useful ops:

```bash
mise run ps
mise run logs
mise run db-shell
mise run db-reset
mise run openapi
mise run openapi-check
```

After seeding, try:

```bash
curl http://localhost:8080/instances | jq
```

## App tasks

```bash
cd apps/castaway-web
mise run lint
mise run test
mise run integration
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:season-scenario
mise run build
mise run run
mise run migrate
mise run sqlc
mise run generate-seeds
mise run seed
mise run openapi
./bin/castaway-web --version
```

## Integration tests

Integration tests create temporary databases and run migrations themselves.

Preferred local path:

```bash
cd apps/castaway-web
mise run integration
```

That task starts an ephemeral local PostgreSQL 16 container, sets `CASTAWAY_TEST_DATABASE_URL`, runs the integration suites, and tears the container down.

Run the canonical YAML-managed Hurl scenario from the repository root with:

```bash
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:season-scenario
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:season-scenario scenarios/two-episode.yaml
```

The scenario task creates its own disposable PostgreSQL database and local service, then generates and runs one sequential Hurl file. Scenario schedules are contiguous and start at episode 0, which can represent preseason. It does not resume whole-file runs or support managed activity mechanics yet.

You can still point the tests at another non-prod Postgres instance by setting `CASTAWAY_TEST_DATABASE_URL` manually.

The captured Keeling and Mooney original-message tests run offline against disposable databases:

```bash
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:integration -- -run 'TestDraftThread(Keeling|Mooney)OriginalMessage' -count=1
```

The watched-thread parser keeps existing exact/fuzzy thresholds. It can infer one unmatched whole name word only in a complete, valid, duplicate-free draft when there is one unused contestant. Weak candidate suggestions require confirmation through a corrected message or targeted Probst file import; they do not save picks or award rewards. Tests assert every rank, preserved second-submission credit, and replay idempotency. See [the matching plan](../probst/plans/contextual-draft-name-matching.md) and [fixture provenance](../probst/testdata/README.md).

## Historical season rehearsal

Run `MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:rehearsal-season43` for a disposable, repeatable Season 43 test with six fake drafts and thirteen public-bonus Wordle rounds. Historical inputs are pinned to survivoR; no online data or Discord access is needed by default. See the [rehearsal guide](../../docs/guides/season43-rehearsal.md) for fixture editing, explicit BrainLand delivery, and observed results. This uses legacy mode and leaves the managed YAML scenario unchanged.

## Castawordle preview

All content pages require Discord login. `/` shows scores, `/castawordle` lists the configured season's games, and `/castawordle/{gameID}` resumes one game. Unlinked accounts see the access-request screen. OAuth routes, static assets, and `/healthz` remain reachable without a session.

Instance admins prepare **unscored** puzzles from the game-list page or `POST /api/instances/{instanceID}/castawordle` with `name` and `answer`:

- Omit `episode_number` (or use `null`) for an **admin-only test**. This includes existing preview games. Tests default to opening immediately for seven days; optional `opens_at` / `cutoff_at` remain supported for tests.
- Select `episode_number` for a **scheduled player puzzle**. The server uses that instance's episode dates: 1pm Eastern on the selected episode day through noon on the next episode day, including DST. Prepare it ahead of time; linked players can see it, but guesses cannot start before opening. No background scheduler is required.
- Scheduled puzzles reject unknown episodes, intermediate schedule gaps, caller-provided timestamps, and expired windows; allow one puzzle per instance/episode. A duplicate returns 409 without replacing the answer. For the final registered episode, cutoff is noon seven Eastern calendar days later (next Wednesday for Season 51), without creating a synthetic episode.

Answers are validated against the pinned dictionary and determine the 4–8-letter board width. Games cannot be edited after creation. Existing answers and saved progress are preserved when trials become admin-only; admins still need a linked player to play.

Players use `GET /api/castawordle/{gameID}/play` and `POST .../play/guesses` with `guess` and the next one-based `position`. The server owns the answer, feedback, six-guess limit, saved progress, and terminal state. Repeating the same guess/position is idempotent; a conflicting move returns 409. Cookie writes require the same-origin `Origin` header.

**No scoring inputs or bonus awards are written.** Connecting browser results to the existing Wordle scorer is deferred. Admin-only test visibility on lists/pages/play/guess APIs, schedule-derived opening/cutoff, own-player progress, cross-instance denial, concurrency, and zero scoring effects are covered by disposable-database tests.

The preview uses SCOWL/English Speller Database size 70, pinned revision `1e5b7d3a72f47a71da5d28686c1dd4b397178485`, with American/British spellings and inflections: **59,212** alphabetic 4–8-letter guesses. It includes `SWADDLE`; no individual-word patches or runtime downloads. Curate familiar answers separately. [Source, generation and license](internal/castawordle/data/README.md). Existing CMUdict-backed unscored trials require a guarded answer-compatibility check and explicit dictionary-metadata upgrade; answers, progress and scoring remain unchanged. Historical saved guesses remain replayable even if absent from the new list.

Press Enter (keyboard or on-screen) to submit a guess. Invalid words show `invalid word, try again`, consume no turn, and the message clears when the guess changes. The Scores page shows total and bonus points without a separate Draft column. See [the implementation plan](../../plans/castawordle-and-responsive-site.md).

For the one-time unscored preview dictionary upgrade, supply `DATABASE_URL` through your credential provider and run the read-only check before rollout:

```bash
mise exec -- go run ./cmd/upgrade-preview-dictionary
```

After deploying the compatible SCOWL web image, explicitly apply with `--apply --rollback-file /private/local/path/dictionary-rollback.json`. The command rechecks answers under row locks, blocks incompatible answers without printing them, and updates only the legacy trial dictionary version. The new rollback file is mode 0600 and contains versions/game IDs, not answers or guesses. Keep it untracked. To restore metadata, use its recorded IDs and old version in a guarded transaction after restoring a compatible image; do not rewrite answers or plays. No schema migration or score award is involved.

### Profiles

**Me** (`/me`) and player profiles (`/players/<participant-id>`, linked from Scores) show one season's score and draft plus the player's seasons in this league. Seasons are joined by Discord account and limited to `PUBLIC_INSTANCE_ID` plus `PUBLIC_LEAGUE_INSTANCE_IDS` (`PUBLIC_LEAGUE_NAME` labels them); only current-season players and their league history are viewable. Scores use visible bonus only (secret points never show). Drafts are soft-closed, so other players' current-season drafts appear only after draft submissions close and the viewer has saved a draft; owners and admins always see them, and past seasons are open. Missing data shows "Unavailable".

To link past seasons' unlinked players to the Discord account of the same-named (ignoring case) current player, run `go run ./cmd/link-league-history` (dry run) then `--apply`, with `DATABASE_URL` and the public league variables set. Ambiguous or conflicting names are skipped.

Optional browser checks use an already-installed Node/Playwright runtime, without installing global tools:

```bash
CASTAWAY_BROWSER_TEST=1 mise run integration -- -run TestCastawordle -count=1
```

Make Playwright resolvable by Node in your local environment. These tests run fake Discord OAuth and games against a disposable database, covering 320px and desktop layouts, theme persistence, touch/physical input, lost-response retries, and cross-device resume. They do not authenticate to production.

## Manual Wordle rounds

For legacy instances, create a `tribe_wordle` activity, then use:

- `POST /activities/{activityID}/wordle-rounds` with `round_key`, `name`, `opens_at`, and `cutoff_at` (timestamps include a timezone).
- `PUT /wordle-rounds/{roundID}/participants/{participantID}` with `participant_group_id` and positive `guess_count` to enter or replace a result during `[opens_at, cutoff_at)`.
- `POST /wordle-rounds/{roundID}/close` to freeze submissions, then `POST /wordle-rounds/{roundID}/resolve` at or after cutoff to award points.
- `GET /wordle-rounds/{roundID}` to inspect inputs and resolution.

All five operations require service authentication and an instance-admin `X-Discord-User-ID`. Creation retries reuse `round_key`; changed creation details conflict. Close and resolution are retry-safe. Generic occurrence writes cannot modify these rounds. Managed instances remain unsupported.

The existing scorer averages each tribe's best **up to three** submitted guesses, permits tied winning tribes, and awards **one public bonus point to every active member** of each winning tribe at cutoff, including members who did not submit. Groups with fewer than three results participate. There is no post-cutoff editing, scheduler, or Discord announcement automation yet. These rules still require launch acceptance.

Upcoming-season public-only enforcement and retirement of Stir the Pot, auctions, and loans are planned separately; these endpoints do not disable existing mechanics. Historical secret balances remain unchanged.

## Discord administration

`BOOTSTRAP_ADMIN_DISCORD_USER_ID` defaults empty. Configure it only for the initial operator; `POST /instances/:instanceID/admins/bootstrap` additionally requires service authentication, a matching asserted `X-Discord-User-ID`, and no existing instance admins. An already-authorized retry is idempotent. No existing instances receive admins automatically.

Channel bindings and player linking require service authentication and transaction-bound instance-admin authorization. Rebinding requires admin access to both the old and new instances; use Probst. See [`apps/probst`](../probst/README.md).

## Production deployment note

For self-hosted Kubernetes deployments, production migrations should run through a dedicated migration Job or equivalent pre-traffic hook. Do not rely on app-startup auto-migration for production rollouts.

The production container image now includes a dedicated migration entrypoint:

- `/app/castaway-web-migrate`

Recommended production defaults for the web Deployment:

- `AUTO_MIGRATE=false`
- `SERVICE_AUTH_ENABLED=true`
- `SERVICE_AUTH_BEARER_TOKENS` populated from managed secrets
- `SERVICE_AUTH_PRINCIPAL=castaway-discord-bot`

Public listener (off unless `PUBLIC_PORT` is set): serves the website, Discord login (`/auth/*`), and the API under `/api` with session auth only. It never honors the service token or `X-Discord-User-ID`. Requires `PUBLIC_BASE_URL`, `DISCORD_OAUTH_CLIENT_ID`, and `DISCORD_OAUTH_CLIENT_SECRET`; `PUBLIC_INSTANCE_ID` picks the season shown. See `../../plans/public-website-planning.md` and `../../plans/castawordle-and-responsive-site.md`.
- leave `BOOTSTRAP_ADMIN_DISCORD_USER_ID` empty after explicit first-admin bootstrap

`/healthz` remains unauthenticated for cluster health checks.

## OpenAPI

- TypeSpec source: `typespec/main.tsp`
- Generated OpenAPI: `openapi/openapi.yaml`

Regenerate:

```bash
mise run openapi
```

Verify that committed OpenAPI stays in sync with TypeSpec and the registered Gin routes:

```bash
mise run openapi-check
```

## Regression coverage

A self-contained Hurl suite lives in `hurl/` and exercises:
- seeded historical read behavior for seasons 49 and 50
- create/update leaderboard workflows
- import alias normalization behavior

Run it with:

```bash
mise run regression
```

The task starts a disposable PostgreSQL container, seeds historical data, runs `castaway-web` locally, and executes the Hurl files.

## API (MVP)

- `GET /healthz`
- `GET /instances` (`season`, `name` filters supported)
- `POST /instances`
- `POST /instances/import`
- `GET /instances/:instanceID`
- `POST /instances/:instanceID/contestants`
- `GET /instances/:instanceID/contestants`
- `POST /instances/:instanceID/participants`
- `GET /instances/:instanceID/participants` (`name` filter supported)
- `GET /admin/session`
- `POST /instances/:instanceID/admins/bootstrap`
- `GET|PUT|DELETE /discord/guilds/:guildID/channels/:channelID`
- `PUT|DELETE /instances/:instanceID/participants/:participantID/discord-link`
- `PUT /instances/:instanceID/drafts/:participantID`
- `GET /instances/:instanceID/drafts/:participantID`
- `PUT /instances/:instanceID/outcomes/:position`
- `GET /instances/:instanceID/outcomes`
- `GET /instances/:instanceID/leaderboard` (`participant_id` filter supported; rows also include linked `participant_discord_user_id` and `current_tribe_name` when available)
- `GET /instances/:instanceID/activities`
- `POST /instances/:instanceID/activities`
- `GET /activities/:activityID/occurrences`
- `POST /activities/:activityID/occurrences`
- `POST /occurrences/:occurrenceID/participants`
- `POST /occurrences/:occurrenceID/groups`
- `POST /occurrences/:occurrenceID/resolve`
- Merge gameplay routes
  - `GET /instances/:instanceID/stir-the-pot/me`
  - `POST /instances/:instanceID/stir-the-pot/start`
  - `POST /instances/:instanceID/stir-the-pot/close`
  - `POST /instances/:instanceID/stir-the-pot/me/contributions` (linked self by default; admins may target another participant via `participant_id`)
  - `GET /instances/:instanceID/auction/me`
  - `POST /instances/:instanceID/auction/lots/start`
  - `POST /instances/:instanceID/auction/lots/:contestantID/stop`
  - `PUT /instances/:instanceID/auction/contestants/:contestantID/bid/me` (linked self by default; admins may target another participant via `participant_id`)
  - `POST /instances/:instanceID/merge-auction/record` (admin-only resolved-result recorder for Merge Auction)
  - `GET /instances/:instanceID/ponies/me`
  - `GET /instances/:instanceID/loan-shark/me`
  - `POST /instances/:instanceID/loan-shark/me/borrow`
  - `POST /instances/:instanceID/loan-shark/me/repay`
  - `POST /instances/:instanceID/individual-pony/immunity`

## Seed data

Historical seasons are captured in:

- `seeds/historical-seasons.json`

Local merge-gameplay verification scaffolding lives in:

- `seeds/verification-merge-gameplay.json`

Season 50 now seeds first-class bonus gameplay structures, including participant groups, `tribal_pony`, `tribe_wordle`, and journey occurrences, while preserving the historical leaderboard end-state.

The verification seed is intentionally small and contrived. It exists to exercise Stir the Pot, auction bidding, refund behavior, Loan Shark borrowing/repayment, secret-point reveal conversion, episode-targeted merge windows, and individual pony immunity payouts in local integration tests.

Regenerate from legacy CLI data (`../cli/drafts`, `../cli/rosters`):

```bash
mise run generate-seeds
```

## Follow-on work

Core bonus points (`ponies`, immunity, journeys, etc.) are implemented.
See `../../docs/castaway-web-future-work.md` for remaining operator/API/auth follow-up work.
