# Self-Hosted Home k3s Operator's Guide

Status: `reference`
Type: `operators-guide`

## Short summary

This is the simplest way to think about operating Castaway on the home `k3s` target:

1. Bootstrap the target VM and cluster from `~/dev/infra`.
2. Apply the Argo CD `AppProject` and `Application` from `~/dev/srvivor` once.
3. Make app changes in `~/dev/srvivor`, validate them, and merge to `main`.
4. GitHub Actions builds and pushes Docker images to GHCR.
5. GitHub Actions updates the pinned image digests in `deploy/environments/home-k3s/kustomization.yaml`.
6. Argo CD running in the cluster pulls that new desired state and deploys it to the target VM.
7. `castaway-web` runs its migration Job before the web rollout serves traffic.

In steady state, the normal deployment path is:

- **git change -> merge to `main` -> GitHub Actions publishes image -> Argo CD deploys**

You should not normally hand-apply app manifests or manually push Docker images for routine deploys.

## Purpose

This guide explains how to operate the current self-hosted Castaway deployment path for the first `home-k3s` target.

It covers:

- the operator mental model
- day-0 bootstrap
- normal app change and deployment flow
- how Docker images are produced
- how changes reach the target VM
- how to verify rollout and recover from common issues

## Scope

This guide is about the current self-hosted deployment path only:

- private home `k3s`
- GHCR for images
- Argo CD for cluster sync
- Kubernetes manifests under `deploy/`
- PostgreSQL in-cluster with persistent storage
- private Tailscale-first access

## Primary repos

There are two repos involved in normal operations.

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
- first-pass 1Password -> Kubernetes secret materialization

## Operating model

The intended delivery path is:

1. make a code change in `srvivor`
2. validate locally
3. merge to `main`
4. GitHub Actions builds and pushes changed images to GHCR
5. GitHub Actions updates image digests in `deploy/environments/home-k3s/kustomization.yaml`
6. Argo CD detects the git change
7. Argo CD syncs the cluster on the target VM
8. `castaway-web` migration runs before web rollout
9. workloads settle healthy

This is a **pull-based GitOps** deployment path.

That means the normal steady-state deploy path is:

- change git
- let CI publish images
- let Argo CD pull desired state into the cluster

## Day-0 bootstrap

Day-0 bootstrap is mostly performed from `~/dev/infra`.

### Prerequisites

You need:

- a suitable target VM for Castaway
- Tailscale reachability
- required 1Password items in vault `bry-guy`
- local `mise`, `fnox`, and `kubectl`
- configured values in `~/dev/infra/mise.toml`
- configured secret mappings in `~/dev/infra/fnox.toml`

Primary infra reference:

- `~/dev/infra/docs/selfhost-castaway-k3s-bootstrap.md`

### Bootstrap the Castaway VM and cluster

From `~/dev/infra`:

```bash
mise run "selfhost:castaway:k3s:bootstrap"
mise run "selfhost:castaway:kubeconfig:fetch"
mise run "selfhost:castaway:argocd:bootstrap"
mise run "selfhost:castaway:secrets:sync"
```

Or use the convenience task:

```bash
mise run "selfhost:castaway:bootstrap"
```

What this bootstrap does:

- installs or enables Tailscale on the target VM
- joins the VM to the tailnet
- installs k3s
- fetches a kubeconfig that points at the VM's tailnet hostname
- installs Argo CD in-cluster
- materializes initial Kubernetes `Secret`s from 1Password

### Verify the basic cluster state

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes -o wide
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get ns
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n argocd get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get secrets
```

## Argo CD bootstrap for the Castaway app repo

Installing Argo CD is an infra task, but the Castaway app/project definitions live in `srvivor`.

After the cluster and Argo CD exist, apply the app repo resources once.

From `~/dev/srvivor`:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/project-castaway.yaml
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/app-home-k3s.yaml
```

This tells Argo CD to watch:

- repo: `https://github.com/bry-guy/srvivor.git`
- path: `deploy/environments/home-k3s`

