# Castaway

Castaway is the monorepo for Survivor fantasy draft tooling.

## Status

This repo is in transition from a single CLI project into a broader multi-app workspace.

- `apps/cli` contains the original `srvivor` CLI.
- `apps/castaway-web` contains the persistent Gin + PostgreSQL web API.
- `apps/castaway-discord-bot` will host the Discord bot integration that queries `castaway-web`.
- The `srvivor` command and behavior are intentionally preserved for backwards compatibility.
- New work should be organized as additional apps/packages under this monorepo.

## Development

This repository uses [mise](https://mise.jdx.dev/) for task and tool management and [fnox](https://github.com/jdx/fnox) for local secret injection.

```bash
mise install
mise run ci
```

The monorepo shares a single 1Password vault, `castaway`, through the root `fnox.toml`. Each app selects only the fnox profile it needs through `mise` configuration, while non-secret defaults live in `mise.toml` env blocks.

### Web stack

```bash
mise run start
mise run seed
mise run ps
mise run logs
mise run stop
```

### Legacy CLI (`srvivor`)

```bash
cd apps/cli
mise run lint
mise run test
mise run build
mise run run
```

See `apps/cli/README.md` for CLI command usage.

### castaway-web

See `apps/castaway-web/README.md` for API + workflow details.

### castaway-discord-bot

See `apps/castaway-discord-bot/README.md` for local setup, commands, and Discord app configuration.

### Planning and production docs

- `docs/castaway-discord-bot-plan.md`
- `docs/non-functional-requirements.md`
- `docs/production-readiness-checklist.md`
- `docs/secrets-and-config.md`
