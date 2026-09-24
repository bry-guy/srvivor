# Private HTTPS endpoint for Probst

Status: planning (no infrastructure change approved)

## Why

Probst currently needs `kubectl port-forward` because `castaway-web` is a `ClusterIP` service. Probst accepts HTTPS API URLs (HTTP only for loopback); local JSON configuration alone does not provide a network route.

## Read-only findings

- `castaway/castaway-web` exposes port 8080 internally as a `ClusterIP` service, with no external IP or Tailscale service annotation.
- The cluster has Ingress, Gateway/HTTPRoute, and Traefik IngressRoute APIs, but no instances of these resources were found. Tailscale `ProxyClass` and `Connector` resource types are not installed. An out-of-cluster proxy or route has **not** been ruled out.
- The laptop sees online tailnet peers `platform-control-0`, `platform-worker-0`, and `platform-worker-1`. This proves tailnet node connectivity, not API exposure or permission to change those nodes.

## Proposed smallest safe route (requires separate approval)

1. Ask the platform owner to check for an existing tailnet-only HTTPS reverse proxy; reuse it if it can route to the internal Castaway service without public DNS or ingress. Confirm the real HTTPS URL and certificate before configuring Probst.
2. If none exists, propose Tailscale Serve on an already tailnet-connected platform node, forwarding to a **node-local, loopback-only** proxy with a stable internal upstream to the `castaway-web` Service. Confirm the node actually has stable Kubernetes service reachability; do not hardcode the current ClusterIP or rely on an unattended `kubectl port-forward`. Do not install the Tailscale Kubernetes operator solely for this CLI without a separate decision.
3. Restrict tailnet ACLs to the intended operator identity, keep the API's service bearer authentication enabled, use a valid tailnet HTTPS certificate, preserve Probst's TLS verification and no-redirect policy, and avoid public listeners, Funnel, or identity-header-as-auth shortcuts.
4. Verify from the operator laptop that the tailnet HTTPS URL reaches health and authenticated admin/session endpoints without displaying credentials. Check that an unauthenticated request is rejected and that the URL is inaccessible off-tailnet. Only then set `api_url` in the local config and retire routine port-forwarding.
5. Roll back by removing only the new Tailscale Serve mapping/node-local proxy and returning the Probst URL to loopback port-forwarding; leave existing bot-to-service traffic and production database unchanged.

**Open prerequisites:** identify the node/proxy owner, stable upstream routing, proposed hostname, tailnet ACL policy and TLS availability. No manifest or command here is approved to run against production.
