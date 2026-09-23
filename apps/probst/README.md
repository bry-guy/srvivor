# Probst

Small HTTP-only Castaway operator client. No database access, scheduler, OAuth login, or season-operation commands.

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
probst channel bind CHANNEL --guild GUILD --instance INSTANCE
probst channel show CHANNEL --guild GUILD
probst channel unbind CHANNEL --guild GUILD --yes
probst player list --instance INSTANCE
probst player link PARTICIPANT --discord-user USER --instance INSTANCE
probst player unlink PARTICIPANT --instance INSTANCE --yes
probst draft show PARTICIPANT --instance INSTANCE
probst scores --instance INSTANCE
```

Use `--json` for machine-readable output. `--server` and `--actor` override configuration. A channel rebind requires `--yes` and API admin authorization over both instances. Player identity replacement requires explicit unlinking first; conflicts never silently transfer identities.

Bootstrap is disabled unless the API has `BOOTSTRAP_ADMIN_DISCORD_USER_ID` configured. The service-authenticated actor must match that ID, and the instance must have no admins. An already-authorized matching retry succeeds. This does not grant access to existing administered instances.

Player commands use API-owned guild/channel bindings; threads inherit a parent binding when they lack an explicit binding. Bindings do not validate Discord permissions—the bot verifies thread context when resolving inheritance.

## Validation

```sh
MISE_EXPERIMENTAL=1 mise run //apps/probst:ci
MISE_EXPERIMENTAL=1 mise run //apps/castaway-web:integration -- -run TestProbstChannelAndPlayerAdministration -count=1
```

The integration task uses a fresh disposable PostgreSQL container and invokes the built Probst binary against a local test API. It does not use production databases.

See [requirements](functional-requirements.md), [security requirements](non-functional-requirements.md), [readiness](production-readiness-checklist.md), [plan](plans/player-administration.md), and [changelog](CHANGELOG.md).
