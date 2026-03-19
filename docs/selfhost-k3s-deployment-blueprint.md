# Self-Hosted k3s Deployment Blueprint

Status: `planning`
Type: `blueprint`

## Goal

Define the first concrete deployment design for Castaway on a self-hosted k3s VM, with a clean path from monorepo changes to running workloads in the cluster.

This document is descriptive design/reference documentation. Execution sequencing and agent work breakdown live in the companion implementation plan.

Execution companion:

- `plans/selfhost-k3s-implementation-plan.md`

This blueprint is intentionally opinionated for the first target only:

- self-hosted k3s on the home VM
- GitHub Actions for CI and image publishing
- GHCR for image storage
- Argo CD for cluster-side deployment sync
- Kustomize for Kubernetes manifests
- in-cluster PostgreSQL with persistent storage
- private access only; no direct public exposure of the home IP

## Non-goals for this first slice

- implementing Oracle, AWS, or other cloud targets now
- exposing the home router or Proxmox host directly to the public internet
- designing a multi-replica Discord bot before its state backend is changed
- treating the current local BoltDB file as the long-term production state backend

Future targets are kept in view only as portability constraints.

## Locked-in decisions for v1

### Delivery model

Use pull-based GitOps:

1. merge to `main`
2. GitHub Actions runs CI
3. changed app images are built and pushed to GHCR
4. CI updates the image digests referenced by the `home-k3s` overlay
5. Argo CD in the cluster syncs the new desired state
6. the `castaway-web` migration Job runs before new web pods serve traffic
7. k3s rolls out updated pods

### Deploy tooling

Use:

- GitHub Actions
- GHCR
- Argo CD
- Kustomize

Do not introduce Helm unless a specific packaged dependency makes it materially simpler.

### Database model

For the first self-hosted deployment, PostgreSQL runs inside the cluster as its own long-lived workload with persistent storage.

Requirements:

- PostgreSQL must be deployed separately from the app rollouts
- PostgreSQL data must live on a persistent volume
- app pod restarts and rollouts must not destroy or recreate the data volume
- backups and restore procedures must be handled as an operational concern, not assumed away by Kubernetes

Even with in-cluster PostgreSQL, app manifests should still consume database connection info through environment variables and secrets so another environment can later point them at a different Postgres host.

### Bot state path

Short term:

- `castaway-discord-bot` stays single-replica
- it keeps its current local state backend initially
- it uses a PVC and `Recreate` rollout semantics if that file-backed path is deployed in Kubernetes

Planned direction:

- move the bot state backend to PostgreSQL
- use the same Postgres instance as `castaway-web`
- use a separate bot database and credentials so the bot can later move independently

### Secrets source of truth

Use 1Password as the source of truth, specifically the existing `bry-guy` vault.

Operational constraint:

- deployments must not depend on an interactive 1Password session
- use dedicated service-account-based access for unattended secret reads

This is a deployment/infrastructure decision for the self-hosted path. It does not replace the repo's existing local-development `castaway` vault and `fnox` workflow.

The application repo should reference Kubernetes `Secret` names only. The bridge from 1Password into the cluster belongs to infrastructure provisioning and operations.

### Exposure model

- no public IP exposure from the self-hosted box
- no home-router port forwarding as part of this app deployment design
- initial access should work over Tailscale/tailnet paths
- long-term friendly hostname can be delivered by a secure outbound tunnel layer such as Cloudflare Tunnel

The applications themselves should not know whether requests arrived via tailnet-only access, a tunnel, or some later ingress path. They only need Kubernetes Services and, for the web app, an ingress rule or equivalent reverse-proxy route.

## Proposed repository layout

```text
deploy/
  base/
    postgres/
      kustomization.yaml
      service.yaml
      statefulset.yaml

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
      postgres-patch.yaml
      postgres-init-configmap.yaml
      web-configmap.yaml
      web-patch.yaml
      web-migration-hook-patch.yaml
      bot-configmap.yaml
      bot-patch.yaml
      ingress-web.yaml
      kustomization.yaml

  argocd/
    project-castaway.yaml
    app-home-k3s.yaml
```

Notes:

- only `home-k3s` is part of the initial repo shape
- future overlays for Oracle VM + k3s, AKS, EKS, or other targets are explicitly deferred
- `postgres-init-configmap.yaml` is where the self-hosted environment can create the initial logical databases and roles for `castaway-web` and the future bot database

