# Security Audit: Web and Discord Identity Model

This checklist identifies security gaps in the boundary between `castaway-web` (HTTP API) and `castaway-discord-bot` (Discord slash-command interface), with a focus on how user identity flows — or fails to flow — between the two systems.

## Identity model gaps

### No web-side user identity
- [ ] The web API has no concept of individual users — all authenticated requests resolve to a single service principal (`castaway-discord-bot`)
- [ ] `participants` are name-only records with no link to an authenticated identity (Discord or otherwise)
- [ ] No audit trail exists for who created, modified, or deleted resources through the API
- [ ] Any holder of a valid bearer token can modify any participant's draft, outcomes, or instance data

### Discord user context lost at the API boundary
- [ ] The Discord bot extracts `interaction.Member.User.ID` but never forwards it to the web API
- [ ] The web API has no header, claim, or parameter for the originating Discord user
- [ ] Write operations (draft updates, outcome changes, participant creation) cannot be attributed to a specific Discord user on the API side
- [ ] Read operations cannot be scoped per-user — the bot can query any instance or participant regardless of who asked

### Participant identity confusion
- [ ] A `participant.name` is just a string — two Discord users named "Bryan" collide on `UNIQUE(instance_id, name)`
- [ ] No mechanism maps a Discord user ID to a participant record, so the bot cannot enforce "you may only edit your own draft"
- [ ] Participant creation is open to any authenticated caller — the bot could create participants on behalf of users who never consented

## Authorization gaps

### Web API: no resource-level authorization
- [ ] All protected routes use the same binary auth check (valid token → full access)
- [ ] No ownership model: any authenticated request can PUT to any participant's draft
- [ ] No ownership model: any authenticated request can PUT to any instance's outcomes
- [ ] No read restrictions: all instances, participants, and leaderboards are visible to any authenticated caller
- [ ] No rate limiting on write endpoints

### Discord bot: guild-scoped authorization only
- [ ] Guild admin checks (`hasGuildManagePermission`) only gate guild-default commands — not data-mutating API calls
- [ ] Any Discord user in a guild can trigger bot commands that write to the web API
- [ ] No per-user command allowlist or role-based restriction beyond Discord's built-in guild permissions
- [ ] DM-initiated commands bypass guild permission checks entirely (bot supports DMs via `interaction.User`)

## Authentication gaps

### Static bearer token risks
- [ ] Single static token shared between bot and API — compromise of either side exposes the other
- [ ] Token comparison uses map lookup, not `crypto/subtle.ConstantTimeCompare` — timing side-channel possible
- [ ] No token expiration or TTL enforcement
- [ ] No token rotation automation — rotation is manual and undocumented at the operational level
- [ ] `SERVICE_AUTH_ENABLED` defaults to `false` — forgetting to set it leaves the API open

### Direct API access bypasses Discord authorization
- [ ] Anyone with the bearer token can call the API directly, bypassing all Discord permission checks
- [ ] No IP allowlisting or network-level restriction between bot and API (relies on private network assumption)
- [ ] No mutual TLS or client certificate to distinguish the bot from other callers
- [ ] Multiple bots or scripts sharing the same token are indistinguishable

## Logging and observability gaps

### Insufficient audit surface
- [ ] Structured logging is not implemented — both apps use `log.Printf`
- [ ] No request logging middleware that captures caller identity, action, and target resource
- [ ] No correlation ID between a Discord interaction and the resulting API call(s)
- [ ] Auth failures are returned as `401` with `{"error": "unauthorized"}` but not logged with caller context

### Secret leakage risk
- [ ] Bearer token is injected via environment variable `SERVICE_AUTH_BEARER_TOKENS` — visible in process listings, container inspect, and debug dumps
- [ ] No log-scrubbing verification exists in CI or tests
- [ ] Bot state database URL in `BOT_STATE_DATABASE_URL` may contain credentials — same exposure risk

## Cross-system integrity gaps

### No transactional consistency
- [ ] The bot makes multiple sequential API calls (e.g., resolve instance, then resolve participant, then fetch leaderboard) with no transactional guarantee
- [ ] A race between two Discord users issuing commands could produce inconsistent state
- [ ] No optimistic concurrency (etag, version column) on mutable resources like drafts or outcomes

### State divergence
- [ ] The bot maintains its own state (guild/user defaults) in a separate database with no foreign-key relationship to web API data
- [ ] If an instance is deleted in the web API, the bot's saved defaults still reference a stale instance UUID
- [ ] No reconciliation or cleanup mechanism for orphaned bot-side references

## Recommended mitigations (priority order)

1. **Forward Discord user context to the API** — add a `X-Castaway-Actor` header (or equivalent) carrying the Discord user ID on every bot-to-API request, and log it server-side
2. **Add timing-safe token comparison** — replace map lookup with `crypto/subtle.ConstantTimeCompare`
3. **Implement structured logging** — adopt `log/slog` in both apps with request-scoped fields
4. **Add request audit logging middleware** — log method, path, principal, actor, and response status on every request
5. **Link participants to Discord user IDs** — add an optional `external_id` column to `participants` to enable future ownership enforcement
6. **Default `SERVICE_AUTH_ENABLED` to `true`** — require explicit opt-out for local dev rather than opt-in for production
7. **Add resource-scoped authorization** — start with write endpoints (draft PUT, outcome PUT) requiring the caller to prove ownership or admin status
8. **Add correlation IDs** — generate a request ID in the bot, forward it as a header, and log it on both sides
