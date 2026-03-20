# Self-Hosted k3s Deployment Blueprint

Status: `reference`
Type: `blueprint`

## Goal

Define the current Castaway self-hosted deployment contract for the home `k3s` target.

This document describes the app-repo side only. Infrastructure provisioning, VM lifecycle, PostgreSQL installation, backups, and secret materialization belong in `~/dev/infra`.

Execution history and follow-through planning live in:

- `plans/selfhost-k3s-implementation-plan.md`
- `docs/plans/selfhost-home-k3s-shared-vm-follow-through-plan.md`

## Current target shape

Castaway targets a shared-VM home lab layout:

- appliance/control-plane VM(s) for cluster control functions
- service VM node(s) for stateless Castaway workloads
- a shared stateful VM hosting PostgreSQL outside Kubernetes
- delivery path: `GitHub Actions -> GHCR -> Argo CD`

This repo owns Kubernetes manifests for the Castaway applications only.

## Locked decisions

### Delivery model

Use pull-based GitOps:

1. merge to `main`
2. GitHub Actions runs CI
3. changed app images are built and pushed to GHCR
4. image digests are pinned in `deploy/environments/home-k3s/kustomization.yaml`
5. Argo CD syncs that desired state into the cluster
6. the `castaway-web` migration Job runs as a `PreSync` hook
7. updated workloads roll out on the service node pool

### Database model

For the active `home-k3s` target, PostgreSQL is external to Kubernetes.

Requirements:

- `castaway-web` reads `DATABASE_URL` from `castaway-web-secrets`
- `castaway-discord-bot` reads `BOT_STATE_DATABASE_URL` from `castaway-discord-bot-secrets`
- both applications point at the same PostgreSQL server
- each application uses its own logical database and credentials
- database host provisioning, patching, backups, and restore drills are infra-owned concerns

### Bot runtime model

The bot is PostgreSQL-backed in the self-hosted target.

Requirements:

- `BOT_STATE_BACKEND=postgres`
- one replica
- `Recreate` deployment strategy remains acceptable for this target
- bot and web databases stay logically separated even on the same PostgreSQL server
- service-to-service auth remains enabled between bot and web

### Secrets source of truth

Use 1Password as the deployment secret source of truth.

Operational constraint:

- the app repo references Kubernetes `Secret` names only
- infra is responsible for materializing those secrets into the cluster for unattended operation

### Scheduling model

Castaway stateless workloads should land on service nodes, not the appliance/control-plane node by default.

Current node label contract:

- `selfhost.bry-guy.net/role=service`

This label is applied by infra to the appropriate cluster node or nodes.

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

Notes:

- `home-k3s` is the single active self-hosted overlay in this repo
- there is no in-cluster PostgreSQL base for the active target
- image digests are pinned in the overlay Argo CD watches

## Workload responsibilities

### `deploy/base/castaway-web`

Responsible for:

- API deployment
- migration Job definition
- internal Service
- health checks
- config and secret consumption

Runtime assumptions:

- production deployments do not rely on startup auto-migration
- `castaway-web-migrate` runs before app rollout serves traffic
- PostgreSQL is reachable through the external `DATABASE_URL`

### `deploy/base/castaway-discord-bot`

Responsible for:

- bot deployment only
- no ingress
- config and secret consumption
- bot-to-API traffic aimed at the in-cluster `castaway-web` Service

Runtime assumptions:

- one replica
- PostgreSQL-backed bot state
- service-to-service auth remains enabled between bot and web

## Delivery flow in detail

### CI and image publishing

Keep GitHub Actions responsible for:

- lint, test, and build validation
- building changed application images
- publishing those images to GHCR
- updating the pinned digests in `deploy/environments/home-k3s/kustomization.yaml`
- validating that the `home-k3s` overlay still renders

### Argo CD sync

Argo CD should watch:

- `deploy/environments/home-k3s`

Recommended behavior:

- auto-sync enabled
- prune enabled
- self-heal enabled
- `castaway-web-migrate` preserved as a `PreSync` hook

## Operator responsibilities split

### App repo (`~/dev/srvivor`)

Owns:

- application code
- Dockerfiles
- Kubernetes manifests under `deploy/`
- Argo CD `AppProject` and `Application` manifests
- image digest pinning in git

### Infra repo (`~/dev/infra`)

Owns:

- VM provisioning and lifecycle
- k3s bootstrap
- node labels and taints
- external PostgreSQL host installation and hardening
- database backups and restore tests
- secret bridge from 1Password into Kubernetes

## Validation expectations

The minimum repeatable deployment validation for this target is:

```bash
kubectl kustomize deploy/environments/home-k3s > /tmp/home-k3s-render.yaml
```

Broader verification should still run repo CI before merge.

## Compact summary

The active self-hosted Castaway contract is:

- one `home-k3s` overlay
- external PostgreSQL
- Argo CD deploys only app workloads
- image digests stay pinned in the overlay git path Argo watches
- web, bot, and migration Job are scheduled onto service nodes via node label
