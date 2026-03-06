# castaway-discord-bot plan

## Goal

Add a standalone Discord bot app at `apps/castaway-discord-bot` that reads draft state from `castaway-web`, presents Discord-native slash command workflows, and stays easy to run locally during development.

## Decisions

- App name: `castaway-discord-bot`
- Language: Go
- Data source: `castaway-web` over HTTP
- No shared SDK yet; the bot will own a small internal Castaway client
- `castaway-web` remains the source of truth for scoring and draft state
- `castaway-discord-bot` owns Discord UX, formatting, and saved instance context
- Use existing repo tooling: `mise` for tasks and `fnox` for secrets/env injection
- Root `fnox.toml` points at the shared 1Password vault `castaway`; the bot selects its own fnox profile via `mise`

## Small castaway-web API enhancements

These are additive enhancements intended to simplify bot workflows without changing separation of concerns.

- `GET /instances`
  - optional `season`
  - optional `name`
- `GET /instances/:instanceID/participants`
  - optional `name`
- `GET /instances/:instanceID/leaderboard`
  - optional `participant_id`
- Keep TypeSpec/OpenAPI in sync with the server for these routes

## OpenAPI drift prevention

TypeSpec is the source of truth for the documented API.

Planned safeguards:

1. `mise run //apps/castaway-web:openapi-check`
   - regenerate OpenAPI from TypeSpec
   - fail if committed `openapi/openapi.yaml` is stale
2. Go route parity test
   - compare Gin routes to documented OpenAPI methods/paths
   - fail on undocumented routes, missing routes, or method mismatches
3. Keep documented public routes aligned with the server before merge

## Discord command surface

Top-level slash command: `/castaway`

### Query commands
- `/castaway score participant:<name> [instance] [season]`
- `/castaway scores [instance] [season]`
- `/castaway draft participant:<name> [instance] [season]`

### Context commands
- `/castaway instance list [season]`
- `/castaway instance set instance:<name> [season] [scope:me|guild]`
- `/castaway instance show`
- `/castaway instance clear [scope:me|guild]`

### Resolution order
When a command needs an instance:

1. explicit `instance`
2. user default within the current guild context
3. guild default
4. if a supplied season resolves to exactly one instance, use it
5. otherwise return an actionable ambiguity message

## Bot architecture

```text
apps/castaway-discord-bot/
  cmd/bot/main.go
  internal/config/config.go
  internal/discord/bot.go
  internal/discord/commands.go
  internal/discord/handlers.go
  internal/castaway/client.go
  internal/state/store.go
  internal/format/format.go
  mise.toml
  README.md
```

### Responsibility split
- `internal/castaway`: HTTP client for `castaway-web`
- `internal/state`: persistent defaults for guild/user instance context
- `internal/discord`: slash command registration and handlers
- `internal/format`: Discord-safe response formatting

## Local development

1. Start and seed the API stack:
   - `mise run start`
   - `mise run seed`
2. Ensure `op`/fnox can resolve the `castaway-discord-bot` profile from the shared `castaway` vault
3. Run the bot locally:
   - `mise run //apps/castaway-discord-bot:run`

Use guild-scoped command registration in development so command updates propagate quickly.

## Delivery plan

### Phase 1
- docs, non-functional requirements, and production checklist
- `castaway-web` API filters
- OpenAPI anti-drift checks

### Phase 2
- scaffold `apps/castaway-discord-bot`
- add config, Castaway client, persistent state store, command registration

### Phase 3
- implement MVP read-only slash commands
- add autocomplete and default-instance workflows
- wire monorepo CI and local-dev docs

## Out of scope for this MVP

- Shared Castaway SDK package
- Discord-driven write workflows
- Production authn/authz completion for `castaway-web`
- Multi-replica shared state for the bot
