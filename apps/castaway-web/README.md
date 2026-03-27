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
mise run build
mise run run
mise run migrate
mise run sqlc
mise run generate-seeds
mise run seed
mise run openapi
./bin/castaway-web --version
```

## Production deployment note

For self-hosted Kubernetes deployments, production migrations should run through a dedicated migration Job or equivalent pre-traffic hook. Do not rely on app-startup auto-migration for production rollouts.

The production container image now includes a dedicated migration entrypoint:

- `/app/castaway-web-migrate`

Recommended production defaults for the web Deployment:

- `AUTO_MIGRATE=false`
- `SERVICE_AUTH_ENABLED=true`
- `SERVICE_AUTH_BEARER_TOKENS` populated from managed secrets
- `SERVICE_AUTH_PRINCIPAL=castaway-discord-bot`

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
- `PUT /instances/:instanceID/drafts/:participantID`
- `GET /instances/:instanceID/drafts/:participantID`
- `PUT /instances/:instanceID/outcomes/:position`
- `GET /instances/:instanceID/outcomes`
- `GET /instances/:instanceID/leaderboard` (`participant_id` filter supported)
- `GET /instances/:instanceID/activities`
- `POST /instances/:instanceID/activities`
- `GET /activities/:activityID/occurrences`
- `POST /activities/:activityID/occurrences`
- `POST /occurrences/:occurrenceID/participants`
- `POST /occurrences/:occurrenceID/groups`
- `POST /occurrences/:occurrenceID/resolve`

## Seed data

Historical seasons are captured in:

- `seeds/historical-seasons.json`

Season 50 now seeds first-class bonus gameplay structures, including participant groups, `tribal_pony`, `tribe_wordle`, and journey occurrences, while preserving the historical leaderboard end-state.

Regenerate from legacy CLI data (`../cli/drafts`, `../cli/rosters`):

```bash
mise run generate-seeds
```

## Follow-on work

Core bonus points (`ponies`, immunity, journeys, etc.) are implemented.
See `../../docs/castaway-web-future-work.md` for remaining operator/API/auth follow-up work.
