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

## Historical season rehearsal

Run `MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:rehearsal-season43` for a disposable, repeatable Season 43 test with six fake drafts and thirteen public-bonus Wordle rounds. Historical inputs are pinned to survivoR; no online data or Discord access is needed by default. See the [rehearsal guide](../../docs/guides/season43-rehearsal.md) for fixture editing, explicit BrainLand delivery, and observed results. This uses legacy mode and leaves the managed YAML scenario unchanged.

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
