# Self-Hosted Home k3s Operator's Guide

Status: `reference`
Type: `operators-guide`

## Short summary

Operate Castaway on the home `k3s` target like this:

1. bootstrap VMs, k3s, PostgreSQL, and secrets from `~/dev/infra`
2. apply the Argo CD project and app manifests from `~/dev/srvivor` once
3. make app changes in `~/dev/srvivor` and merge to `main`
4. GitHub Actions builds images, pins digests into `deploy/environments/home-k3s/kustomization.yaml`, and validates the overlay render
5. Argo CD syncs the app workloads into the cluster
6. `castaway-web-migrate` runs before the web rollout serves traffic

In steady state, the deployment path is:

- **git change -> merge to `main` -> GitHub Actions publishes image and updates digest -> Argo CD deploys**

## Scope

This guide covers the active self-hosted deployment path only:

- private home `k3s`
- GHCR for images
- Argo CD for sync
- Kubernetes manifests under `deploy/`
- external PostgreSQL on a shared stateful VM
- service-node scheduling for app workloads
- private access patterns managed by infra

## Primary repos

### App repo

- `~/dev/srvivor`

This repo owns:

- application code
- Dockerfiles
- Kubernetes manifests under `deploy/`
- Argo CD `AppProject` and `Application` definitions
- GitHub Actions image publishing flow

### Infra repo

- `~/dev/infra`

This repo owns:

- VM/bootstrap access and cluster bootstrap
- Tailscale and kubeconfig bootstrap path
- Argo CD installation bootstrap
- Kubernetes secret materialization from 1Password
- external PostgreSQL installation, patching, backups, and restore drills
- node labels such as `selfhost.bry-guy.net/role=service`

## Operating model

The intended delivery path is:

1. make a code change in `srvivor`
2. validate locally
3. merge to `main`
4. GitHub Actions builds and pushes changed images to GHCR
5. GitHub Actions updates image digests in `deploy/environments/home-k3s/kustomization.yaml`
6. CI validates `kubectl kustomize deploy/environments/home-k3s`
7. Argo CD detects the git change
8. Argo CD syncs the cluster
9. `castaway-web` migration runs before the web rollout
10. workloads settle healthy on service nodes

## Day-0 bootstrap

Day-0 bootstrap is mostly performed from `~/dev/infra`.

### Prerequisites

You need:

- appliance/control-plane and service/stateful VM targets prepared by infra
- Tailscale reachability where required
- required 1Password items in the deployment vault
- local `mise`, `fnox`, and `kubectl`
- configured infra-side bootstrap values

### Bootstrap infra-managed components

From `~/dev/infra`, run the project-specific bootstrap tasks that install or configure:

- k3s
- Argo CD
- Kubernetes secrets
- external PostgreSQL
- node labels for workload placement
- backups and restore verification for PostgreSQL

Exact commands live in the infra repo because this repo does not own those workflows.

### Verify the basic cluster state

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes -o wide --show-labels
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get ns
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n argocd get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get secrets
```

Before deploying apps, confirm at least one target node has:

- `selfhost.bry-guy.net/role=service`

## Argo CD bootstrap for the Castaway app repo

After the cluster and Argo CD exist, apply the app repo resources once.

From `~/dev/srvivor`:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/project-castaway.yaml
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/app-home-k3s.yaml
```

This tells Argo CD to watch:

- repo: `https://github.com/bry-guy/srvivor.git`
- path: `deploy/environments/home-k3s`

## Secrets and database contract

The active self-hosted target assumes external PostgreSQL.

Application contracts:

- `castaway-web-secrets` provides `DATABASE_URL`
- `castaway-discord-bot-secrets` provides `BOT_STATE_DATABASE_URL`
- `castaway-web` and `castaway-discord-bot` use separate logical databases and credentials
- `BOT_STATE_BACKEND=postgres`
- `DISCORD_TARGET_SEVER_ID` is the target guild env var; source it from whichever 1Password item matches the environment

This repo does **not** create or manage PostgreSQL as a Kubernetes workload.

## Normal operator workflow for app changes

### 1. Make code or deployment changes in `srvivor`

Typical locations:

- `apps/castaway-web/**`
- `apps/castaway-discord-bot/**`
- `deploy/**`
- `.github/workflows/**`
- `script/update-home-k3s-digests.py`

### 2. Validate locally before merging

If you changed app code, run the app-local checks.

If you changed deployment wiring, at minimum render the overlay:

```bash
kubectl kustomize deploy/environments/home-k3s > /tmp/home-k3s-render.yaml
```

If you want a broader pass:

```bash
mise run ci
```

### 3. Commit and merge to `main`

Once your change is ready:

```bash
git add .
git commit -m "feat: ..."
git push
```

Merging to `main` is the normal deploy trigger.

## How images and desired state are updated

GitHub Actions is responsible for:

- building changed app images
- pushing them to GHCR
- updating pinned digests in `deploy/environments/home-k3s/kustomization.yaml`
- validating that the active overlay still renders

This avoids delivery automation drift because Argo CD watches the same overlay file CI updates.

## What Argo CD deploys

The active overlay contains:

- namespace
- `castaway-web` deployment and service
- `castaway-web-migrate` job
- `castaway-discord-bot` deployment
- configmaps
- ingress
- scheduling patches for service-node placement

The overlay does **not** contain:

- PostgreSQL StatefulSet
- PostgreSQL Service
- PostgreSQL init ConfigMap

## Placement and rollout expectations

Stateless Castaway workloads should schedule onto nodes labeled:

- `selfhost.bry-guy.net/role=service`

This includes:

- `castaway-web`
- `castaway-web-migrate`
- `castaway-discord-bot`

If pods remain Pending, first verify the label exists on at least one schedulable node.

## Rollout verification

Useful checks:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get all
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods -o wide
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway describe job castaway-web-migrate
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs job/castaway-web-migrate
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-web
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-discord-bot
```

Confirm:

- migration Job succeeded
- web pods are Ready
- bot pod is running
- workloads landed on service nodes

## Troubleshooting checklist

### Argo app exists but workloads do not progress

Check:

- Argo CD application sync status
- overlay render success locally
- missing secrets in `castaway` namespace
- service-node label presence

### Migration job fails

Check:

- `DATABASE_URL` secret contents
- network reachability from service node to external PostgreSQL host
- migration binary and container image digest

### Bot fails to start

Check:

- `BOT_STATE_DATABASE_URL`
- `BOT_STATE_BACKEND=postgres`
- Discord token secret values
- API auth token alignment between bot and web secrets/config

### Pods remain Pending

Check:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes --show-labels | grep selfhost.bry-guy.net/role=service
```

If no node matches, fix infra/node labeling first.

## Responsibilities that stay outside this repo

Do not use this repo to manage:

- VM provisioning
- PostgreSQL installation or upgrades
- PostgreSQL backups and restore drills
- Tailscale topology
- cluster node labeling policy beyond consuming the agreed label

## Compact summary

For the active self-hosted target:

- infra owns PostgreSQL and secrets materialization
- this repo owns app manifests and Argo CD app wiring
- `home-k3s` is the single active overlay
- CI updates the same overlay Argo CD watches
- workloads schedule to nodes labeled `selfhost.bry-guy.net/role=service`