After this step, Argo CD becomes the normal deployment mechanism for application changes.

## Normal operator workflow for application changes

This is the main ongoing workflow.

### 1. Make code changes in `srvivor`

Typical app code changes live in:

- `apps/castaway-web/**`
- `apps/castaway-discord-bot/**`

Deployment and rollout changes live in:

- `deploy/**`
- `.github/workflows/**`
- `script/update-home-k3s-digests.py`

### 2. Validate locally before merging

Run the narrowest meaningful checks for the app you changed.

#### If you changed `castaway-web`

```bash
cd ~/dev/srvivor/apps/castaway-web
mise run lint
mise run test
mise run build
```

#### If you changed `castaway-discord-bot`

```bash
cd ~/dev/srvivor/apps/castaway-discord-bot
mise run lint
mise run test
mise run build
```

#### If you changed shared deploy logic

```bash
cd ~/dev/srvivor
python3 -m py_compile script/update-home-k3s-digests.py
git diff --check
```

#### If you want a broader pass

```bash
cd ~/dev/srvivor
mise run ci
```

### 3. Commit and merge to `main`

Once your change is ready:

```bash
cd ~/dev/srvivor
git add .
git commit -m "feat: ..."
git push
```

Then merge through your normal GitHub or local flow.

For this deployment design, merging to `main` is the normal deployment trigger.

## How Docker images are produced

Docker image publishing is handled by GitHub Actions in:

- `.github/workflows/publish-images.yml`

On pushes to `main`, when relevant app paths changed, the workflow:

- detects which apps changed
- builds only those changed apps
- pushes images to GHCR
- captures the immutable digest
- updates `deploy/environments/home-k3s/kustomization.yaml`
- commits that digest update back to `main`

Current image repositories are:

- `ghcr.io/bry-guy/castaway-web`
- `ghcr.io/bry-guy/castaway-discord-bot`

## Important operator implication

For normal deployments, you should **not** need to manually:

- build Docker images yourself
- push Docker images yourself
- edit live cluster image tags by hand

The intended path is:

- merge code
- let GitHub Actions publish the image
- let GitHub Actions update the digest in git
- let Argo CD deploy the updated desired state

## How a change reaches the target VM

Once the digest update lands in `main`, Argo CD observes the change and syncs:

- `deploy/environments/home-k3s`

That overlay includes:

- PostgreSQL StatefulSet and Service
- `castaway-web` Deployment, Service, and migration Job
- `castaway-discord-bot` Deployment
- ConfigMaps
- Ingress
- environment-specific patches

During rollout:

- `castaway-web-migrate` runs as a pre-traffic Argo CD sync hook
- then the new `castaway-web` Deployment rolls out
- then the bot and web settle to healthy state

The target VM therefore gets updates by **pulling desired state from git** through Argo CD, not by the operator manually pushing YAML to the cluster for routine deploys.

## Rollout verification

Use the kubeconfig produced by the infra bootstrap.

### Check pods

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods
```

### Watch rollout progress

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods -w
```

### Check migration job

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get jobs
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs job/castaway-web-migrate
```

### Check web logs

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-web
```

### Check bot logs

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway logs deploy/castaway-discord-bot
```

### Check services and ingress

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get svc,ingress
```

## Access model

This deployment is intentionally private-first.

Current intended access path:

- Tailscale to the VM
- kubeconfig over the tailnet
- Traefik handling private ingress inside k3s
- no direct public exposure of the home router or public IP

For now, think about access this way:

- operator access happens through Tailscale
- cluster access happens through the tailnet-rewritten kubeconfig
- web access is private ingress
- any future public-friendly tunnel is an infra concern, not an app concern

## Secrets operations

Deployment secrets are sourced from 1Password `bry-guy` and materialized into Kubernetes by infra-owned scripts.

### Rerun secrets sync when values change

If deploy-time secrets change, rerun:

```bash
cd ~/dev/infra
mise run "selfhost:castaway:secrets:sync"
```

