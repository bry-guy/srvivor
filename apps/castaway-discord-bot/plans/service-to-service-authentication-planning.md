# Service-to-Service Authentication Plan

Status: `done`

Implementation completed — bearer token injected via `castaway.Options{BearerToken}` in API client, `Authorization: Bearer` header sent on every request, config loaded from `CASTAWAY_API_AUTH_TOKEN` env var.

## Goal

Define the bot-side production authentication model for calls from `castaway-discord-bot` to `castaway-web`.

## Why this exists

The bot depends on `castaway-web` for all meaningful data reads. In production, bot-to-API traffic should not be treated as anonymous, even in a private self-hosted environment.

This plan focuses on the bot as a service client. It complements the API-side plan in `apps/castaway-web/plans/service-to-service-authentication-planning.md`.

## Recommended v1 model

Use a shared bearer token first.

Behavior:

- the bot loads a service-auth secret from Kubernetes
- the bot sends that token with every API request
- the bot fails fast when service auth is required but the token is missing or invalidly configured

## Client expectations

### Request behavior

Every request from the bot to `castaway-web` should include the configured service-auth header.

### Error handling

The bot should treat auth failures distinctly from general API outages.

Examples:

- `401` or `403` should be logged as an auth/configuration problem
- timeouts and `5xx` should be logged as dependency availability problems
- user-facing responses should remain actionable without leaking operational details

### Logging safety

Logs must never include the raw token or full authorization header values.

## Configuration expectations

The exact env var names can be finalized during implementation, but the bot should support:

- local development without service auth by default
- production mode with a required service-auth token
- explicit startup validation when auth is expected

## Rotation expectations

Token rotation should be boring.

Recommended rollout shape:

1. API accepts both old and new tokens temporarily
2. bot secret is updated to the new token
3. bot is restarted or rolled out
4. API drops the old token

The bot does not need complex multi-token logic if the API handles the overlap window.

## Implementation phases

### Phase 1: config and client wiring

- add config support for the API auth token
- inject the header in the Castaway API client
- validate required production config at startup

### Phase 2: tests

- add client tests that verify auth headers are attached
- add config tests for missing or malformed auth config

### Phase 3: operator behavior

- document auth-related failure modes
- document token rotation procedure
- ensure production logs and health signals make auth misconfiguration obvious

## Resolved questions

- should the bot have an explicit production-mode flag, or is presence of the token enough to imply auth behavior? → **Resolved: token presence implies auth; empty token means no auth header sent**
- does the first version need anything beyond one bearer token and one service principal? → **Resolved: one token, one principal is sufficient for v1**

## Open questions

- should the bot expose a local health indicator that can surface repeated auth failures distinctly from general API failures?

## Related threads

- `../../castaway-web/plans/service-to-service-authentication-planning.md`
- `state-backend-and-operations-planning.md`
- `../../non-functional-requirements.md`
