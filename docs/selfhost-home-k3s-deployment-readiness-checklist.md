# Self-hosted home-k3s deployment readiness checklist

Use this checklist before attempting the first Castaway deployment to the self-hosted `home-k3s` environment.

This checklist is narrower than `docs/production-readiness-checklist.md`.
It is specifically about whether the self-hosted deployment path is actually wired up and ready to run.

## Repo and GitOps wiring
- [ ] `deploy/argocd/project-castaway.yaml` is applied to the target cluster
- [ ] `deploy/argocd/app-home-k3s.yaml` is applied to the target cluster
- [ ] Argo CD shows `castaway-home-k3s` as a healthy Application object
- [ ] Argo CD is watching `deploy/environments/home-k3s`
- [ ] image digests in `deploy/environments/home-k3s/kustomization.yaml` point at published GHCR images
- [ ] CI render validation for `deploy/environments/home-k3s` is passing

## Cluster and node placement
- [ ] the intended self-hosted cluster is reachable through the kubeconfig used for bootstrap
- [ ] Argo CD is installed in that same cluster
- [ ] the `castaway` namespace exists or can be created by Argo CD
- [ ] at least one schedulable node has label `selfhost.bry-guy.net/role=service`
- [ ] service-node placement is acceptable for:
  - [ ] `castaway-web`
  - [ ] `castaway-web-migrate`
  - [ ] `castaway-discord-bot`

## External PostgreSQL readiness
- [ ] PostgreSQL is running outside Kubernetes on the shared stateful VM
- [ ] network connectivity from service nodes to PostgreSQL is confirmed
- [ ] a logical database exists for `castaway-web`
- [ ] a logical database exists for `castaway-discord-bot`
- [ ] separate credentials exist for the web app and bot
- [ ] backup ownership and restore expectations are documented in infra

## Kubernetes secret materialization
- [ ] infra has materialized `castaway-web-secrets` into the `castaway` namespace
- [ ] infra has materialized `castaway-discord-bot-secrets` into the `castaway` namespace

### Required keys for `castaway-web-secrets`
- [ ] `DATABASE_URL`
- [ ] `SERVICE_AUTH_BEARER_TOKENS`

### Required keys for `castaway-discord-bot-secrets`
- [ ] `CASTAWAY_DISCORD_BOT_TOKEN`
- [ ] `CASTAWAY_DISCORD_APPLICATION_ID`
- [ ] `CASTAWAY_API_AUTH_TOKEN`
- [ ] `BOT_STATE_DATABASE_URL`

### Environment-specific optional keys
- [ ] `DISCORD_BRAINLAND_SERVER_ID` is set if the target bot workflow needs a dev/test guild binding

## App contract checks
- [ ] `castaway-web` is configured with `AUTO_MIGRATE=false`
- [ ] `castaway-web-migrate` remains an Argo CD `PreSync` hook
- [ ] `castaway-discord-bot` is configured with `BOT_STATE_BACKEND=postgres`
- [ ] `castaway-discord-bot` points at in-cluster API URL `http://castaway-web:8080`
- [ ] web and bot secret names match the manifests exactly
- [ ] service-auth token contract matches between web and bot secrets

## First deployment verification
- [ ] `kubectl kustomize deploy/environments/home-k3s` renders cleanly
- [ ] Argo CD sync succeeds without manual manifest edits
- [ ] the migration Job completes successfully
- [ ] `castaway-web` becomes Ready
- [ ] `castaway-discord-bot` becomes Ready
- [ ] both workloads land on service-labeled nodes
- [ ] `castaway-web` can reach PostgreSQL successfully
- [ ] `castaway-discord-bot` can reach PostgreSQL successfully
- [ ] `castaway-discord-bot` can reach `castaway-web` successfully

## Operational handoff
- [ ] the operator knows which kubeconfig points at the intended self-hosted cluster
- [ ] the operator knows that Argo CD deploys into the cluster it runs inside, not directly to a VM by name
- [ ] the operator knows that node labels determine which VM/node actually runs the workloads
- [ ] the operator knows that PostgreSQL host provisioning, backups, and secret sync are infra-owned responsibilities
