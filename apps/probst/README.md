# Probst

Small HTTP-only Castaway operator client for season setup and draft loading. No database access, scheduler, or OAuth login.

## Setup

Build from the repository root:

```sh
MISE_EXPERIMENTAL=1 mise run //apps/probst:build
apps/probst/bin/probst --help
```

For one-time local setup, create `~/.config/probst/config.json` with owner-only permissions (`mkdir -p ~/.config/probst && chmod 700 ~/.config/probst`; create the file with `umask 077` and verify `chmod 600 ~/.config/probst/config.json`). Do not commit the file or print its credentials:

```json
{
  "api_url": "https://castaway.bry-guy.net",
  "discord_user_id": "YOUR_DISCORD_USER_ID",
  "token": "YOUR_CASTAWAY_SERVICE_TOKEN",
  "discord_bot_token": "YOUR_DISCORD_BOT_TOKEN"
}
```

`https://castaway.bry-guy.net` is the tailnet HTTPS endpoint for the Castaway web API; routine Probst calls no longer need a port-forward. Probst does not fetch Kubernetes secrets or configure Tailscale for you. Obtain credentials through your approved secret provider; the JSON file stores them as plaintext on your machine, so prefer environment injection if that is unsuitable. Missing config files preserve the environment-only workflow. Explicit flags (`--server`, `--actor`) override environment variables, which override config fields; an explicitly empty environment value also overrides the file. HTTPS is required except on loopback. Redirects are rejected. `auth status` verifies service access and reports the asserted actor; it does not authenticate a human or prove admin access to every instance.

## Commands

```sh
probst auth status
probst instance list
probst instance show INSTANCE
probst instance bootstrap-admin INSTANCE
probst instance create --name NAME --season N --contestants-file seasons/51-contestants.txt
probst contestant list --instance INSTANCE
probst channel bind CHANNEL --guild GUILD --instance INSTANCE
probst channel show CHANNEL --guild GUILD
probst channel unbind CHANNEL --guild GUILD --yes
probst player list --instance INSTANCE
probst player add NAME [--discord-user USER] --instance INSTANCE
probst player link PARTICIPANT --discord-user USER --instance INSTANCE
probst player unlink PARTICIPANT --instance INSTANCE --yes
probst draft show PARTICIPANT --instance INSTANCE
probst draft import THREAD_URL --instance INSTANCE [--before RFC3339] [-v] [--yes]
probst draft import --file FILE --participant PLAYER --instance INSTANCE [--yes]
probst scores --instance INSTANCE
```

Use `--json` for machine-readable output. `--server` and `--actor` override configuration. A channel rebind requires `--yes` and API admin authorization over both instances. Player identity replacement requires explicit unlinking first; conflicts never silently transfer identities.

Bootstrap is disabled unless the API has `BOOTSTRAP_ADMIN_DISCORD_USER_ID` configured. The service-authenticated actor must match that ID, and the instance must have no admins. An already-authorized matching retry succeeds. This does not grant access to existing administered instances.

Player commands use API-owned guild/channel bindings; threads inherit a parent binding when they lack an explicit binding. Bindings do not validate Discord permissions—the bot verifies thread context when resolving inheritance.

## Season setup

These steps use Probst against the API. Check for an existing instance before `instance create`: repeating it creates another instance with the same name and season. Bootstrap retries for the same authorized admin and unchanged draft imports are safe; `player add` reuses a same-name player on a sequential retry but is not concurrency-safe idempotency.

```sh
# Production access (operator machine on the tailnet with the selfhost kubeconfig):
export PROBST_API_URL=https://castaway.bry-guy.net PROBST_DISCORD_USER_ID=235246238382030849
export PROBST_TOKEN="$(kubectl get secret -n castaway castaway-web-secrets \
  -o jsonpath='{.data.SERVICE_AUTH_BEARER_TOKENS}' | base64 -d | cut -d, -f1)"

probst instance create --name "Season 51 (BrainLand)" --season 51 --contestants-file seasons/51-contestants.txt
probst instance bootstrap-admin INSTANCE              # makes PROBST_DISCORD_USER_ID the first admin
probst channel bind CHANNEL --guild GUILD --instance INSTANCE
probst player add NAME --discord-user USER --instance INSTANCE   # reuses a same-name player
probst draft import THREAD_URL --instance INSTANCE --before CUTOFF   # review, then add --yes
```

Contestant files list one name per line; write nicknames in quotes (`Danny "Kilby" Kilby`). First names, surnames, and nicknames all become draft aliases.

Limits: `bootstrap-admin` works only on an instance with no admins and only for the API's configured `BOOTSTRAP_ADMIN_DISCORD_USER_ID`. There is no command to add a second admin.

## Loading drafts

After one-time configuration and private HTTPS routing, the usual dry run is `probst draft import THREAD_URL --instance INSTANCE --verbose`. Review it, then repeat with `--yes` to submit only READY drafts. Keep `--instance` explicit to avoid writing to the wrong season.

`draft import` is a dry run unless `--yes`. It prints one row per player: `READY`, `UNCHANGED`, `NEEDS REVIEW` (with reasons), `UNLINKED` (a Discord author with no player link), or `NO DRAFT`. `-v` shows how every line matched. `--yes` writes only `READY` drafts; everything else needs a repost or a `--file` import.

Thread reading needs `CASTAWAY_DISCORD_BOT_TOKEN` (fnox profile `castaway-discord-bot`) and the bot's Message Content intent. Rules:

- A message is a draft only if at least 15 of its lines name contestants; chat is ignored.
- Each author's latest draft before `--before` wins; later drafts and edits after the cutoff are reported.
- Numbered lines rank by number (any of `1.`, `1)`, `1 -`, `1:`); otherwise lines rank top to bottom. Comma lists work.
- Names match exactly on full name, nickname, first name, or surname, or fuzzily when the winner is clear. Close calls (for example `An`: Ana or Thien An) are reported, never guessed.
- The API still rejects any draft that is not every contestant exactly once.

## Validation

```sh
MISE_EXPERIMENTAL=1 mise run //apps/probst:ci
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:integration -- -run TestProbstChannelAndPlayerAdministration -count=1
```

The integration task uses a fresh disposable PostgreSQL container and invokes the built Probst binary against a local test API. It does not use production databases.

See [requirements](functional-requirements.md), [security requirements](non-functional-requirements.md), [readiness](production-readiness-checklist.md), [administration plan](plans/player-administration.md), [private-endpoint proposal](plans/private-api-endpoint.md), and [changelog](CHANGELOG.md).
