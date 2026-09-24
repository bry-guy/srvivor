# Probst

Small HTTP-only Castaway operator client for season setup and draft loading. No database access, scheduler, or OAuth login.

## Setup

Build from the repository root:

```sh
MISE_EXPERIMENTAL=1 mise run //apps/probst:build
apps/probst/bin/probst --help
```

Configure `PROBST_API_URL`, `PROBST_DISCORD_USER_ID` (admin actor), and inject `PROBST_TOKEN` through your credential provider. Never print the token. HTTPS is required except on loopback. Redirects are rejected. `auth status` verifies service access and reports the asserted actor; it does not authenticate a human or prove admin access to every instance.

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

1. `probst instance create` (legacy mode) with a contestant file, one name per line. Write nicknames in quotes (`Danny "Kilby" Kilby`); first names, surnames, and nicknames all become draft aliases.
2. `probst instance bootstrap-admin INSTANCE`, then `probst channel bind`.
3. `probst player add NAME --discord-user USER` for each player.

## Loading drafts

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

See [requirements](functional-requirements.md), [security requirements](non-functional-requirements.md), [readiness](production-readiness-checklist.md), [plan](plans/player-administration.md), and [changelog](CHANGELOG.md).
