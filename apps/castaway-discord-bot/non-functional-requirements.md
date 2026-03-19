# castaway-discord-bot Non-Functional Requirements

## Security

- Discord secrets must be injected through managed secret tooling and never committed.
- Bot-to-API traffic must support bearer-token authentication for production deployments.
- Guild-scoped state changes must require appropriate Discord permissions.
- Logs must avoid leaking tokens or sensitive payload content.

## Reliability

- Startup must fail fast when required configuration is missing or invalid.
- The bot must surface actionable user-facing failures when `castaway-web` is unavailable.
- The saved-instance state store must remain durable enough for the chosen deployment model.

## Observability

- Structured logs should be emitted for Discord interaction failures, Castaway API failures, and state-store errors.
- Operational guidance must exist for token rotation, restarts, and failed command handling before production use.

## Performance

- Commands should prefer filtered API lookups over unnecessary full-list fetches.
- Discord interaction handling must stay within Discord timing expectations.
- Response formatting must respect Discord message length limits.

## Operations

- The local development path must remain reproducible through the repo-level `mise` tasks.
- The bot must support an explicit operator-run path to migrate saved defaults from BoltDB into PostgreSQL.
- Multi-replica production deployment requires a shared state backend or an equivalent design update.
