# Castaway self-hosted k3s private ingress and secrets plan

## Goal

Provision the self-hosted infrastructure needed for Castaway to run on a k3s VM without exposing the home public IP, while keeping secrets sourced from 1Password and deployments compatible with Argo CD.

## Scope

This plan covers infrastructure-side concerns that the Castaway app repo should not own directly:

- k3s host reachability and admin access
- private ingress shape
- tunnel strategy for a friendly hostname later
- unattended secret access from 1Password
- Argo CD bootstrap expectations that coordinate with the app repo's deployment blueprint

## Locked-in constraints

- Do not expose the home public IP for Castaway.
- Do not depend on home-router port forwarding for app traffic.
- Keep the application VM reachable in the tailnet.
- Use 1Password `bry-guy` as the secret source of truth.
- Avoid interactive 1Password requirements during steady-state operations.

## Boundary with the Castaway app repo

The Castaway app repo should own:

- app manifests
- Services
- web ingress rules or route definitions
- Argo CD application manifests
- image update flow

This infra repo should own:

- the Proxmox VM and host substrate
- k3s installation/bootstrap path
- Tailscale connectivity for the host/VM
- any tunnel connector such as Cloudflare Tunnel
- the secret bridge from 1Password into Kubernetes
- storage/backups for persistent infrastructure concerns

## Recommended access model

### Phase 1 access path: tailnet only

First working access path:

- the Castaway k3s VM joins Tailscale
- Traefik handles HTTP routing inside the VM/cluster
- trusted devices reach the service over the tailnet
- no WAN exposure is required

This is enough to get private access without deciding the final public-facing hostname path.

### Phase 2 access path: outbound tunnel for friendly hostname

Long-term desired hostname:

- `castaway.bry-guy.net`

Recommended shape:

- provision Cloudflare Tunnel or an equivalent outbound-only tunnel
- publish `castaway.bry-guy.net` through that tunnel
- point the tunnel at Traefik or another internal ingress endpoint on the k3s VM
- keep the home IP private and unadvertised

That preserves the no-public-IP requirement while allowing a stable user-facing hostname later.

## Tunnel and ingress implications

The app workloads do not need to know about Cloudflare directly.

They only need:

- an internal Service for `castaway-web`
- an ingress route that Traefik can serve

Infra then decides whether that ingress is consumed by:

- Tailscale-only access
- Cloudflare Tunnel
- another private reverse-proxy path later

## Secret-delivery model

### Source of truth

Use 1Password `bry-guy` as the source of truth.

### Operational requirement

Cluster operations must be able to read needed secrets without an interactive `op signin` flow.

### Recommended shape

- create a dedicated 1Password service account for infrastructure/cluster secret reads
- store and manage that service-account material in the infra repo's existing secret workflow
- provision a cluster-side or admin-side secret sync path that materializes Kubernetes `Secret`s needed by Castaway

The exact bridge can be chosen later, but it must satisfy:

- unattended operation
- no plaintext secrets committed to git
- compatibility with Argo CD-managed app deployment

## Candidate secret-bridge approaches

These are options to evaluate, not final implementation decisions.

### Option A: bootstrap/apply sync job

- infra tooling reads from 1Password
- infra tooling writes Kubernetes `Secret`s directly

Pros:

- very simple
- no extra controller to run

Cons:

- less continuously reconciled
- more imperative than GitOps-style secret flows

### Option B: in-cluster secret operator or bridge

- run a controller/bridge in the cluster
- authenticate it with a 1Password service account
- let it maintain Kubernetes `Secret`s continuously

Pros:

- more self-healing
- cleaner for long-running operations

Cons:

- more moving parts
- requires careful bootstrap ordering

The first implementation can pick the simpler approach if it keeps the operational model understandable.

## Argo CD coordination expectations

Infra should document or automate enough bootstrap to make the app repo's Argo CD path viable.

Minimum expectations:

- k3s is installed
- Traefik behavior is known
- Argo CD is installed
- the namespace and permissions needed for Castaway are understood
- required secrets are present before workloads start

## Persistent data expectations

The app blueprint depends on persistent PostgreSQL storage.

Infra responsibilities should therefore also include:

- selecting the storage path/class used by k3s for the Postgres volume
- documenting backup and restore for the Postgres data path
- keeping database persistence independent from stateless app rollouts

## Execution phases

### Phase 1: substrate and cluster bootstrap

- provision the Castaway k3s VM on Proxmox
- ensure the VM joins Tailscale
- install k3s and verify Traefik behavior
- install Argo CD

### Phase 2: secret bridge

- create or confirm a dedicated 1Password service account
- wire it through the infra repo's secret management
- choose and implement the first secret materialization path into Kubernetes

### Phase 3: private service access

- validate tailnet-only access to the web ingress path
- keep WAN exposure at zero
- document the access path for operators

### Phase 4: friendly hostname

- provision Cloudflare Tunnel or equivalent outbound connector
- map `castaway.bry-guy.net` to the internal ingress path
- document DNS, tunnel ownership, and TLS responsibilities

## Compact handoff summary

- Keep Castaway private first: Tailscale access, no port forwarding, no public IP exposure.
- Let the app repo stay focused on manifests and Argo CD inputs.
- Let infra own the tunnel, Tailscale, k3s bootstrap, and 1Password secret bridge.
- Use 1Password `bry-guy` with a dedicated service account for unattended operations.
- Treat Cloudflare Tunnel as the preferred long-term hostname path for `castaway.bry-guy.net`, but not a prerequisite for the first private deployment.
