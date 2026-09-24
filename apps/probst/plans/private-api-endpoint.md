# Private HTTPS endpoint for Probst

Status: in-progress (endpoint operational; effective ACL and off-tailnet access unverified)

## Deployed

- `https://castaway.bry-guy.net` routes through Caddy on `platform-control-0` to the `castaway/castaway-web` ClusterIP Service. The Discord bot still reaches the API internally; this route is for Probst.
- Infra `main` at `0e879e565cb3b6adc54016e211b8a4404c0145ff` declares the Caddy site and a DNS-only CNAME to `platform-control-0.tail9e13b.ts.net`. The Caddy generator resolves the Service IP and port at apply time. Its apply path validates the staged configuration before replacing the active file and retains a backup for rollback.
- Before deployment, the full generated Caddyfile matched the then-live file byte-for-byte except for the new Castaway site. The comparison is saved in `private-api-caddy-render.diff`; `private-api-infra.patch` records the original review-only patch. The active Caddyfile reached the expected candidate hash after apply.
- The Cloudflare plan and apply reported one CNAME added, zero changed, zero destroyed. Ordinary DNS resolves the hostname; trusted HTTPS (no address override or certificate bypass) returns `/healthz` 200 and unauthenticated `/admin/session` 401. Authenticated `probst auth status` succeeds through this URL without port-forwarding, using an existing service token injected into the process environment. Existing home, admin, and pi-sync sites remained healthy after the Caddy change.

## Remaining access review

The active listener is bound to the node's Tailscale address; DNS-only does not itself enforce authorization. Effective tailnet ACL policy and off-tailnet denial were not verified in this session because ACL-file reading was blocked by the host workspace boundary. Shared-IP port-443 ACLs cannot restrict this hostname separately from Caddy's other sites. Service bearer authentication remains enabled. No local Probst credential file was created or modified.

## Rollback

Revert only the Castaway route and DNS record through the reviewed infra tasks, checking the full generated Caddyfile and DNS plan for unrelated changes. If needed, restore the Caddy backup created before replacement. Probst can temporarily use loopback port-forwarding without changing the bot or database.
