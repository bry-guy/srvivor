# Self-Hosted k3s Implementation Plan

Status: `planning`

## Goal

Turn `docs/selfhost-k3s-deployment-blueprint.md` into an executable, agent-friendly set of implementation work for the first self-hosted deployment target.

This document is the implementation plan. The blueprint remains the structural design/reference doc.

## Scope

This plan covers the first self-hosted deployment target only:

- `home-k3s`
- GitHub Actions + GHCR + Argo CD
- in-cluster PostgreSQL with persistent storage
- dedicated `castaway-web` migration Job
- bot-to-API service authentication
- bot state migration toward PostgreSQL
- private Tailscale-first access

## Required inputs

Agents should read these documents before starting work:

- `docs/selfhost-k3s-deployment-blueprint.md`
- `apps/castaway-web/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/service-to-service-authentication-planning.md`
- `apps/castaway-discord-bot/plans/postgres-state-backend-planning.md`
- `/Users/brain/dev/infra/docs/plans/castaway-selfhost-k3s-private-ingress-and-secrets-plan.md`

## Hard constraints

- Only `home-k3s` is in scope for implementation.
- Production `castaway-web` deploys must use a dedicated migration Job or equivalent pre-traffic hook.
- Production web pods must not rely on startup auto-migration.
- PostgreSQL must remain persistent across app rollouts.
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
  - PostgreSQL
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

### Recommended defaults

Use these values unless the user chooses different ones:

- namespace: `castaway`
- web Service name: `castaway-web`
- Postgres Service name: `castaway-postgres`
- bot Deployment name: `castaway-discord-bot`
- migration Job name: `castaway-web-migrate`
- config objects:
  - `castaway-web-config`
  - `castaway-discord-bot-config`
- secret objects:
  - `castaway-web-secrets`
  - `castaway-discord-bot-secrets`
  - `castaway-postgres-secrets`

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
- deployment-related docs under `plans/` only when explicitly assigned

**Deliverables:**

- `deploy/base/postgres`
- `deploy/base/castaway-web`
- `deploy/base/castaway-discord-bot`
- `deploy/environments/home-k3s`
- `deploy/argocd`
- dedicated `castaway-web` migration Job manifest
- overlay patch that turns the migration Job into an Argo CD `PreSync` hook or equivalent ordered sync step
- web Deployment configured for startup auto-migration disabled in cluster
- bot Deployment configured for one replica and `Recreate` semantics while file-backed state remains

**Dependencies:**

- locked assumptions above
- migration command contract once finalized in `apps/castaway-web`

**Parallelizable with:**

- Workstreams B, C, D, E

**Validation expectation:**

- render the Kustomize bases and `home-k3s` overlay
- if no render task exists yet, add a minimal repeatable render check instead of relying on manual eyeballing

### Workstream B — GitHub Actions image publishing and digest updates

**Owner repo:** `srvivor`

**Primary file ownership:**

- `.github/workflows/**`
- helper scripts used only by those workflows

**Deliverables:**

- workflow to build and publish changed app images to GHCR
- workflow or script path to update image digests in `deploy/environments/home-k3s`
- path filtering so unchanged apps do not rebuild unnecessarily
- immutable image reference flow suitable for Argo CD consumption

**Dependencies:**

- locked assumptions above
- deployment path shape from Workstream A

**Parallelizable with:**

- Workstreams A, C, D, E

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

**Dependencies:**

- locked assumptions above

**Parallelizable with:**

- Workstreams A, B, D, E

**Validation expectation:**

Run at least:

```bash
cd apps/castaway-web
mise run lint
mise run test
mise run build
```

If request-path behavior changes materially, also run the regression suite or document why it was deferred.

### Workstream D — `castaway-discord-bot` production client and state backend

**Owner repo:** `srvivor`

**Primary file ownership:**

- `apps/castaway-discord-bot/**`

**Why this stays one stream:**

The bot auth client work and the PostgreSQL state backend both naturally touch bot config and runtime wiring. Keeping them in one stream avoids avoidable merge conflicts in `internal/config/config.go` and related startup code.

**Deliverables:**

- bot-to-API auth header support
- bot config for service auth and bot database access
- PostgreSQL-backed state store using the bot's own logical database
- explicit migration/import path from BoltDB if existing saved defaults must be preserved
- updated bot docs and operational notes

**Dependencies:**

- locked assumptions above

**Parallelizable with:**

- Workstreams A, B, C, E

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

**Primary file ownership:**

- `/Users/brain/dev/infra/selfhost/**`
- `/Users/brain/dev/infra/docs/plans/**`
- any supporting infra scripts/config owned by that repo

**Deliverables:**

- self-hosted k3s VM/bootstrap path
- Tailscale reachability for the VM
- Argo CD installation/bootstrap path
- unattended 1Password service-account path for secret reads
- first secret bridge into Kubernetes
- private access path to Traefik over the tailnet
- later tunnel readiness for `castaway.bry-guy.net`

**Dependencies:**

- infra plan inputs already documented

**Parallelizable with:**

- Workstreams A, B, C, D

**Validation expectation:**

- use the infra repo's `mise` tasks and smoke tests
- document operator verification steps for Tailscale access, secret materialization, and Argo CD readiness

## Suggested implementation order

Work can happen in parallel, but integration should prefer this order:

1. Workstream C
2. Workstream D
3. Workstream A
4. Workstream B
5. Workstream E
6. final integration and smoke validation

Why this order:

- app-level contracts land before deployment wiring depends on them
- deployment manifests land before automation starts mutating overlay digests
- infra bootstrap can proceed in parallel, but final cluster hookup is most useful once manifests and images are real

## Final integration checklist

One final integration pass should verify the combined result.

### In the app repo

- rebase all surviving workstreams onto current `main`
- resolve any remaining contract drift explicitly
- run repo-level CI
- render the `home-k3s` Kustomize overlay
- verify the web migration hook path is included and ordered correctly
- verify the bot stays single-replica until PostgreSQL state is complete and deployed

### In the infra repo

- verify Tailscale-only access path
- verify Kubernetes secrets exist before Argo CD syncs app workloads
- verify PostgreSQL persistence strategy is documented and understood
- verify no WAN exposure was introduced as part of bootstrap

## Agent handoff template

Each agent should hand back a short note with:

- files changed
- locked assumptions relied on
- checks run
- known follow-ups
- anything that blocks another workstream

## Compact handoff summary

- Treat the blueprint as design reference and this file as the executable plan.
- Preserve locked assumptions unless the user changes them.
- Split implementation by file ownership, not by vague feature labels.
- Keep `castaway-web` migration work in a dedicated production path from day one.
- Keep all bot runtime changes in one stream to avoid config conflicts.
- Keep infra work in the infra repo.
- Use a final integration pass to validate the combined system.
