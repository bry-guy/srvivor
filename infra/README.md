# Castaway selfhost app infra

This directory owns the Castaway-specific layer that runs on top of the shared
homelab platform from `~/dev/infra`.

The split is intentionally simple:

- `~/dev/infra` provisions and operates the shared platform: Proxmox VMs, k3s,
  Argo CD, shared PostgreSQL, backups, and platform kubeconfig.
- `~/dev/srvivor/infra` owns Castaway app wiring: Kubernetes secrets, Argo CD
  app/project manifests, and app-level verification.

This keeps app-specific secret material and Argo CD resources with the app repo,
while still coordinating with the platform repo through a kubeconfig file.

## Default coordination point

By default these scripts use the current replacement platform:

```text
~/.kube/selfhost-platform-7049-1.yaml
```

The name identifies homelab 7049's first platform cluster. The legacy
rollback cluster remains available through:

```text
~/.kube/selfhost-platform-us-west.yaml
```

The replacement file is produced by the platform repository's replacement k3s
workflow. You can override paths if needed:

```bash
export CASTAWAY_PLATFORM_INFRA_REPO_PATH=~/dev/infra
export CASTAWAY_SELFHOST_KUBECONFIG_PATH=~/.kube/selfhost-platform-7049-1.yaml
export CASTAWAY_SELFHOST_NAMESPACE=castaway
```

Legacy env names such as `SELFHOST_CASTAWAY_KUBECONFIG_PATH` are still accepted
so old command snippets remain recoverable.

## Normal app deploy wiring

From this repo, using the unified `castaway:*` verb taxonomy:

```bash
mise run castaway:platform:check       # platform prerequisites
mise run castaway:postgres:target:restore restore-castaway-postgres # restore Castaway DBs
mise run castaway:secrets:apply        # K8s secrets from 1Password
mise run castaway:argocd:apply         # Argo CD project/application manifests
mise run castaway:argocd:sync          # explicit retry after a failed migration hook
mise run castaway:check                # post-deploy verification
```

Or run the app-level sequence:

```bash
mise run castaway:apply
```

The old `infra:*` task names have been removed; use the canonical `castaway:*`
names above.

The published interface this app consumes from the shared platform is
documented in `~/dev/infra/docs/selfhost-platform-contract.md`. App repos
depend only on those names.

The target restore is intentionally app-owned and destructive only to the two
Castaway databases. It leaves unrelated platform databases and the platform
`postgres` credential unchanged. Run it only after the replacement PostgreSQL
backup/restore gates pass and before `castaway:apply`:

```bash
mise run castaway:postgres:target:restore restore-castaway-postgres
```

The restore task uses `platform-worker-1.tail9e13b.ts.net` as its default target
SSH host now that `tag:platform` authorizes Fedora Tailscale SSH. Override it
with `CASTAWAY_SELFHOST_POSTGRES_TARGET_SSH_HOST` when needed. This is only the
administrative SSH path; Castaway's PostgreSQL application endpoint remains the
separately configured LAN address and port.

If the migration hook previously failed before the database restore, request
one explicit retry after the restore:

```bash
mise run castaway:argocd:sync
```

After the Argo CD `Application` exists, normal Castaway releases should deploy
through GitOps: update/push the app manifests or image tags consumed by
`deploy/environments/home-k3s`, and Argo CD reconciles them.

## Required secrets

The `castaway:secrets:apply` task uses the `castaway-selfhost` fnox profile and
writes these Kubernetes secrets into the `castaway` namespace:

- `castaway-postgres-secrets`
- `castaway-web-secrets`
- `castaway-discord-bot-secrets`

Expected 1Password items live in the `castaway` vault and are declared in the
repo-local `fnox.toml`.

## Legacy recovery material

`infra/legacy-from-infra` contains the deleted Castaway-specific scripts/docs as
recovered from `~/dev/infra` before the platform/app split. Treat it as recovery
reference, not the preferred operating path.
