# castaway-web

`castaway-web` is a Gin + PostgreSQL web API for persistent Survivor fantasy drafts.

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
```

## OpenAPI

- TypeSpec source: `typespec/main.tsp`
- Generated OpenAPI: `openapi/openapi.yaml`

Regenerate:

```bash
mise run openapi
```

## API (MVP)

- `GET /healthz`
- `GET /instances`
- `POST /instances`
- `GET /instances/:instanceID`
- `POST /instances/:instanceID/contestants`
- `GET /instances/:instanceID/contestants`
- `POST /instances/:instanceID/participants`
- `GET /instances/:instanceID/participants`
- `PUT /instances/:instanceID/drafts/:participantID`
- `GET /instances/:instanceID/drafts/:participantID`
- `PUT /instances/:instanceID/outcomes/:position`
- `GET /instances/:instanceID/outcomes`
- `GET /instances/:instanceID/leaderboard`

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
