# Castaway self-hosted k3s bootstrap

This document describes the current infra-side bootstrap flow for running
Castaway on the shared selfhost platform.

It intentionally keeps WAN exposure at zero and is still focused on
**cluster/bootstrap concerns**.

External PostgreSQL on the shared stateful VM is now documented separately in:

- `docs/selfhost-castaway-postgres-operations.md`

## Operating model

- Proxmox and static substrate stay in Terraform/OpenTofu.
- Shared VM identities are modeled in Terraform.
- The preferred cluster shape is:
  - shared **appliance VM** = k3s server/control-plane
  - shared **service VM** = k3s agent for stateless workloads
  - shared **stateful VM** = reserved for persistent infrastructure in a later
    phase
- Argo CD runs inside the cluster.
- The first secret bridge is an **admin-side sync task** from 1Password into
  Kubernetes `Secret`s.
- The Castaway app repo owns app manifests and Argo CD Application resources.
- This infra repo owns bootstrap, access, and secret materialization.

## Networking stance

Use:

- Tailscale for operator/admin reachability
- LAN / `vmbr0` for east-west VM traffic where practical

Current implementation note:

- the service-node join task can default to the appliance VM’s Tailscale name
  for `K3S_URL`
- prefer overriding that with a LAN-reachable `SELFHOST_CASTAWAY_K3S_SERVER_URL`
  when the final LAN addressing is known

## Why the first secret bridge is admin-side

The first implementation chooses the simpler path from
`docs/plans/castaway-selfhost-k3s-private-ingress-and-secrets-plan.md`:

- infra tooling reads secrets from 1Password via fnox
- infra tooling writes Kubernetes `Secret`s directly

This keeps moving parts low while the first self-hosted deployment is being
brought up.

Trade-off:

- it is more imperative than a continuously reconciling in-cluster operator
- secret sync must be rerun when secret values change

That is acceptable for the first private deployment.

## Required 1Password items

### `bry-guy` vault (infra-owned)

- `SELFHOST_CASTAWAY_VM_SSH_KEY/private key`
- `SELFHOST_CASTAWAY_VM_SSH_KEY/public key`
- `TAILSCALE_AUTHKEY/password`
- `SELFHOST_CASTAWAY_POSTGRES_RESTIC_PASSWORD/password`

### `castaway` vault (shared with srvivor)

- `CASTAWAY_POSTGRES_SUPERUSER_PASSWORD/password`
- `CASTAWAY_WEB_DB_PASSWORD/password`
- `CASTAWAY_DISCORD_BOT_DB_PASSWORD/password`
- `CASTAWAY_DISCORD_BOT_TOKEN/password`
- `CASTAWAY_DISCORD_APPLICATION_ID/password`
- `CASTAWAY_DISCORD_PUBLIC_KEY/password`
- `DISCORD_BRAINLAND_SERVER_ID/password`
- `CASTAWAY_BOT_API_TOKEN/password`

## Expected non-secret config

Configured through `mise.toml`:

### Shared platform VM defaults

- `SELFHOST_SERVICE_TEMPLATE_VM_ID`
- `SELFHOST_STATEFUL_TEMPLATE_VM_ID`
- `SELFHOST_APPLIANCE_TEMPLATE_VM_ID`
- `SELFHOST_VM_SHARED_SERVICE_0_ID`
- `SELFHOST_VM_SHARED_STATEFUL_0_ID`
- `SELFHOST_VM_SHARED_APPLIANCE_0_ID`

### Castaway cluster bootstrap defaults

- `SELFHOST_CASTAWAY_APPLIANCE_VM_HOST`
- `SELFHOST_CASTAWAY_APPLIANCE_VM_SSH_USER`
- `SELFHOST_CASTAWAY_APPLIANCE_VM_TAILNET_HOSTNAME`
- `SELFHOST_CASTAWAY_SERVICE_VM_HOST`
- `SELFHOST_CASTAWAY_SERVICE_VM_SSH_USER`
- `SELFHOST_CASTAWAY_SERVICE_VM_TAILNET_HOSTNAME`
- `SELFHOST_CASTAWAY_K3S_SERVER_URL`
- `SELFHOST_CASTAWAY_APPLIANCE_NODE_NAME`
- `SELFHOST_CASTAWAY_SERVICE_NODE_NAME`
- `SELFHOST_CASTAWAY_KUBECONFIG_PATH`
- `SELFHOST_CASTAWAY_NAMESPACE`
- `SELFHOST_CASTAWAY_ARGOCD_VERSION`
- `SELFHOST_CASTAWAY_POSTGRES_HOST`
- `SELFHOST_CASTAWAY_POSTGRES_PORT`
- `SELFHOST_CASTAWAY_POSTGRES_SUPERUSER`
- optional legacy `SELFHOST_CASTAWAY_POSTGRES_SERVICE_HOST`

## Bootstrap flow

### 1. Bootstrap the appliance VM with Tailscale + k3s server

```bash
mise run "selfhost:castaway:k3s:bootstrap"
```

