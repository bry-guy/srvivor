# Service-to-Service Authentication Plan

Status: `done`

Implementation completed — bearer-token service auth middleware in `internal/httpapi/auth.go`, tests in `auth_test.go`, `/healthz` exemption, multi-token support for rotation, principal context injection.

## Goal

Define the first production-ready authentication slice for `castaway-web`: trusted service-to-service access from `castaway-discord-bot` to the API.

## Why this exists

`castaway-web` currently has no production authentication model. For the first self-hosted deployment, the Discord bot is the primary expected client, so the simplest safe step is to authenticate bot-to-API traffic explicitly.

This plan is intentionally narrower than the broader auth and authorization thread. Human-facing auth, end-user sessions, and future write authorization can build on top of this later.

## Recommended v1 model

### Authentication shape

Use a shared bearer-token model first.

Behavior:

- `castaway-discord-bot` sends an authorization token with every API request
- `castaway-web` validates that token before serving non-health endpoints
- `/healthz` remains unauthenticated for cluster health checks

### Why this is the first choice

Pros:

- simple to implement and operate
- easy to provision through Kubernetes secrets
- easy to rotate with a clear runbook
- enough for a single trusted bot client in a private self-hosted deployment

Non-goals for v1:

- mTLS
- OAuth
- end-user login flows
- fine-grained per-user authorization

## Proposed contract

### API behavior

When service auth is enabled:

- requests without a valid bearer token receive `401 Unauthorized`
- requests with an invalid token receive `401 Unauthorized`
- requests with a valid token are tagged internally as an authenticated service principal

### Route policy

First production policy:

- require auth on all API routes except `/healthz`

This keeps the initial surface area simple and avoids accidentally leaving useful routes open while the API is still primarily bot-facing.

### Principal model

Start with one named service principal:

- `castaway-discord-bot`

Even if the first implementation only checks token validity, the internal design should preserve the idea of a caller identity so future authorization rules have something concrete to key on.

## Secret and rotation expectations

### Source of truth

- 1Password `bry-guy` vault remains the source of truth
- Kubernetes receives the materialized secret through infra-managed unattended access

### Rotation shape

The API should support safe token rotation.

Recommended behavior:

- accept a small set of active tokens during rotation
- remove the old token after the bot has been updated and restarted

That avoids a brittle all-at-once deploy dependency between the bot and the API.

## Configuration expectations

The exact env var names can be finalized during implementation, but the design should support:

- local development with auth disabled by default
- production with service auth enabled explicitly
- one or more active bearer tokens loaded from secrets

## Authorization boundary

This plan only covers authentication of the bot as a service caller.

It does not replace the broader need to decide:

- what future human-facing access should look like
- what future write endpoints require stronger authorization rules
- whether some read-only routes should later be public

## Implementation phases

### Phase 1: API-side contract

- define the service-auth configuration model
- add auth middleware to `castaway-web`
- exempt `/healthz`
- attach authenticated principal info to request context

### Phase 2: coverage and tests

- add HTTP tests for missing, invalid, and valid credentials
- verify route coverage so protected routes do not accidentally bypass middleware

### Phase 3: bot integration

- update `castaway-discord-bot` to send the bearer token on all API requests
- document the shared rollout and rotation procedure

### Phase 4: operational hardening

- add startup validation for required auth config in production mode
- document token rotation
- verify logs do not leak auth headers or token values

## Resolved questions

- should the first implementation accept exactly one token or a small token set for easier rotation? → **Resolved: multi-token set via CSV env var `SERVICE_AUTH_BEARER_TOKENS`**
- does the API need to distinguish service principals before human auth exists, or is one principal enough initially? → **Resolved: single principal (`castaway-discord-bot`) with context injection for future expansion**

## Open questions

- should the production API remain bot-only for a while, or should any human-facing read-only access be planned now?

## Related threads

- `auth-and-authorization-planning.md`
- `../../non-functional-requirements.md`
- `../../production-readiness-checklist.md`
