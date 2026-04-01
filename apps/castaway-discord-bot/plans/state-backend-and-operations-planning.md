# State Backend and Operations Plan

Status: `done`

## Goal

Define the production storage and operational model for `castaway-discord-bot`.

## Resolution

Production state is now fully backed by PostgreSQL:

- `BOT_STATE_BACKEND=postgres` configured in deploy configmap
- `BOT_STATE_DATABASE_URL` provisioned via k8s secret pointing to the `castaway_discord_bot` database
- `PostgresStore` with auto-schema management handles `guild_defaults` and `user_defaults` tables
- BoltDB import path (`--import-bolt-state-from`) available for migration
- No local writable volume required in production deployment

Remaining operational concerns (token rotation, runbooks, alerting) are tracked in the production readiness checklists rather than this plan.

## Related threads

- `postgres-state-backend-planning.md` — PostgreSQL store implementation (done)
- `service-to-service-authentication-planning.md` — bot-side auth (done)
