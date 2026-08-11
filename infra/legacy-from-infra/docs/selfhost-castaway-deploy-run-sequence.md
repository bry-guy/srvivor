# Castaway selfhost deploy run sequence

This is the practical bring-up sequence for the first Castaway rollout on the shared selfhost platform.

It is written so the operator or this agent can run it directly from `~/dev/infra`.

## What this sequence assumes

This sequence assumes the following are already true:

- the reusable Proxmox templates already exist:
  - `3000` service
  - `4000` stateful
  - `5000` appliance
- `~/dev/srvivor` exists locally and contains the Castaway Argo CD manifests
- required 1Password/fnox secrets are present
- `SELFHOST_CASTAWAY_POSTGRES_RESTIC_REPOSITORY` is set to a real restic repository

This sequence does **not** create the Fedora templates themselves.

## One-command path

From `~/dev/infra`:

```bash
mise run "selfhost:castaway:deploy"
```

That task runs, in order:

1. `selfhost:castaway:preflight`
2. `selfhost:us-west:apply`
3. `selfhost:castaway:bootstrap`
4. `selfhost:castaway:postgres:bootstrap`
5. `selfhost:castaway:postgres:backup:install`
6. `selfhost:castaway:postgres:backup`
7. `selfhost:castaway:secrets:sync`
8. `selfhost:castaway:argocd:app:apply`
9. `selfhost:castaway:verify`

## Step-by-step version

### 1. Preflight

```bash
mise run "selfhost:castaway:preflight"
```

Checks:

- `tofu validate` passes for `selfhost/regions/us-west`
- required Proxmox template VMs exist
- `srvivor` repo path and Argo CD manifests exist
- required secret inputs are available through `fnox`
- restic repository is configured

### 2. Provision the shared VMs in Proxmox

```bash
mise run "selfhost:us-west:apply"
```

Expected result:

- shared appliance VM exists
- shared service VM exists
- shared stateful VM exists

### 3. Bootstrap the cluster substrate

```bash
mise run "selfhost:castaway:bootstrap"
```

Expected result:

- appliance VM is running k3s server
- service VM joined as k3s agent
- kubeconfig fetched locally
- node labels applied
- Argo CD installed
- Kubernetes secrets synchronized

### 4. Bootstrap external PostgreSQL

```bash
mise run "selfhost:castaway:postgres:bootstrap"
```

Expected result:

- PostgreSQL is installed on the stateful VM via Podman/systemd
- Castaway DB roles and databases exist

### 5. Install and test backups

```bash
mise run "selfhost:castaway:postgres:backup:install"
mise run "selfhost:castaway:postgres:backup"
```

Expected result:

- daily backup timer installed
- one successful backup pushed to restic
- retention policy applied

### 6. Re-sync Kubernetes secrets

```bash
mise run "selfhost:castaway:secrets:sync"
```

Do this after PostgreSQL bootstrap so the cluster has the final external DB URLs and credentials materialized.

### 7. Apply the app-repo Argo CD resources

```bash
mise run "selfhost:castaway:argocd:app:apply"
```

Expected result:

- `project-castaway.yaml` applied
- `app-home-k3s.yaml` applied
- Argo CD begins reconciling the `home-k3s` overlay from `srvivor`

### 8. Verify the platform state

```bash
mise run "selfhost:castaway:verify"
```

Checks:

- cluster nodes present and labeled
- `argocd` and `castaway` namespaces exist
- Argo CD pods exist
- Castaway secrets exist
- Argo CD application objects exist
- PostgreSQL service is up on the stateful VM
- backup timer exists on the stateful VM

## If preflight fails

Most likely causes:

- one or more of template VMs `3000/4000/5000` do not exist yet
- `SELFHOST_CASTAWAY_POSTGRES_RESTIC_REPOSITORY` is still blank
- `~/dev/srvivor` is not present at `SELFHOST_CASTAWAY_SRVIVOR_REPO_PATH`
- required 1Password secret items have not been created/populated yet

## First-live-rollout note

This sequence gets the substrate, cluster, database, secret materialization, and Argo CD app wiring into place.

After that, still expect a first-live validation pass for:

- image pull success from GHCR
- app health
- migration hook success
- private ingress reachability
- application-level connectivity to external PostgreSQL