## Workload responsibilities

### `deploy/base/postgres`

Responsible for:

- one PostgreSQL StatefulSet
- one persistent volume claim via `volumeClaimTemplates` or an equivalent persistent volume path
- one stable `ClusterIP` Service
- liveness/readiness probes
- a durable data path

Initial logical database shape:

- `castaway_web`
- `castaway_discord_bot`

Initial role shape:

- one role scoped to `castaway_web`
- one role scoped to `castaway_discord_bot`

Even before the bot is migrated, provisioning the second database now keeps the split explicit.

### `deploy/base/castaway-web`

Responsible for:

- API deployment
- dedicated migration Job definition
- stable internal service name
- `/healthz` readiness and liveness checks
- consuming `DATABASE_URL` and future auth-related secrets from Kubernetes

Runtime assumptions:

- PostgreSQL is reachable through cluster DNS
- production deployments do not rely on app-startup migrations
- the web Deployment runs with startup auto-migration disabled
- the migration Job runs before new web pods serve traffic
- public exposure is not required for the app to function for the bot

### `deploy/base/castaway-discord-bot`

Responsible for:

- bot deployment only
- no ingress
- consuming Discord credentials, Castaway API base URL, and future service-auth credentials from Kubernetes

Short-term runtime assumptions:

- one replica only
- PVC-backed local state if the file store is still in use
- internal API calls go to the in-cluster web Service

## Delivery flow in detail

### CI and image publishing

Keep the existing CI workflow for lint/test/build.

Add a publish workflow that:

- runs on merges to `main`
- builds only the apps whose paths changed
- publishes images to GHCR
- tags each image with at least:
  - `sha-<git-sha>`
  - semver tags when release flows produce them

### Desired-state update

After publishing, CI updates the image digests in:

- `deploy/environments/home-k3s/kustomization.yaml`

This keeps Git as the source of truth that Argo CD watches.

### Argo CD sync

Argo CD in the cluster should:

- watch `deploy/environments/home-k3s`
- auto-sync
- prune removed resources
- self-heal drift where appropriate
- run the `castaway-web` migration Job before rolling new web pods

Recommended shape:

- keep the generic migration Job manifest in `deploy/base/castaway-web/migration-job.yaml`
- add Argo CD hook annotations from the `home-k3s` overlay via `web-migration-hook-patch.yaml`
- keep Argo-specific behavior out of the base manifests when possible

## Registry choice

Use GHCR first.

Why:

- native GitHub Actions integration
- no extra registry to operate
- good fit for a small self-hosted deployment

Billing note:

- public packages are free
- GitHub currently documents container registry storage and bandwidth as free for the container registry
- broader GitHub Packages quotas still apply for private-package billing contexts, so verify current GitHub billing rules if repo visibility or usage changes materially

## Secrets and configuration model

### Kubernetes-facing contract

The manifests in this repo should assume named Kubernetes objects such as:

- `castaway-web-config`
- `castaway-web-secrets`
- `castaway-discord-bot-config`
- `castaway-discord-bot-secrets`
- `castaway-postgres-secrets`

### Secret source of truth

1Password `bry-guy` remains the source of truth.

This repo should not commit encrypted or plaintext copies of those values as its primary secret workflow.

### Infra boundary

The infra repo is responsible for provisioning or documenting the secret bridge that copies or reconciles values from 1Password into Kubernetes using unattended credentials.

That means this repo stays decoupled from whether infra uses:

- a one-time bootstrap sync
- a periodic sync job
- a Kubernetes operator
- another service-account-backed secret ingestion path

## Private ingress and access model

### Initial access

The first operational access path should be tailnet-only.

Practical shape:

- the k3s VM joins Tailscale
- Traefik on the VM handles HTTP routing for cluster services
- no inbound home-router forwarding is required
- trusted tailnet devices can reach the ingress path over the tailnet

### Friendly hostname later

Long-term desired hostname:

- `castaway.bry-guy.net`

Recommended future shape:

- provision a Cloudflare Tunnel or equivalent outbound tunnel in infrastructure
- route `castaway.bry-guy.net` to the internal ingress path
- keep the home IP private and unadvertised

### What this repo needs to know

Very little.

This repo only needs to define:

- web Service
- optional ingress rule / host routing for the web app
- internal DNS names used by workloads

