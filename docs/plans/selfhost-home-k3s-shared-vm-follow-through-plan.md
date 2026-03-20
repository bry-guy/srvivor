# Self-hosted home-k3s shared-VM follow-through plan

Status: `done`

## Goal

Adapt the Castaway app repo's `home-k3s` deployment contract to the shared-VM target where PostgreSQL is external to Kubernetes and owned by infra.

## Outcome

Completed app-repo follow-through:

- `deploy/environments/home-k3s` now represents the active external-PostgreSQL target
- in-cluster PostgreSQL Kubernetes manifests were removed from this repo
- web, bot, and migration workloads are pinned to service-labeled nodes via `selfhost.bry-guy.net/role=service`
- the existing Argo CD application path remains `deploy/environments/home-k3s`
- CI now validates the `home-k3s` overlay render so delivery automation stays aligned with the overlay Argo CD watches
- selfhost docs now describe the external-PostgreSQL/shared-VM contract

## Implementation notes

The repo intentionally kept a single active selfhost overlay instead of adding parallel in-cluster and external-PostgreSQL overlays.

Why:

- nothing was deployed yet, so preserving the in-cluster variant added churn without value
- a single active overlay avoids delivery automation drift
- the publish flow, digest pinning, and Argo CD target stay pointed at the same path

## Files changed

Deployment:

- `deploy/environments/home-k3s/kustomization.yaml`
- `deploy/environments/home-k3s/web-node-placement-patch.yaml`
- `deploy/environments/home-k3s/web-migration-node-placement-patch.yaml`
- `deploy/environments/home-k3s/bot-node-placement-patch.yaml`
- removed `deploy/base/postgres/**`
- removed `deploy/environments/home-k3s/postgres-init-configmap.yaml`
- removed `deploy/environments/home-k3s/postgres-patch.yaml`

Automation:

- `.github/workflows/ci.yml`

Docs:

- `docs/selfhost-k3s-deployment-blueprint.md`
- `docs/guides/selfhost-home-k3s-operators-guide.md`
- `docs/production-readiness-checklist.md`
- `plans/selfhost-k3s-implementation-plan.md`

## Verification target

Minimum deploy-shape verification for this contract:

```bash
kubectl kustomize deploy/environments/home-k3s > /tmp/home-k3s-render.yaml
```

Broader repo validation should continue to use:

```bash
mise run ci
```
