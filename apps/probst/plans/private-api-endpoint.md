# Private HTTPS endpoint for Probst

Status: planning (no infrastructure change approved)

## Why

Probst currently needs `kubectl port-forward` because `castaway-web` is a `ClusterIP` service. Probst accepts HTTPS API URLs (HTTP only for loopback); local JSON configuration alone does not provide a network route.

## Read-only findings

- `castaway/castaway-web` exposes port 8080 internally as a `ClusterIP` service, with no external IP or Tailscale service annotation.
- The cluster has Ingress, Gateway/HTTPRoute, and Traefik IngressRoute APIs, but no instances of these resources were found. Traefik has no assigned LoadBalancer address. Tailscale `ProxyClass` and `Connector` resource types are not installed.
- Infra documents steady-state administration as `fedora@platform-control-0.tail9e13b.ts.net`. A read-only check there found `tailscale serve status --json` empty; Caddy is active and bound to the node's Tailscale address on 80/443.
- A filtered elevated read of the live Caddyfile found private HTTPS routes for `admin.bry-guy.net`, `home.bry-guy.net`, `pi.bry-guy.net`, and other apps, but no Castaway route. The infra generator `scripts/selfhost-homepage-caddy-apply.sh` already routes other ClusterIP services through Caddy, with Cloudflare DNS-01 TLS and DNS-only tailnet hostnames.
- The control node reached the current Castaway ClusterIP at `http://10.43.169.252:8080/healthz` with HTTP 200. This verifies node-to-service connectivity, not database readiness. `castaway.bry-guy.net` returned no CNAME or A record in the read-only lookup; naming and ACL ownership still need confirmation.

## Proposed smallest safe route (requires separate approval)

1. In infra's existing Caddy generator, add one `castaway.bry-guy.net` site following the existing tailnet-bound, Cloudflare DNS-01 TLS proxy pattern. Resolve the `castaway-web` Service IP and port at apply time, as the generator does for other services; never hardcode today's ClusterIP. Review the *entire generated Caddyfile* before applying because the task replaces that file, not just one site.
2. Add a DNS-only CNAME for the agreed hostname pointing to `platform-control-0.tail9e13b.ts.net`, following the existing private site pattern. No Tailscale Serve, new proxy, Kubernetes operator, public ingress, or Funnel is needed. Confirm tailnet ACLs restrict access to intended operators; DNS-only by itself is not authorization.
3. Keep the API service bearer authentication enabled and Probst's HTTPS certificate validation and redirect refusal. Verify tailnet TLS from the operator laptop, unauthenticated admin/session rejection, authenticated admin/session success without exposing credentials, and no off-tailnet reachability before setting `api_url` in the local config. `/healthz` alone does not prove the database works.
4. Roll back only the new Caddy site and DNS record using infra's established backup/restore path; return Probst to loopback port-forwarding. Preserve existing Caddy sites, bot-to-service traffic, and production database state.

**Open prerequisites:** approve the hostname, confirm DNS-01 certificate issuance and ACL ownership, and review the complete generated Caddy diff and rollback method before any infra apply. No change here is approved to run against production.
