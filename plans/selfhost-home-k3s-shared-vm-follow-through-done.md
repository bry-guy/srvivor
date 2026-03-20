# Self-hosted home-k3s shared-VM follow-through

Status: `done`

## Goal

Adapt the Castaway app repo's `home-k3s` deployment contract to the updated shared-VM infra target:

- shared appliance / service / stateful VMs in `~/dev/infra`
- k3s running on appliance + service nodes
- PostgreSQL hosted outside the Castaway overlay on the shared stateful VM
- automatic delivery remaining `GitHub Actions -> GHCR -> Argo CD`

## What changed

The app repo now treats:

- `deploy/environments/home-k3s`

as the single active selfhost overlay for the preferred external-PostgreSQL target.

Completed app-repo follow-through:

- removed in-cluster PostgreSQL manifests from the active selfhost overlay
- removed app-repo-owned PostgreSQL base manifests for the active target
- preserved web, bot, ingress, configmap, secret-reference, and migration-hook behavior
- added explicit service-node placement patches for web, migration job, and bot
- kept Argo CD pointed at `deploy/environments/home-k3s`
- kept digest pinning in Git and updated automation to stay aligned with the active overlay
- updated selfhost docs and implementation guidance to reflect the external-PostgreSQL/shared-VM contract

## Final contract

For `home-k3s`:

- Argo CD deploys app workloads only
- infra owns PostgreSQL hosting, lifecycle, and backups
- `castaway-web` reads `DATABASE_URL` from `castaway-web-secrets`
- `castaway-discord-bot` reads `BOT_STATE_DATABASE_URL` from `castaway-discord-bot-secrets`
- both apps point at the same PostgreSQL server with separate DB/users
- stateless app workloads target nodes labeled `selfhost.bry-guy.net/role=service`

## Verification checklist

- [x] active `home-k3s` overlay excludes in-cluster PostgreSQL resources
- [x] web deployment still references `castaway-web-secrets`
- [x] bot deployment still references `castaway-discord-bot-secrets`
- [x] migration Job remains a `PreSync` hook
- [x] scheduling rules target the service node label cleanly
- [x] docs reflect the new app/infra split
- [x] delivery automation stays aligned with `deploy/environments/home-k3s`

## Notes

Nothing had been deployed yet, so this follow-through was implemented by simplifying the repo to one active selfhost overlay instead of introducing parallel selfhost variants.
