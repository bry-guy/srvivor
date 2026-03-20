# Self-Hosted k3s Deployment Blueprint

Status: `planning`
Type: `blueprint`

## Goal

Define the Castaway deployment contract for the current self-hosted `home-k3s` target.

The preferred target now assumes:

- shared **appliance / service / stateful** VMs managed from `~/dev/infra`
- k3s running on the appliance and service nodes
- PostgreSQL running **outside** the Castaway k3s overlay on the shared stateful VM
- GitHub Actions publishing images to GHCR
- Argo CD reconciling the app repo into the cluster

This document is descriptive design/reference documentation. Execution sequencing lives in:

- `plans/selfhost-k3s-implementation-plan.md`

## Scope

This blueprint is for the app repo deployment contract only.

It covers:

- app Kubernetes manifests under `deploy/`
- Argo CD application wiring
- image publishing and digest pinning expectations
- runtime scheduling expectations for app workloads
- the app/infra boundary for secrets and database ownership

It does **not** cover:

- VM provisioning
- k3s bootstrap
- PostgreSQL installation on the shared stateful VM
- backup and restore automation
- Tailscale, LAN, or tunnel implementation details

Those belong in `~/dev/infra`.

## Locked-in decisions

### Delivery model

Use pull-based GitOps:

1. merge to `main`
2. GitHub Actions runs CI and publishes changed images to GHCR
3. GitHub Actions updates pinned digests in `deploy/environments/home-k3s/kustomization.yaml`
4. Argo CD syncs the `home-k3s` overlay into the cluster
5. the `castaway-web` migration Job runs before new web pods serve traffic
6. web and bot roll out onto the service node pool

### Overlay model

There is one active self-hosted deployment overlay in this repo:

- `deploy/environments/home-k3s`

That overlay is the external-PostgreSQL/shared-VM target.

This repo no longer treats in-cluster PostgreSQL as part of the active selfhost contract.

### Database model

For the current self-hosted target:

- PostgreSQL is external to the Castaway overlay
- infra owns PostgreSQL installation, lifecycle, and backups
- `castaway-web` consumes `DATABASE_URL` from `castaway-web-secrets`
- `castaway-discord-bot` consumes `BOT_STATE_DATABASE_URL` from `castaway-discord-bot-secrets`
- both apps point at the same PostgreSQL server
- each app uses its own logical database and credentials

Expected logical databases:

- `castaway_web`
- `castaway_discord_bot`

### Bot runtime model

- `castaway-discord-bot` remains single replica
- `BOT_STATE_BACKEND=postgres` remains the selfhost target contract
- bot and web continue using the existing service-auth contract

### Scheduling model

Stateless Castaway workloads should land on shared service nodes, not on the appliance/control-plane node by default.

Current node-label contract:

- `selfhost.bry-guy.net/role=service`

`castaway-web`, `castaway-web-migrate`, and `castaway-discord-bot` should target that label consistently.

## Repository layout

```text
deploy/
  base/
    castaway-web/
      deployment.yaml
      migration-job.yaml
      service.yaml
      kustomization.yaml

    castaway-discord-bot/
      deployment.yaml
      kustomization.yaml

  environments/
    home-k3s/
      namespace.yaml
      web-configmap.yaml
      web-patch.yaml
      web-placement-patch.yaml
      web-migration-hook-patch.yaml
      web-migration-placement-patch.yaml
      bot-configmap.yaml
      bot-patch.yaml
      bot-placement-patch.yaml
      ingress-web.yaml
      kustomization.yaml

  argocd/
    project-castaway.yaml
    app-home-k3s.yaml
```

## Workload responsibilities

### `deploy/base/castaway-web`

Responsible for:

- API deployment
- dedicated migration Job definition
- internal service
- health checks
- consuming config and secrets from Kubernetes

Runtime assumptions:

- `DATABASE_URL` comes from `castaway-web-secrets`
- startup auto-migration is disabled in deployed web pods
- production migrations run through the dedicated Job
- the migration Job remains an Argo CD `PreSync` hook via the overlay

### `deploy/base/castaway-discord-bot`

Responsible for:

- bot deployment only
- no ingress
- consuming bot config and secrets from Kubernetes

Runtime assumptions:

- one replica
- `BOT_STATE_BACKEND=postgres`
- `BOT_STATE_DATABASE_URL` comes from `castaway-discord-bot-secrets`
- internal API calls target the cluster-local `castaway-web` Service

### `deploy/environments/home-k3s`

Responsible for:

- namespace
- environment ConfigMaps
- ingress
- rollout/scheduling patches
- image digest pinning
- Argo CD migration hook annotations

Not responsible for:

- PostgreSQL StatefulSet/Service manifests
- PostgreSQL initialization scripts
- PostgreSQL storage and backup policy

## Secrets and configuration model

### Kubernetes-facing contract

The manifests in this repo assume these Kubernetes objects exist:

- `castaway-web-config`
- `castaway-web-secrets`
- `castaway-discord-bot-config`
- `castaway-discord-bot-secrets`

This repo should not rename those objects unless app and infra changes are coordinated together.

### Secret source of truth

1Password `bry-guy` remains the deployment secret source of truth.

The app repo references Kubernetes secret names only. The bridge from 1Password into Kubernetes belongs to infra.

## Delivery flow

### Image publishing

GitHub Actions should:

- build changed app images
- publish them to GHCR
- capture immutable digests
- update `deploy/environments/home-k3s/kustomization.yaml`

### Drift prevention

Delivery automation must stay aligned with the single active selfhost overlay.

That means:

- Argo CD points at `deploy/environments/home-k3s`
- digest-update automation writes to `deploy/environments/home-k3s/kustomization.yaml`
- CI validates that the `home-k3s` overlay still renders cleanly

If more overlays are introduced later, digest ownership must be refactored intentionally rather than allowing separate overlays to drift.

### Argo CD sync behavior

Argo CD should:

- watch `deploy/environments/home-k3s`
- auto-sync
- prune removed resources
- self-heal drift where appropriate
- run `castaway-web-migrate` before web rollout

## Access model

This deployment is private-first.

Expected shape:

- operator access through Tailscale
- private ingress through cluster networking / Traefik
- no direct public exposure of the home public IP
- any friendly hostname or outbound tunnel remains an infra concern

## Verification expectations

At minimum, the app repo should support validating:

- `deploy/environments/home-k3s` renders successfully
- migration hook annotations remain present
- web and bot secret references remain intact
- scheduling patches target the service-node label cleanly

## Compact summary

- `home-k3s` is now the external-PostgreSQL selfhost target.
- This repo deploys app workloads only.
- Infra owns PostgreSQL hosting and backups.
- Web and bot still deploy through Argo CD and still use pinned image digests in Git.
- `castaway-web-migrate` remains the pre-traffic migration path.
- Stateless workloads should land on nodes labeled `selfhost.bry-guy.net/role=service`.