Tunnel choice, DNS records, TLS termination details, and tailnet mechanics belong to infrastructure.

## Migration strategy

### What migrations are

A database migration is an ordered change to the database schema or stored data so that a newer version of the application can run safely.

Examples:

- create a new table
- add a column
- backfill data
- create an index
- split one table into two

### Why they matter here

`castaway-web` already applies SQL migrations. As soon as the app runs against a long-lived PostgreSQL instance, schema changes become a first-class deployment concern.

### Trade-offs

#### Startup migrations from the app pod

Pros:

- simplest to get working
- low ceremony for a single-replica home deployment

Cons:

- app rollout and schema change are tightly coupled
- multiple pods starting together can race to migrate
- failed migrations show up as failed app startups rather than a distinct deployment step
- rollback is harder to reason about

#### Dedicated migration Job or Argo CD hook

Pros:

- one clear actor runs the migration
- failure happens before new app pods serve traffic
- easier to observe and operate
- scales better once multiple replicas exist

Cons:

- one more deployment object and step to manage
- slightly more initial setup work

### Decision for the first slice

- require a dedicated migration Job or equivalent pre-traffic hook from the first self-hosted deployment
- disable startup auto-migration in the deployed `castaway-web` pods
- prefer Argo CD `PreSync` execution or an equivalent ordered sync mechanism so failed migrations stop the rollout before new pods serve traffic
- prefer additive, backward-compatible migrations where possible so code rollouts and rollbacks stay safer

## Portability table

The current implementation target is only `home-k3s`. The other columns are design constraints and proposed future targets, not current deliverables.

| Concern | home-k3s (current target) | Oracle VM + k3s (proposed) | Managed cloud cluster such as AKS/EKS (proposed) |
|---|---|---|---|
| Cluster type | k3s on self-hosted VM | k3s on VM | managed Kubernetes |
| Delivery flow | GitHub Actions + GHCR + Argo CD | same | same |
| App manifests | same base manifests | same base manifests | same base manifests |
| Overlay count | `home-k3s` only | future overlay | future overlay |
| PostgreSQL | in-cluster StatefulSet + PVC | likely external VM Postgres or same pattern | likely managed Postgres |
| Secrets source | 1Password `bry-guy` via infra-managed bridge | same source, different bridge details | same source or cloud secret manager later |
| Ingress | tailnet-first, private | private tunnel or cloud edge | cloud ingress / private edge |
| Public IP exposure | none | optional, but avoid by design | platform-dependent |
| Bot replica model | 1 replica until state backend changes | same | same until backend changes |
| Bot state backend | Bolt file first, Postgres planned | Postgres preferred | Postgres preferred |

## Execution phases

### Phase 1: repo deployment scaffolding

Add:

- `deploy/base/postgres`
- `deploy/base/castaway-web`
- `deploy/base/castaway-discord-bot`
- `deploy/environments/home-k3s`
- `deploy/argocd`
- dedicated `castaway-web` migration Job manifests and overlay hook patches

### Phase 2: image publishing

Add GitHub Actions workflows for:

- building changed app images
- pushing to GHCR
- updating `home-k3s` image digests in git

### Phase 3: cluster bootstrap

In the target cluster/infrastructure:

- install k3s
- confirm Traefik behavior
- install Argo CD
- provision the 1Password-to-Kubernetes secret bridge
- apply the Argo CD app for `home-k3s`

### Phase 4: app hardening

- add service-to-service auth between bot and API
- move bot state to PostgreSQL
- document backup, restore, and rollback procedures
- add deployment render/smoke validation around the migration hook path

## Compact handoff summary

- Start with one target only: `home-k3s`.
- Use GitHub Actions + GHCR + Argo CD + Kustomize.
- Run PostgreSQL in-cluster, but as its own long-lived persistent workload.
- Provision both `castaway_web` and `castaway_discord_bot` databases on that instance.
- Require a dedicated `castaway-web` migration Job before web rollouts; do not rely on startup auto-migration in production pods.
- Keep the bot single-replica until its state backend moves off the local Bolt file.
- Use 1Password `bry-guy` as the secret source of truth through unattended service-account access managed by infra.
- Keep the home IP private; use Tailscale first and an outbound tunnel later for `castaway.bry-guy.net`.
- Treat cloud targets as proposed portability constraints, not part of the initial repo layout.
