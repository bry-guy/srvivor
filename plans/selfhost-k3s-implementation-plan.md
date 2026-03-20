# Self-Hosted k3s Implementation Plan

Status: `planning`

## Goal

Turn `docs/selfhost-k3s-deployment-blueprint.md` into an executable, agent-friendly set of implementation work for the current self-hosted deployment target.

This document is the implementation plan. The blueprint remains the structural design/reference doc.

## Scope

This plan covers the current self-hosted deployment target only:

- `home-k3s`
- GitHub Actions + GHCR + Argo CD
- external PostgreSQL hosted on the shared stateful VM
- dedicated `castaway-web` migration Job
- bot-to-API service authentication
- PostgreSQL-backed bot state
- private Tailscale-first access
- service-node placement for stateless workloads

## Required inputs

Agents should read these documents before starting work:

- `docs/selfhost-k3s-deployment-blueprint.md`
- `docs/guides/selfhost-home-k3s-operators-guide.md`
- `apps/castaway-web/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/postgres-state-backend-planning.md`
- `/Users/brain/dev/infra/docs/plans/castaway-selfhost-k3s-private-ingress-and-secrets-plan.md`

## Hard constraints

- Only `home-k3s` is in scope for implementation.
- Production `castaway-web` deploys must use a dedicated migration Job or equivalent pre-traffic hook.
- Production web pods must not rely on startup auto-migration.
- The app repo must not assume ownership of PostgreSQL deployment, bootstrap, or backup resources for this target.
- The home public IP must not be exposed.
- 1Password `bry-guy` remains the deployment secret source of truth for the self-hosted path.
- This does not replace the repo's existing local-development `castaway` vault workflow.

## Locked assumptions

Agents should treat the assumptions below as locked unless the user explicitly changes them.

If an assumption is insufficient or contradictory during implementation, stop and ask the user rather than freelancing a new contract.

### Shared contracts to preserve

- Kubernetes namespace name
- image names and registry paths
- Kubernetes object names for:
  - `castaway-web`
  - `castaway-discord-bot`
  - migration Job
- ConfigMap and Secret names
- database names:
  - `castaway_web`
  - `castaway_discord_bot`
- migration command contract for `castaway-web`
- service-auth header and environment variable names
- bot database environment variable names
- service-node label:
  - `selfhost.bry-guy.net/role=service`

### Recommended defaults

Use these values unless the user chooses different ones:

- namespace: `castaway`
- web Service name: `castaway-web`
- bot Deployment name: `castaway-discord-bot`
- migration Job name: `castaway-web-migrate`
- config objects:
  - `castaway-web-config`
  - `castaway-discord-bot-config`
- secret objects:
  - `castaway-web-secrets`
  - `castaway-discord-bot-secrets`

## Rules of engagement for agents

### Worktree and branch isolation

- Each agent should work in its own git worktree or isolated branch.
- Do not share a mutable working directory across agents.
- Keep commits scoped to one workstream.

### File ownership

- Stay inside the file ownership listed for your workstream.
- If you must touch another workstream's files, stop and ask the user before expanding scope.
- Shared documents should only be updated when your workstream explicitly owns them or the user asks for the cross-cutting edit.

### Contract discipline

- Do not rename shared Kubernetes objects, env vars, image names, or database names on your own.
- Do not reintroduce in-cluster PostgreSQL manifests for `home-k3s` without an explicit contract change.
- If an interface is underspecified, document the gap and ask the user for the missing decision.

### Validation discipline

- Run the narrowest meaningful checks for your workstream before handoff.
- The final integration pass should run the broadest repo-level checks.
- Do not skip tests just because another agent owns adjacent code.

## Parallel workstreams

The work is intentionally sliced by file ownership so multiple agents can progress simultaneously.

### Workstream A — deployment manifests and Argo CD wiring

**Owner repo:** `srvivor`

**Primary file ownership:**

- `deploy/**`

**Deliverables:**

- `deploy/base/castaway-web`
- `deploy/base/castaway-discord-bot`
- `deploy/environments/home-k3s`
- `deploy/argocd`
- dedicated `castaway-web` migration Job manifest
- overlay patch that turns the migration Job into an Argo CD `PreSync` hook or equivalent ordered sync step
- service-node placement patches for web, migration, and bot workloads
- web Deployment configured for startup auto-migration disabled in cluster
- bot Deployment configured for one replica and `Recreate` semantics
- no in-cluster PostgreSQL resources in the active `home-k3s` overlay

**Validation expectation:**

- render the `home-k3s` overlay
- if no render task exists yet, add a minimal repeatable render check instead of relying on manual eyeballing

### Workstream B — GitHub Actions image publishing and digest updates

**Owner repo:** `srvivor`

**Primary file ownership:**

- `.github/workflows/**`
- helper scripts used only by those workflows

**Deliverables:**

- workflow to build and publish changed app images to GHCR
- script path to update image digests in `deploy/environments/home-k3s`
- path filtering so unchanged apps do not rebuild unnecessarily
- immutable image reference flow suitable for Argo CD consumption
- render validation for the active `home-k3s` overlay so delivery automation cannot silently drift

**Dependencies:**

- deployment path shape from Workstream A

**Validation expectation:**

- confirm workflow YAML remains structurally valid
- ensure existing CI behavior is preserved
- final integration should run repo CI after merge

### Workstream C — `castaway-web` production deployment contract

**Owner repo:** `srvivor`

**Primary file ownership:**

- `apps/castaway-web/**`

**Deliverables:**

- dedicated migration entrypoint or command for `castaway-web`
- documentation and config updates that make the migration Job the production path
- service-to-service auth middleware for bot-to-API traffic
- tests for missing, invalid, and valid service credentials
- startup/config validation that matches the agreed production contract

**Validation expectation:**

Run at least:

```bash
cd apps/castaway-web
mise run lint
mise run test
mise run build
```

### Workstream D — `castaway-discord-bot` production client and state backend

**Owner repo:** `srvivor`

**Primary file ownership:**

- `apps/castaway-discord-bot/**`

**Deliverables:**

- bot-to-API auth header support
- bot config for service auth and bot database access
- PostgreSQL-backed state store using the bot's own logical database
- explicit migration/import path from BoltDB if existing saved defaults must be preserved
- updated bot docs and operational notes

**Validation expectation:**

Run at least:

```bash
cd apps/castaway-discord-bot
mise run lint
mise run test
mise run build
```

### Workstream E — infrastructure bootstrap, private ingress, and secret bridge

**Owner repo:** `infra`

This remains outside this repo.

## Final integration expectations

At integration time, verify at minimum:

- the `home-k3s` overlay renders
- CI and image-publishing automation still point at the active `home-k3s` overlay
- docs describe external PostgreSQL ownership correctly
- app secret contracts remain unchanged
