# Private HTTPS endpoint for Probst

Status: planning (infra patch prepared locally; no deployment approved)

## Why

Probst currently needs `kubectl port-forward` because `castaway-web` is a `ClusterIP` service. Probst accepts HTTPS API URLs (HTTP only for loopback); local JSON configuration alone does not provide a network route.

## Read-only findings

- `castaway/castaway-web` exposes port 8080 internally as a `ClusterIP` service, with no external IP or Tailscale service annotation.
- The cluster has Ingress, Gateway/HTTPRoute, and Traefik IngressRoute APIs, but no instances of these resources were found. Traefik has no assigned LoadBalancer address. Tailscale `ProxyClass` and `Connector` resource types are not installed.
- Infra documents steady-state administration as `fedora@platform-control-0.tail9e13b.ts.net`. A read-only check there found `tailscale serve status --json` empty; Caddy is active and bound to the node's Tailscale address on 80/443.
- A filtered elevated read of the live Caddyfile found private HTTPS routes for `admin.bry-guy.net`, `home.bry-guy.net`, `pi.bry-guy.net`, and other apps, but no Castaway route. The infra generator `scripts/selfhost-homepage-caddy-apply.sh` already routes other ClusterIP services through Caddy, with Cloudflare DNS-01 TLS and DNS-only tailnet hostnames.
- The control node reached the current Castaway ClusterIP at `http://10.43.169.252:8080/healthz` with HTTP 200. This verifies node-to-service connectivity, not database readiness. `castaway.bry-guy.net` returned no CNAME or A record in the read-only lookup; naming and ACL ownership still need confirmation.
- `private-api-infra.patch` is a review-only patch against infra's Caddy generator, DNS resource/variable, and DNS README; it has **not** been applied to infra. `private-api-caddy-render.diff` compares complete baseline and candidate Caddyfiles rendered locally from the same read-only, 44-service discovery snapshot. Removing the single new site from the candidate reproduces the baseline byte-for-byte. The generated baseline SHA-256 (`148b805cc7fe3fae169f93185448023b6f89bab031349417d3950c66e70249ad`) matched the live Caddyfile hash at inspection time. This does not guarantee live state remains unchanged at a future apply.

## Proposed smallest safe route (requires separate approval)

1. In infra's existing Caddy generator, add one `castaway.bry-guy.net` site following the existing tailnet-bound, Cloudflare DNS-01 TLS proxy pattern. Resolve the `castaway-web` Service IP and port at apply time, as the generator does for other services; never hardcode today's ClusterIP. Review the *entire generated Caddyfile* before applying because the task replaces that file, not just one site.
2. Add a DNS-only CNAME for the agreed hostname pointing to `platform-control-0.tail9e13b.ts.net`, following the existing private site pattern. No Tailscale Serve, new proxy, Kubernetes operator, public ingress, or Funnel is needed. Check the effective tailnet ACLs and intended access: shared-IP port-443 ACLs cannot restrict this hostname separately from the other Caddy sites. DNS-only by itself is not authorization.
3. Keep the API service bearer authentication enabled and Probst's HTTPS certificate validation and redirect refusal. Verify tailnet TLS from the operator laptop, unauthenticated admin/session rejection, authenticated admin/session success without exposing credentials, and no off-tailnet reachability before setting `api_url` in the local config. `/healthz` alone does not prove the database works.
4. Roll back by reverting only the proposed infra Caddy/DNS additions, reviewing the resulting full Caddyfile against the captured baseline, and restoring the previous Caddyfile if the generator diverges. Return Probst to loopback port-forwarding. Preserve existing sites, bot-to-service traffic, and production database state.

The route serves **castaway-web for Probst**, not the Discord bot; the bot continues to reach the API internally. Patch application to temporary copies, `bash -n`, Terraform formatting, and generated baseline/candidate comparison passed. Local Caddy semantic validation was unavailable (no local Caddy binary); certificate issuance, effective ACLs, and end-to-end API authentication require separate deployment-time verification. No Caddy apply/reload, DNS change, token resolution, or Kubernetes mutation has occurred.

**Open prerequisites:** confirm the hostname and shared-port access policy, then review the patch, full generated diff, TLS behavior, and rollback before any infra apply.
