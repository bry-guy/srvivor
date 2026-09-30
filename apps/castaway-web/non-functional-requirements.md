# castaway-web Non-Functional Requirements

## Security

- Production deployments must define and enforce an authentication model before public exposure.
- Bot-to-API traffic should use bearer-token service authentication on all routes except `/healthz`.
- Admin mutations require both a valid service principal and transaction-bound instance-admin identity; bootstrap identity is explicitly configured and disabled by default.
- Secrets must be supplied through managed environment injection and must never be committed.
- Logs must avoid leaking secrets, tokens, or sensitive request data.
- Every content page requires Discord authentication; game APIs derive the player from that session and authorize membership in the game's instance.
- Live puzzle answers must not appear in unfinished-player responses, HTML, assets, or generic occurrence metadata. Cookie-authenticated mutations enforce same-origin checks.

## Reliability

- The app must fail fast on invalid configuration.
- Database migrations must be applied consistently before serving traffic.
- Production deployments must run migrations through a dedicated migration Job or equivalent pre-traffic hook rather than relying on app-startup auto-migration.
- Seed workflows must remain repeatable for local development.
- Weak draft-name suggestions must not save picks or trigger rewards; accepted revisions and message replay must preserve claim order without duplicating bonus, tribe, or announcement effects.
- Castawordle move updates must serialize per game/player; retries, lost acknowledgments, and multiple devices must preserve the accepted guess count. Trial games must not change scoring state.
- Game layouts must work at 320px, support touch/physical input, expose feedback beyond color, and respect theme/reduced-motion preferences.

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