What it does:

- SSHes into the shared appliance VM
- installs/enables Tailscale if missing
- joins the appliance VM to the tailnet
- installs k3s server if missing
- labels the server node with `selfhost.bry-guy.net/role=appliance`
- leaves Traefik enabled
- keeps service exposure private to the tailnet / cluster

### 2. Fetch a kubeconfig that points at the appliance tailnet host

```bash
mise run "selfhost:castaway:kubeconfig:fetch"
```

What it does:

- copies `/etc/rancher/k3s/k3s.yaml` from the appliance VM
- rewrites the server endpoint from `127.0.0.1` to the appliance VM’s Tailscale
  hostname
- writes the result to `SELFHOST_CASTAWAY_KUBECONFIG_PATH`

### 3. Join the shared service VM as a k3s agent

```bash
mise run "selfhost:castaway:k3s:service:join"
```

What it does:

- reads the k3s node token from the appliance VM
- SSHes into the shared service VM
- installs/enables Tailscale if missing
- joins the service VM to the tailnet
- installs `k3s-agent` if missing
- joins the service VM to the appliance-led cluster
- labels the joined node with `selfhost.bry-guy.net/role=service`

Current note:

- the join task defaults to `SELFHOST_CASTAWAY_K3S_SERVER_URL`
- prefer setting that to a LAN-reachable endpoint for steady-state operations

### 4. Apply standard node labels

```bash
mise run "selfhost:castaway:k3s:node-labels"
```

What it does:

- applies or refreshes the standard appliance/service node labels using
  `kubectl`
- prints node labels for verification

### 5. Install Argo CD into the cluster

```bash
mise run "selfhost:castaway:argocd:bootstrap"
```

What it does:

- creates the `argocd` namespace
- installs the pinned Argo CD manifest version
- waits for the core Argo CD deployments to come up

This task does **not** apply the Castaway app repo’s Argo CD Application
resources. That remains app-repo-owned.

### 6. Materialize Castaway secrets into Kubernetes

```bash
mise run "selfhost:castaway:secrets:sync"
```

What it does:

- creates the `castaway` namespace if needed
- writes `castaway-postgres-secrets`
- writes `castaway-web-secrets`
- writes `castaway-discord-bot-secrets`

Current secret contract:

### `castaway-postgres-secrets`

- `postgres-superuser`
- `postgres-superuser-password`
- `castaway-web-db-user`
- `castaway-web-db-password`
- `castaway-discord-bot-db-user`
- `castaway-discord-bot-db-password`

### `castaway-web-secrets`

- `DATABASE_URL`
- `SERVICE_AUTH_BEARER_TOKENS`

### `castaway-discord-bot-secrets`

- `CASTAWAY_DISCORD_BOT_TOKEN`
- `CASTAWAY_DISCORD_APPLICATION_ID`
- `CASTAWAY_DISCORD_PUBLIC_KEY`
- `DISCORD_TARGET_SEVER_ID`
- `CASTAWAY_API_AUTH_TOKEN`
- `BOT_STATE_DATABASE_URL`

The K8s key is generic; infra sources it from the shared vault item `DISCORD_BRAINLAND_SERVER_ID/password` by default.

## Convenience path

```bash
mise run "selfhost:castaway:bootstrap"
```

Current dependency chain:

1. bootstrap appliance server
2. fetch kubeconfig
3. join service node
4. apply node labels
5. install Argo CD
6. sync secrets

## End-to-end deploy path

Once the Proxmox templates already exist and the required secrets are populated,
you can run the full infra-side bring-up from:

```bash
mise run "selfhost:castaway:deploy"
```

That sequence is documented in:

- `docs/selfhost-castaway-deploy-run-sequence.md`

## Operator checks

After bootstrap, verify:

```bash
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes -o wide
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get nodes -L selfhost.bry-guy.net/role
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl get ns
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n argocd get pods
KUBECONFIG="$SELFHOST_CASTAWAY_KUBECONFIG_PATH" kubectl -n castaway get secrets
```

## Manual vs automated

### Automated now

- appliance bootstrap with Tailscale + k3s server
- kubeconfig fetch and rewrite
- service-node join path
- appliance/service node labels
- Argo CD install
- first secret sync into Kubernetes

### Still manual or repo-external

- provisioning the actual shared VMs and templates on Proxmox if they do not
  already exist
- selecting the final Fedora atomic image build path for the new server families
- applying the Castaway app repo’s Argo CD Application resources
- first Cloudflare Tunnel provisioning
- app-repo Argo CD resources and overlays in `srvivor`

External PostgreSQL installation/automation and backup/restore are covered in:

- `docs/selfhost-castaway-postgres-operations.md`

## Zero-WAN-exposure note

These tasks intentionally avoid:

- home-router port forwarding
- public LoadBalancer assumptions
- exposing Argo CD or Traefik over the public internet

Initial access is through:

- Tailscale to the shared VMs
- kubeconfig over the tailnet
- Kubernetes Services / ingress inside the private cluster
