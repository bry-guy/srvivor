# castaway-web Non-Functional Requirements

## Security

- Production deployments must define and enforce an authentication model before public exposure.
- Secrets must be supplied through managed environment injection and must never be committed.
- Logs must avoid leaking secrets, tokens, or sensitive request data.

## Reliability

- The app must fail fast on invalid configuration.
- Database migrations must be applied consistently before serving traffic.
- Seed workflows must remain repeatable for local development.

## Observability

- The app must expose a health check.
- Structured logs should be emitted for startup, request failures, and database failures.
- API changes must stay synchronized with TypeSpec/OpenAPI and route registration tests.

## Performance

- API filters should remain available where bot workflows depend on bounded lookups.
- Leaderboard and draft lookups should remain efficient for current season-scale data sizes.

## Operations

- The local development workflow must stay documented and reproducible through `mise`.
- Database backup, restore, and rollback procedures are required before production use.
