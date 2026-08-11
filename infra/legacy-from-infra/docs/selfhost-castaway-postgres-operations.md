# Castaway external PostgreSQL operations

This document describes the preferred stateful-database path for Castaway on the shared selfhost platform.

It covers:

- the external PostgreSQL host model on the shared stateful VM
- bootstrap of the PostgreSQL service and logical databases
- automated logical backups with retention
- restore guidance

## Target architecture

Preferred shape:

- shared **appliance VM** runs k3s server/control-plane duties
- shared **service VM** runs stateless Castaway workloads
- shared **stateful VM** runs external PostgreSQL and backup automation

This keeps PostgreSQL outside Kubernetes while still letting the apps consume normal connection URLs from Kubernetes `Secret`s.

## Database model

`castaway-web` and `castaway-discord-bot` share one PostgreSQL server process, but use separate:

- logical databases
- roles/users
- credentials

Expected databases:

- `castaway_web`
- `castaway_discord_bot`

Expected users:

- `castaway_web`
- `castaway_discord_bot`

Expected superuser:

- default `postgres` unless intentionally overridden

## Operator tasks

From `~/dev/infra`:

### Bootstrap PostgreSQL on the shared stateful VM

```bash
mise run "selfhost:castaway:postgres:bootstrap"
```

What it does:

- connects to the shared stateful VM over SSH
- installs required runtime packages when the host supports `dnf`
- creates a root-owned PostgreSQL config directory
- writes a systemd-managed Podman service for PostgreSQL
- starts PostgreSQL
- ensures the expected Castaway roles and databases exist idempotently

Current implementation note:

- this first path uses a systemd service wrapping `podman run`
- it is intentionally simple and practical for the first external-Postgres rollout
- a future quadlet or more image-native path can replace it later without changing the logical database contract

### Install automated backup + retention on the shared stateful VM

```bash
mise run "selfhost:castaway:postgres:backup:install"
```

What it does:

- connects to the shared stateful VM over SSH
- writes a root-owned backup environment file
- installs a root-owned backup runner script
- installs a systemd oneshot service and daily timer
- enables the timer

### Trigger a manual backup immediately

```bash
mise run "selfhost:castaway:postgres:backup"
```

What it does:

- connects to the shared stateful VM over SSH
- creates fresh logical dumps for globals and both app databases
- pushes the backup into restic
- applies retention immediately

## Required configuration

### Non-secret config in `mise.toml`

Expected operator-facing values:

- `SELFHOST_CASTAWAY_STATEFUL_VM_HOST`
- `SELFHOST_CASTAWAY_STATEFUL_VM_SSH_USER`
- `SELFHOST_CASTAWAY_STATEFUL_VM_TAILNET_HOSTNAME`
- `SELFHOST_CASTAWAY_POSTGRES_HOST`
- `SELFHOST_CASTAWAY_POSTGRES_PORT`
- `SELFHOST_CASTAWAY_POSTGRES_IMAGE`
- `SELFHOST_CASTAWAY_POSTGRES_DATA_DIR`
- `SELFHOST_CASTAWAY_POSTGRES_CONFIG_DIR`
- `SELFHOST_CASTAWAY_POSTGRES_WEB_DB_NAME`
- `SELFHOST_CASTAWAY_POSTGRES_BOT_DB_NAME`
- `SELFHOST_CASTAWAY_POSTGRES_RESTIC_REPOSITORY`
- `SELFHOST_CASTAWAY_POSTGRES_BACKUP_KEEP_DAILY`
- `SELFHOST_CASTAWAY_POSTGRES_BACKUP_KEEP_WEEKLY`
- `SELFHOST_CASTAWAY_POSTGRES_BACKUP_KEEP_MONTHLY`
- optional `SELFHOST_CASTAWAY_POSTGRES_BACKUP_ON_CALENDAR`

### Secret inputs from `fnox.toml`

Required:

- `SELFHOST_CASTAWAY_VM_SSH_KEY/private key`
- `CASTAWAY_POSTGRES_SUPERUSER_PASSWORD/password`
- `CASTAWAY_WEB_DB_PASSWORD/password`
- `CASTAWAY_DISCORD_BOT_DB_PASSWORD/password`
- `SELFHOST_CASTAWAY_POSTGRES_RESTIC_PASSWORD/password`

Also reused for the restic S3 backend when applicable:

- `OCI_OBJECTSTORAGE_ACCESS_KEY_ID/password`
- `OCI_OBJECTSTORAGE_SECRET_ACCESS_KEY/password`

## Backup format and retention

Current backup strategy is intentionally simple:

- logical dump of global roles/metadata
- logical dump of `castaway_web`
- logical dump of `castaway_discord_bot`
- backup artifacts stored in a timestamped directory on the stateful VM
- restic snapshots pushed to the configured repository
- retention enforced by restic

Retention target:

- `7 daily`
- `4 weekly`
- `12 monthly`

This is a practical first production posture without introducing WAL archiving or point-in-time recovery complexity.

## What the backup job captures

Each run creates a timestamped directory containing:

- `00-globals.sql`
- `10-castaway_web.dump`
- `20-castaway_discord_bot.dump`
- `backup-manifest.txt`

Recommended interpretation:

- `00-globals.sql` restores roles and global metadata
- each `*.dump` file restores one logical application database

## Restore guidance

Restore should be treated as an operator-controlled event.

High-level restore sequence:

1. provision or confirm a target PostgreSQL host exists
2. stop app traffic or point workloads away from the old host as appropriate
3. retrieve the desired backup snapshot from restic
4. restore global roles/metadata first
5. recreate or reset the target databases as needed
6. restore `castaway_web`
7. restore `castaway_discord_bot`
8. validate application connectivity before resuming normal traffic

### Example restore outline

On the PostgreSQL host after retrieving a backup directory:

```bash
cat 00-globals.sql | psql -U postgres -d postgres

dropdb --if-exists castaway_web
createdb --owner castaway_web castaway_web
pg_restore -U postgres -d castaway_web 10-castaway_web.dump

dropdb --if-exists castaway_discord_bot
createdb --owner castaway_discord_bot castaway_discord_bot
pg_restore -U postgres -d castaway_discord_bot 20-castaway_discord_bot.dump
```

Then verify:

```bash
psql -U postgres -d castaway_web -c '\dt'
psql -U postgres -d castaway_discord_bot -c '\dt'
```

## Disaster recovery notes

### If the PostgreSQL container fails but the stateful VM and data directory survive

- fix or restart the systemd-managed PostgreSQL service
- data should remain intact in the mounted data directory

### If the stateful VM is lost but restic backups survive

- provision a replacement stateful VM
- rerun PostgreSQL bootstrap
- install backup tooling
- restore from the latest good snapshot
- rerun Kubernetes secret sync if the database host/URL changed

### If secrets rotate

After password rotation:

1. update the source secret values in 1Password
2. rerun PostgreSQL bootstrap so DB roles/passwords reconcile
3. rerun:

```bash
mise run "selfhost:castaway:secrets:sync"
```

## Future improvements

This first external-Postgres path intentionally does **not** implement:

- PITR / WAL archiving
- standby replicas
- fully image-native PostgreSQL host automation
- automated restore drills

Those can come later once the baseline stateful host flow is proven.
