# Admin CLI plan

Status: `planning`
Owner: castaway-admin (new app)
Last updated: 2026-04-16

## Problem

Operating Castaway currently requires a mix of:
- Discord slash commands with bespoke admin subcommands
- Direct SSH + psql into production Postgres
- Manual curl against the castaway-web API via kubectl port-forward
- Vibe coding with an AI agent to backfill, correct, and advance game state

This is fragile, slow, undiscoverable, and requires the admin to know internal implementation details. The admin cannot play as a regular player because operating the game exposes them to everyone's secrets.

## Goal

Build a dedicated Admin CLI that:
- talks to the castaway-web API over Tailscale
- replaces all Discord admin commands and manual DB surgery
- provides a complete admin view of game state
- supports all current gameplay operations
- is runnable from any machine on the tailnet
- is the single tool an admin needs to operate a season

## Non-goals for v1
- Replace player-facing Discord commands (players still use `/castaway` in Discord)
- Build a web admin UI
- Full season automation (see companion plan: `season-automation-planning.md`)

## Design

### App location

New Go binary at `apps/castaway-admin/` in the monorepo.

### Reuse the existing API client

The Discord bot already has a well-tested Go API client at `apps/castaway-discord-bot/internal/castaway/client.go`. The admin CLI should either:
- import it directly (preferred if the module structure allows), or
- extract it into a shared `pkg/castaway` package that both the bot and CLI import

The client already covers most read and write operations. New admin-only endpoints can be added to the web API and client as needed.

### Authentication

The CLI authenticates the same way the Discord bot does:
- `Authorization: Bearer <token>` for service auth
- `X-Discord-User-ID: <admin-discord-id>` for admin identity

Configuration via environment variables or a local config file:
```
CASTAWAY_API_BASE_URL=http://castaway-web.tailnet:8080
CASTAWAY_API_TOKEN=<bearer-token>
CASTAWAY_ADMIN_DISCORD_ID=<discord-user-id>
```

Or a `~/.config/castaway/admin.toml`:
```toml
api_url = "http://castaway-web.tailnet:8080"
api_token = "..."
admin_discord_id = "..."
default_instance = "9226f36f-1be7-4726-b9d8-3ee626a570de"
```

### Network access

Requires the castaway-web service to be reachable over Tailscale. Currently it's ClusterIP-only behind k3s. Options:
1. Add a Tailscale sidecar/proxy to the castaway-web pod (preferred)
2. Expose via a Tailscale funnel or k3s ingress
3. Keep using kubectl port-forward (acceptable for v1)

For v1, option 3 is fine. A follow-up can add Tailscale sidecar access.

### Command structure

```
castaway-admin [global flags] <command> [subcommand] [flags]
```

Global flags:
- `--instance <id-or-name>` (overrides default)
- `--api-url <url>` (overrides config)
- `--json` (raw JSON output instead of formatted tables)

## Command inventory

### Instance management

```
castaway-admin instance list
castaway-admin instance show [--instance]
castaway-admin instance set-default <instance-id-or-name>
```

### Game state (read-only)

```
castaway-admin leaderboard [--instance]
castaway-admin contestants [--instance]
castaway-admin participants [--instance]
castaway-admin outcomes [--instance]
castaway-admin episodes [--instance]
castaway-admin activities [--instance]
castaway-admin activity <activity-id>
castaway-admin occurrences <activity-id>
castaway-admin occurrence <occurrence-id>
castaway-admin ponies [--participant <name>] [--instance]
castaway-admin draft <participant-name> [--instance]
castaway-admin ledger <participant-name> [--instance]
castaway-admin history <participant-name> [--instance]
```

### Player management

```
castaway-admin link <participant-name> <discord-user-id> [--instance]
castaway-admin unlink <participant-name> [--instance]
castaway-admin link-all <csv-or-json-file> [--instance]
```

`link-all` bulk-links participants to Discord user IDs from a mapping file. This replaces having each player run `/castaway instance set` themselves.

### Episode management

```
castaway-admin episode current [--instance]
castaway-admin episode advance [--instance]
castaway-admin episode set-current <episode-number> [--instance]
```

`advance` moves the current episode forward by updating the air date of the next episode to now. `set-current` sets a specific episode as current. Both are needed for binge-watching old seasons or correcting schedule drift.

**New API needed:** `PUT /instances/:instanceID/episodes/:episodeNumber` to update episode air dates.

### Outcome recording

