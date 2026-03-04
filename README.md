# Castaway

Castaway is the monorepo for Survivor fantasy draft tooling.

## Status

This repo is in transition from a single CLI project into a broader multi-app workspace.

- `apps/cli` contains the original `srvivor` CLI.
- The `srvivor` command and behavior are intentionally preserved for backwards compatibility.
- New work should be organized as additional apps/packages under this monorepo.

## Development

This repository uses [mise](https://mise.jdx.dev/) for task and tool management.

```bash
mise install
mise run ci
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
