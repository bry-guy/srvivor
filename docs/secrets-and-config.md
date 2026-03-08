# Secrets and config

This monorepo uses a shared 1Password vault named `castaway` for secret material and `fnox` for injecting those secrets into local commands.

## Principles

- **Private values live in 1Password and are exposed to apps through fnox.**
- **Non-secret defaults belong in `mise.toml` env blocks or app config defaults.**
- **Each app selects only the fnox profile it needs.**
- **Do not inline secret environment variables before `fnox exec` when a fnox profile already defines them.**

## Shared provider

The repository root `fnox.toml` defines a shared 1Password provider:

- provider name: `op`
- provider type: `1password`
- vault: `castaway`

`fnox` resolves secrets through the 1Password CLI (`op`). For local development, use one of these approaches:

- sign in interactively with `op signin`, or
- provide `OP_SERVICE_ACCOUNT_TOKEN` / `FNOX_OP_SERVICE_ACCOUNT_TOKEN`

Depending on how your local 1Password setup is authenticated, some `op` subcommands may behave differently for interactive accounts vs service-account access. The repo-standard smoke test is therefore `fnox check` + `fnox exec`, not raw `op` output.

If you need to debug 1Password authentication specifically, `fnox provider test op` can be useful once your `op` session or service account token is configured. In practice, the most reliable day-to-day validation is `fnox check` plus a redacted `fnox exec` smoke test.

## App profiles

Apps should select their own profile through `mise.toml` environment configuration.

### `castaway-discord-bot`

The Discord bot uses the `castaway-discord-bot` profile. It maps the following 1Password items into environment variables:

- `CASTAWAY_DISCORD_BOT_TOKEN`
- `CASTAWAY_DISCORD_APPLICATION_ID`
- `DISCORD_BRAINLAND_SERVER_ID`
- `CASTAWAY_DISCORD_PUBLIC_KEY`

Only the first three are currently consumed by the gateway-based bot. The public key is loaded and documented now for future Discord signature-verification workflows.

Validate the profile without printing secret values:

```bash
fnox check -P castaway-discord-bot
fnox exec -P castaway-discord-bot -- env \
  | rg '^(CASTAWAY_DISCORD_BOT_TOKEN|CASTAWAY_DISCORD_APPLICATION_ID|DISCORD_BRAINLAND_SERVER_ID|CASTAWAY_DISCORD_PUBLIC_KEY)=' \
  | sed 's/=.*$/=<redacted>/'
```

If you specifically want to test the provider handshake itself, you can also try:

```bash
fnox provider test op
```

## Public config via mise

Non-secret runtime defaults should live in app-level `mise.toml` env sections. For `apps/castaway-discord-bot`, that includes:

- `FNOX_PROFILE=castaway-discord-bot`
- `CASTAWAY_API_BASE_URL=http://localhost:8080`
- `BOT_STATE_PATH=./data/state.db`
- `LOG_LEVEL=INFO`

That means local commands can stay simple:

```bash
mise run //apps/castaway-discord-bot:check-config
mise run //apps/castaway-discord-bot:run
```

The root stack tasks also use the same fnox profile when starting the Docker Compose bot service:

```bash
mise run start
mise run bot
```

## Adding secrets for another app

When another app needs secrets:

1. add or confirm the secret exists in the shared `castaway` vault
2. add a new fnox profile in the repo root `fnox.toml`
3. map only the environment variables that app needs
4. set `FNOX_PROFILE` in that app's `mise.toml`
5. keep non-secret defaults in `mise.toml`

This keeps secret selection explicit per app while still sharing one monorepo-wide vault.
