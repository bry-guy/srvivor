# Non-functional requirements

This document tracks cross-cutting requirements that apply across Castaway apps, including `castaway-web` and `castaway-discord-bot`.

## Security

### API authentication
- `castaway-web` currently needs a production-ready authentication story before it is exposed beyond trusted local use.
- Read-only local development can proceed without auth.
- Any production deployment that supports bot traffic should require authenticated bot-to-API access.

### Authorization
- Discord commands that change shared context or future draft state must enforce permissions.
- Guild-scoped configuration changes should require Discord Manage Server permissions.
- Any future write workflow should define a server-side authorization model, not just a Discord UI check.

### Secrets management
- Secrets must not be committed to the repository.
- The monorepo standard is the shared 1Password vault `castaway` accessed through the root `fnox.toml` provider.
- Apps should select only the fnox profile they need via `mise.toml` and keep non-secret defaults in `mise.toml` env blocks.
- Discord tokens and production API credentials should be provided via fnox-backed environment injection or an equivalent managed secret provider.
- Logs must never include secrets or authorization headers.

## Reliability

### Availability assumptions
- `castaway-discord-bot` depends on `castaway-web` being reachable.
- The bot should return actionable user-facing errors when the API is unavailable.
- Startup should fail fast on invalid configuration.

### State durability
- MVP bot defaults are stored locally.
- Local file-backed state is acceptable for a single local or single-instance deployment.
- Multi-replica deployments require a shared state backend.

## Observability

- Use structured logging for request failures, Discord interaction failures, and state-store errors.
- Avoid logging raw Discord payloads in production unless redaction rules are in place.
- Health checks and service-level alerts are required before production rollout.

## Performance and rate limits

- The bot should avoid unnecessary full-list fetches where API filters exist.
- Discord interaction responses should acknowledge quickly and use follow-up responses if work could exceed Discord timing limits.
- Autocomplete handlers should keep remote lookups bounded and lightweight.

## API contract management

- TypeSpec is the source of truth for the documented `castaway-web` API.
- Generated OpenAPI must be committed.
- CI must fail when generated OpenAPI differs from committed output.
- CI must fail when documented routes diverge from registered Gin routes.

## Deployment and operations

- Production rollout requires documented runbooks for bot restarts, API outages, and credential rotation.
- Backups or migration plans are required for any persistent bot state used in production.
- Rollback steps should be documented for API and bot releases.