```
castaway-admin eliminate <contestant-name> [--position <N>] [--instance]
castaway-admin eliminate-batch <contestant1> <contestant2> ... [--instance]
```

`eliminate` records a contestant at the next open outcome position (or a specific position). `eliminate-batch` records multiple in order.

### Stir the Pot

```
castaway-admin pot status [--instance]
castaway-admin pot start [--name <name>] [--instance]
castaway-admin pot close [--instance]
castaway-admin pot contribute <participant-name> <points> [--instance]
castaway-admin pot show-tribe <tribe-name> [--instance]
```

### Individual pony auction (live lots)

```
castaway-admin auction status [--instance]
castaway-admin auction start-lot <contestant-name> [--instance]
castaway-admin auction stop-lot <contestant-name> [--instance]
castaway-admin auction bid <contestant-name> <participant-name> <points> [--instance]
```

### Merge auction (resolved import)

```
castaway-admin merge-auction record <results-json-or-file> [--raw-csv <file>] [--instance]
```

### Individual pony immunity + reward

```
castaway-admin pony immunity <contestant-name> [--instance]
castaway-admin pony reward <contestant-name> [--instance]
```

`reward` is a new operation. It awards +1 to the pony owner of the named contestant.

**New API needed:** `POST /instances/:instanceID/individual-pony/reward` — same pattern as the immunity endpoint but awards +1 instead of +3, with reason "X won individual reward".

### Loan shark

```
castaway-admin loan status <participant-name> [--instance]
castaway-admin loan borrow <participant-name> <points> [--instance]
castaway-admin loan repay <participant-name> <points> [--instance]
```

### Manual corrections

```
castaway-admin bonus award <participant-name> <points> <reason> [--visibility public|secret] [--instance]
castaway-admin bonus correct <participant-name> <points> <reason> [--visibility public|secret] [--instance]
```

These create manual bonus ledger entries for corrections, Monty Hall results, or other ad-hoc awards.

**New API needed:** `POST /instances/:instanceID/participants/:participantID/bonus-ledger/admin` — admin-only manual ledger entry creation.

### Bulk operations

```
castaway-admin import-season <season-config.yaml> [--instance]
```

This is a bridge to the season automation plan. It reads a YAML config that declares contestants, participants, episodes, activities, and draft picks, and creates them all via the API. Useful for setting up a new season quickly.

## New API endpoints needed

These are net-new endpoints in castaway-web required by the CLI but not yet implemented:

1. **`PUT /instances/:instanceID/episodes/:episodeNumber`** — update episode air date and metadata
2. **`POST /instances/:instanceID/individual-pony/reward`** — award +1 to pony owner of a contestant (same shape as immunity but +1)
3. **`POST /instances/:instanceID/participants/:participantID/bonus-ledger/admin`** — admin manual ledger entry creation
4. **`PUT /instances/:instanceID/outcomes/next`** — record next elimination without specifying position number (server picks next open slot)

These are small, focused additions that follow existing patterns.

## Implementation plan

### Phase 1: Scaffold + read commands
- Create `apps/castaway-admin/`
- Wire up config loading, API client, and auth
- Implement all read-only commands: `instance`, `leaderboard`, `contestants`, `participants`, `outcomes`, `episodes`, `activities`, `ponies`, `draft`, `ledger`
- Table formatting for terminal output + `--json` flag

### Phase 2: Core write commands
- `eliminate`, `eliminate-batch`
- `pot start`, `pot close`, `pot contribute`
- `pony immunity`
- `link`, `unlink`
- `episode advance`, `episode set-current`

### Phase 3: New API endpoints + CLI commands
- `pony reward` (new API)
- `bonus award`, `bonus correct` (new API)
- `episode` management (new API)
- `eliminate` with auto-position (new API)
- `merge-auction record`

### Phase 4: Bulk + quality of life
- `link-all` bulk linking
- `import-season` from YAML config
- Tab completion
- Instance name resolution (use names instead of UUIDs everywhere)
- Participant/contestant name fuzzy matching

## Testing plan

- Unit tests for CLI argument parsing, formatting, config loading
- Integration tests that spin up castaway-web and exercise CLI commands end-to-end
- Reuse existing verification seed data for integration scenarios

## Relationship to season automation

The Admin CLI is the **operator interface** for the game. Season automation (separate plan) builds on top of it by:
- defining seasons as declarative configs
- scheduling activities automatically based on episode air dates
- triggering resolutions when external events happen (e.g., episode airs)
- running scripted test seasons for deterministic validation

The CLI provides the primitives; automation composes them.
