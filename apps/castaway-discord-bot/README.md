# castaway-discord-bot

`castaway-discord-bot` is a standalone Discord bot that queries `castaway-web` and presents Survivor draft data through slash commands.

## Documentation

- Changelog: `CHANGELOG.md`
- Functional requirements: `functional-requirements.md`
- Non-functional requirements: `non-functional-requirements.md`
- Production readiness: `production-readiness-checklist.md`
- Shared app blueprint: `../../docs/castaway-discord-bot-blueprint.md`

## Commands

Top-level command: `/castaway`

### Query commands
- `/castaway score participant:<name> [instance] [season]`
- `/castaway scores [instance] [season]`
- `/castaway draft participant:<name> [instance] [season]`
- `/castaway activities [instance] [season]`
- `/castaway activity activity:<name> [instance] [season]`
- `/castaway occurrences activity:<name> [instance] [season]`
- `/castaway occurrence activity:<name> occurrence:<name> [instance] [season]`
- `/castaway history participant:<name> [instance] [season]`

### Context commands
- `/castaway instance list [season]`
- `/castaway instance set instance:<name> [season] [scope:me|guild]`
- `/castaway instance show`
- `/castaway instance clear [scope:me|guild]`

## Local development

Start and seed the full local stack from the repo root:

```bash
mise run start
mise run seed
```

That starts:
- `castawaydb`
- `castaway-web`
- `castaway-discord-bot`

### Secret setup

This app uses the shared monorepo 1Password vault, `castaway`, through the root `fnox.toml`.

The app's `mise.toml` sets `FNOX_PROFILE=castaway-discord-bot`, so you should not need to inline secret env vars before `fnox exec` or `mise run`.

Secret env vars used by the bot:

- `CASTAWAY_DISCORD_BOT_TOKEN`
- `CASTAWAY_DISCORD_APPLICATION_ID`
- `DISCORD_TARGET_SEVER_ID`
- `CASTAWAY_DISCORD_PUBLIC_KEY` (loaded now for future Discord signature verification work; not currently consumed by the gateway bot)

Make sure `fnox` can access 1Password through `op` by doing one of the following:

- run `op signin`, or
- export `OP_SERVICE_ACCOUNT_TOKEN` / `FNOX_OP_SERVICE_ACCOUNT_TOKEN`

Validate access from the repo root:

```bash
fnox check -P castaway-discord-bot
fnox exec -P castaway-discord-bot -- env \
  | rg '^(CASTAWAY_DISCORD_BOT_TOKEN|CASTAWAY_DISCORD_APPLICATION_ID|DISCORD_TARGET_SEVER_ID|CASTAWAY_DISCORD_PUBLIC_KEY)=' \
  | sed 's/=.*$/=<redacted>/'
```

### Public config

Non-secret defaults are provided through `apps/castaway-discord-bot/mise.toml`:

- `CASTAWAY_API_BASE_URL=http://localhost:8080`
- `BOT_STATE_BACKEND=bolt`
- `BOT_STATE_PATH=./data/state.db`
- `LOG_LEVEL=INFO`

Optional production-oriented config:

- `CASTAWAY_API_AUTH_TOKEN` for bot-to-API bearer authentication
- `BOT_STATE_DATABASE_URL` when `BOT_STATE_BACKEND=postgres`

Override them in your shell only when you need a non-default local setup.

### Run the bot locally

The default local workflow is to let the root stack manage the bot lifecycle:

```bash
mise run start
mise run bot-logs
mise run stop
```

If you want to run only the bot service, use:

```bash
mise run bot
mise run bot-logs
```

If you want to run the process directly on the host instead of through Docker Compose, you can still use:

```bash
mise run //apps/castaway-discord-bot:run
```

Validate config and state wiring without connecting to Discord:

```bash
mise run //apps/castaway-discord-bot:check-config
```

If you are migrating saved defaults from BoltDB to PostgreSQL, set `BOT_STATE_BACKEND=postgres`, set `BOT_STATE_DATABASE_URL`, and run:

```bash
BOLT_STATE_IMPORT_PATH=./data/state.db mise run import-bolt-state
```

## Discord setup notes

- Use a dedicated target guild; the bot reads its guild ID from `DISCORD_TARGET_SEVER_ID`, which can be sourced from whichever 1Password item matches the environment.
- Invite the bot with both the `bot` and `applications.commands` scopes.
- The bot only needs guild slash command support for the MVP; it does not require privileged message content intent.

## App tasks

```bash
cd apps/castaway-discord-bot
mise run lint
mise run test
mise run build
mise run run
mise run check-config
mise run import-bolt-state
./bin/castaway-discord-bot --version
```

If the bot cannot resolve secrets, `mise run run` and `mise run check-config` will fail fast via `fnox check` before starting the process.

## Notes

- Guild default instance changes require Discord Manage Server permissions.
- Production bot-to-API traffic can be authenticated with `CASTAWAY_API_AUTH_TOKEN` as a bearer token.
- The bot supports both `bolt` and `postgres` state backends. PostgreSQL is the intended production direction and should use its own logical database, `castaway_discord_bot`, with separate credentials from `castaway-web`.
- BoltDB remains a valid local and compatibility backend, and `import-bolt-state` provides an explicit migration path into PostgreSQL.
