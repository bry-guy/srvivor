# PostgreSQL State Backend Plan

Status: `done`

Implementation completed — `PostgresStore` in `internal/state/postgres_store.go` with auto-schema management, `Store` interface with `Open()` factory supporting both `bolt` and `postgres` backends via `BOT_STATE_BACKEND` config.

## Goal

Move `castaway-discord-bot` off the local BoltDB file and onto PostgreSQL while keeping the bot logically separate from `castaway-web`.

## Desired end state

Use the same PostgreSQL instance as `castaway-web`, but not the same logical database.

Target shape:

- one PostgreSQL instance for the self-hosted deployment
- database `castaway_web` for `castaway-web`
- database `castaway_discord_bot` for `castaway-discord-bot`
- separate database credentials for each app
- bot state stored in PostgreSQL instead of `BOT_STATE_PATH`

This keeps the first self-hosted deployment simple while preserving an easy split later.

## Why this is the preferred next step

### Operational simplicity

- one database service to operate, back up, and restore
- no special PVC behavior needed for a local bot file store once migration is complete
- bot restarts and rollouts become less brittle

### Scalability and portability

- shared durable state is a prerequisite for cleaner future scaling
- separate logical databases mean the bot can later move to a separate Postgres instance with less application churn
- the bot stops depending on node-local filesystem semantics

## Scope of bot state

Current state held locally:

- guild default instance selections
- user default instance selections

Initial PostgreSQL scope should cover exactly those persisted defaults.

## Proposed data model direction

The exact schema can be finalized during implementation, but the initial relational shape should stay small and explicit.

Possible tables:

- `guild_defaults`
  - `guild_id`
  - `instance_id`
  - timestamps if useful
- `user_defaults`
  - `guild_id`
  - `user_id`
  - `instance_id`
  - timestamps if useful

Prefer uniqueness constraints that match the current key semantics rather than introducing extra abstraction.

## Configuration direction

The bot should move toward a database URL-based configuration model.

Likely shape:

- introduce a bot database connection setting sourced from Kubernetes secrets
- keep `BOT_STATE_PATH` only as a temporary compatibility path during migration
- remove file-state assumptions once PostgreSQL is the default

## Migration concerns

### Data migration

The current BoltDB file may contain useful saved defaults.

Migration options to evaluate:

- one-time import command that reads BoltDB and writes PostgreSQL
- startup import path behind an explicit migration flag
- operator-run admin tooling

The migration should be explicit and idempotent.

### Rollout shape

Recommended transition:

1. add a storage abstraction that supports both BoltDB and PostgreSQL backends
2. implement PostgreSQL backend with tests
3. provide a one-time import path from BoltDB
4. switch deployment config to PostgreSQL
5. remove file-backed production assumptions

## Operational expectations

### Backups

Once the bot uses PostgreSQL, backup and restore ride on the shared PostgreSQL operational path, while still preserving logical separation via a dedicated bot database.

### Credentials

Use a distinct bot database role and password even on the shared instance.

### Rollouts

Once file-backed local state is gone, the bot no longer needs deployment semantics shaped around a writable local state file.

That does not automatically imply multi-replica safety for every future bot design concern, but it removes the current local-state blocker.

## Resolved questions

- should the bot adopt the same migration helper pattern that `castaway-web` uses, or should it use a lighter dedicated SQL migration path? → **Resolved: lighter `ensureSchema()` approach with inline CREATE TABLE IF NOT EXISTS**
- do we want timestamps or audit metadata on saved defaults, or should the first relational schema stay minimal? → **Resolved: `updated_at` timestamp included on both tables**

## Open questions

- is a one-time import from BoltDB sufficient, or is a temporary dual-read strategy needed?

## Implementation phases

### Phase 1: backend design

- define storage interface boundaries if they need refinement
- decide on bot database config shape
- define SQL schema and migration approach

### Phase 2: PostgreSQL backend

- implement PostgreSQL store
- add tests for reads, writes, clears, and conflict behavior
- keep key semantics aligned with the current BoltDB behavior

### Phase 3: migration path

- build explicit BoltDB-to-PostgreSQL import tooling
- document cutover steps
- validate migrated data in a staging or local environment

### Phase 4: production cutover

- deploy bot with PostgreSQL-backed config
- stop relying on `BOT_STATE_PATH` in production
- update documentation and operational runbooks

## Related threads

- `state-backend-and-operations-planning.md`
- `service-to-service-authentication-planning.md`
- `../../non-functional-requirements.md`
