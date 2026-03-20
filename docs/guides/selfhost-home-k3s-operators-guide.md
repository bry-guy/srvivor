# Self-Hosted Home k3s Operator's Guide

Status: `reference`
Type: `operators-guide`

## Short summary

Operate Castaway on the self-hosted `home-k3s` target like this:

1. Bootstrap shared VMs, k3s, secrets, and PostgreSQL from `~/dev/infra`.
2. Apply the Argo CD project/application from `~/dev/srvivor` once.
3. Make app changes in `~/dev/srvivor`, validate them, and merge to `main`.
4. GitHub Actions publishes changed images to GHCR.
5. GitHub Actions updates pinned digests in `deploy/environments/home-k3s/kustomization.yaml`.
6. Argo CD pulls the new desired state and deploys app workloads into the cluster.
7. `castaway-web-migrate` runs before the new web rollout serves traffic.

The normal deployment path is:

- **git change -> merge to `main` -> GitHub Actions publishes image -> Argo CD deploys**

## Scope

This guide covers the current self-hosted deployment contract only:

- private home `k3s`
- GHCR for images
- Argo CD for cluster sync
- Kubernetes manifests under `deploy/`
- PostgreSQL hosted **outside** the Castaway overlay on the shared stateful VM
- private Tailscale-first access

## Repo responsibilities

### App repo

- `~/dev/srvivor`

Owns:

- application code
- Dockerfiles
- Kubernetes manifests under `deploy/`
- Argo CD `AppProject` and `Application` definitions
- GitHub Actions image publishing flow

### Infra repo

- `~/dev/infra`

Owns:

- VM/bootstrap access and cluster bootstrap
- PostgreSQL installation, lifecycle, and backups on the shared stateful VM
- Tailscale and kubeconfig bootstrap path
- Argo CD installation bootstrap
- 1Password -> Kubernetes secret materialization
- service-node labels and wider cluster placement policy

## Current deployment contract

For `home-k3s`:

- Argo CD watches `deploy/environments/home-k3s`
- PostgreSQL is external to the Castaway overlay
- `castaway-web` reads `DATABASE_URL` from `castaway-web-secrets`
- `castaway-discord-bot` reads `BOT_STATE_DATABASE_URL` from `castaway-discord-bot-secrets`
- both apps point at the same PostgreSQL server with separate DB/users
- stateless app workloads target nodes labeled `selfhost.bry-guy.net/role=service`

## Day-0 bootstrap

Day-0 bootstrap is mostly performed from `~/dev/infra`.

### Prerequisites

You need:

- appliance / service / stateful VMs ready or planned in infra
- Tailscale reachability
- required 1Password items in vault `bry-guy`
- local `mise`, `fnox`, and `kubectl`
- configured values in `~/dev/infra/mise.toml`
- configured secret mappings in `~/dev/infra/fnox.toml`

### Bootstrap infra-managed dependencies

From `~/dev/infra`:

```bash
mise run "selfhost:castaway:k3s:bootstrap"
mise run "selfhost:castaway:kubeconfig:fetch"
mise run "selfhost:castaway:argocd:bootstrap"
mise run "selfhost:castaway:secrets:sync"
```

Infra should also ensure:

- PostgreSQL exists on the shared stateful VM
- Castaway web and bot databases/users exist
- the connection strings materialized into Kubernetes secrets match the names expected by this repo
- service nodes are labeled `selfhost.bry-guy.net/role=service`

### Verify basic cluster state

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes --show-labels
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get ns
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n argocd get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get secrets
```

## Argo CD bootstrap for the app repo

After the cluster and Argo CD exist, apply the app repo resources once.

From `~/dev/srvivor`:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/project-castaway.yaml
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/app-home-k3s.yaml
```

This tells Argo CD to watch:

- repo: `https://github.com/bry-guy/srvivor.git`
- path: `deploy/environments/home-k3s`

## Normal operator workflow

### 1. Make changes in `srvivor`

Typical changes live in:

- `apps/castaway-web/**`
- `apps/castaway-discord-bot/**`
- `deploy/**`
- `.github/workflows/**`
- `script/update-home-k3s-digests.py`

### 2. Validate before merging

Run the narrowest meaningful checks for the changed area.

#### Web app

```bash
cd ~/dev/srvivor/apps/castaway-web
mise run lint
mise run test
mise run build
```

#### Discord bot

```bash
cd ~/dev/srvivor/apps/castaway-discord-bot
mise run lint
mise run test
mise run build
```

#### Deploy and automation changes

```bash
cd ~/dev/srvivor
python3 -m py_compile script/update-home-k3s-digests.py
mise run ci
```

### 3. Merge to `main`

Merging to `main` is the normal deployment trigger.

## How a change reaches the cluster

Once a digest update lands in `main`, Argo CD syncs:

- `deploy/environments/home-k3s`

That overlay includes:

- `castaway-web` Deployment, Service, and migration Job
- `castaway-discord-bot` Deployment
- ConfigMaps
- ingress
- environment-specific patches
- service-node placement rules

It does **not** include PostgreSQL manifests.

During rollout:

- `castaway-web-migrate` runs as a `PreSync` hook
- the new web Deployment rolls out
- the bot and web settle to healthy state

## Rollout verification

### Check workloads

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get jobs
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get svc,ingress
```

### Check migration job

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs job/castaway-web-migrate
```

### Check workload placement

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods -o wide
```

Confirm the web, migration, and bot workloads landed on nodes carrying:

- `selfhost.bry-guy.net/role=service`

### Check app logs

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-web
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-discord-bot
```

## Secrets operations

Deployment secrets are sourced from 1Password `bry-guy` and materialized into Kubernetes by infra-owned automation.

If deploy-time secrets change, rerun from `~/dev/infra`:

```bash
mise run "selfhost:castaway:secrets:sync"
```

Important checks:

- `castaway-web-secrets` must include `DATABASE_URL`
- `castaway-discord-bot-secrets` must include `BOT_STATE_DATABASE_URL`
- service-auth values must still match the app contract

## PostgreSQL operations boundary

PostgreSQL operations are not owned by this repo.

For the current target, infra is responsible for:

- PostgreSQL installation on the shared stateful VM
- logical database/user provisioning
- backups and restore testing
- server health and capacity
- host-level maintenance

The app repo operator should only need to confirm that application secrets point at the right PostgreSQL host and that migrations succeed.

## Troubleshooting

### Image did not deploy

Check:

- the GitHub Actions run for `publish-images.yml`
- whether the digest update commit landed on `main`
- whether Argo CD synced the latest commit

### Overlay does not render or sync

Check:

- Argo CD application status
- whether the `home-k3s` overlay still renders in CI
- whether image entries still exist in `deploy/environments/home-k3s/kustomization.yaml`

### Pods are crash-looping or unhealthy

Check:

- required secrets exist
- migration job logs
- web and bot logs
- the external PostgreSQL host is reachable from the cluster

### Workloads landed on the wrong node

Check:

- service-node labels exist on the intended nodes
- the overlay still contains the placement patches
- cluster taints/tolerations did not change under the contract

## Related references

- `README.md`
- `docs/selfhost-k3s-deployment-blueprint.md`
- `plans/selfhost-k3s-implementation-plan.md`
- `deploy/argocd/project-castaway.yaml`
- `deploy/argocd/app-home-k3s.yaml`
- `/Users/brain/dev/infra/docs/selfhost-castaway-k3s-bootstrap.md`
