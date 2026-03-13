# castaway-web

`castaway-web` is a Gin + PostgreSQL web API for persistent Survivor fantasy drafts.

## Documentation

- Changelog: `CHANGELOG.md`
- Functional requirements: `functional-requirements.md`
- Non-functional requirements: `non-functional-requirements.md`
- Production readiness: `production-readiness-checklist.md`
- Plans: `plans/`
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

## Seed data

Historical seasons are captured in:

- `seeds/historical-seasons.json`

Regenerate from legacy CLI data (`../cli/drafts`, `../cli/rosters`):

```bash
mise run generate-seeds
```

## Deferred work

Bonus points (`ponies`, immunity, journeys, etc.) are intentionally deferred.
See `../../docs/castaway-web-future-work.md`.
