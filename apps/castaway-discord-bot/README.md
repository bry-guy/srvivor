# castaway-discord-bot

`castaway-discord-bot` is a standalone Discord bot that queries `castaway-web` and presents Survivor draft data through slash commands.

## Commands

Top-level command: `/castaway`

### Query commands
- `/castaway score participant:<name> [instance] [season]`
- `/castaway scores [instance] [season]`
- `/castaway draft participant:<name> [instance] [season]`

### Context commands
- `/castaway instance list [season]`
- `/castaway instance set instance:<name> [season] [scope:me|guild]`
- `/castaway instance show`
- `/castaway instance clear [scope:me|guild]`

## Local development

Start and seed the API stack from the repo root:

```bash
mise run start
mise run seed
```

Provide bot configuration via fnox-backed environment variables or your shell environment:

- `DISCORD_BOT_TOKEN`
- `DISCORD_APPLICATION_ID`
- `DISCORD_DEV_GUILD_ID`
- `CASTAWAY_API_BASE_URL` (default: `http://localhost:8080`)
- `BOT_STATE_PATH` (default: `./data/state.db`)
- `LOG_LEVEL` (default: `INFO`)

Run the bot locally:

```bash
fnox exec -- mise run //apps/castaway-discord-bot:run
```

Validate config and local writable state without connecting to Discord:

```bash
fnox exec -- mise run //apps/castaway-discord-bot:check-config
```

## Discord setup notes

- Use a dedicated development guild and set `DISCORD_DEV_GUILD_ID` so slash command updates register quickly.
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
```

## Notes

- Guild default instance changes require Discord Manage Server permissions.
- Bot instance defaults are currently stored in a local file-backed state store. This is fine for local and single-instance use, but must be revisited before multi-replica production deployment.