Examples:

- PostgreSQL password rotation
- Discord bot token rotation
- service auth token rotation
- any environment secret needed by the deployed workloads

### Important current note

Before the first real production rollout, verify that the Kubernetes secret keys created by the infra sync script match the exact key names expected by the `srvivor` manifests and application config.

If they do not, update the infra secret-sync script before relying on the deployment.

## PostgreSQL operations and persistence

Current deployment shape:

- PostgreSQL runs in-cluster
- it is deployed separately from app rollouts
- it uses persistent storage through a PVC
- app rollouts should not destroy database data

The current logical database split is:

- `castaway_web`
- `castaway_discord_bot`

The web app and Discord bot should use separate logical databases and credentials, even while sharing one PostgreSQL instance.

### Basic persistence checks

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pvc
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get statefulset castaway-postgres
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods -l app.kubernetes.io/name=castaway-postgres
```

### Operator guidance

Treat PostgreSQL as a long-lived workload.

Stateless app rollouts are expected to be routine. PostgreSQL replacement should not be.

Before trusting production data to this system, ensure you have:

- confirmed PVC binding works
- documented where the data actually lives
- documented backup and restore procedures
- understood what VM or disk loss means for recovery

## Local development vs deployed environment

### Local development

For local development in `srvivor`, use the local monorepo stack:

```bash
cd ~/dev/srvivor
mise run start
mise run seed
mise run ps
mise run logs
mise run bot-logs
mise run stop
```

This is the local developer workflow.

### Deployed environment

For the deployed self-hosted environment:

- use `~/dev/infra` for cluster/bootstrap/secrets tasks
- use `~/dev/srvivor` for code, deploy manifests, and Argo definitions
- prefer GitOps and reconciliation over hand-editing cluster state

In normal operation, avoid treating `kubectl apply` of app manifests as the routine deployment method once Argo CD is in charge.

## Steady-state operator happy path

If the environment is already bootstrapped, the normal workflow is:

```bash
cd ~/dev/srvivor

# make your app or deploy change

git add .
git commit -m "feat: improve castaway behavior"
git push
# merge to main
```

Then:

1. GitHub Actions builds and pushes the changed image to GHCR.
2. GitHub Actions updates the pinned digest in `deploy/environments/home-k3s/kustomization.yaml`.
3. Argo CD syncs the cluster.
4. the migration job runs if needed.
5. the workloads roll out.

Then verify:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get jobs
```

## First-time bootstrap happy path

If starting from an unbootstrapped target VM:

```bash
cd ~/dev/infra
mise run "selfhost:castaway:bootstrap"
```

Then:

```bash
cd ~/dev/srvivor
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/project-castaway.yaml
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl apply -f deploy/argocd/app-home-k3s.yaml
```

Then monitor Argo CD and Kubernetes until the workloads settle healthy.

## Troubleshooting

### Image did not deploy

Check:

- the GitHub Actions run for `publish-images.yml`
- whether the digest update commit landed on `main`
- whether Argo CD has synced to the latest commit

### Cluster did not sync

Check:

- `kubectl -n argocd get pods`
- Argo CD application status
- whether Argo CD can read the repo and render the overlay

### Pods are crash-looping or unhealthy

Check:

- whether the required secrets exist
- migration job logs
- web logs
- bot logs
- PostgreSQL pod state
- PVC binding state

### Secrets changed but the deployed app still uses old values

Rerun:

```bash
cd ~/dev/infra
mise run "selfhost:castaway:secrets:sync"
```

### Postgres concerns

Check:

- StatefulSet health
- PVC binding
- whether the storage class is present
- whether data persists across pod restart

## Related references

- `README.md`
- `docs/selfhost-k3s-deployment-blueprint.md`
- `plans/selfhost-k3s-implementation-plan.md`
- `deploy/argocd/project-castaway.yaml`
- `deploy/argocd/app-home-k3s.yaml`
- `/Users/brain/dev/infra/docs/selfhost-castaway-k3s-bootstrap.md`
