# Public Castaway website

Status: `in-progress`

## Goal

Serve a small player website at `https://castaway.bry-guy.net`, with the API at `/api`. This is the first publicly reachable homelab service. It starts read-only for the current season: scores, my draft, and tribes. Later it grows into draft submission and in-browser games such as Castawordle. The home IP is never exposed.

## Decisions

- **Ingress:** use a Cloudflare Tunnel. `cloudflared` runs in k3s and connects outbound only, so no router ports are opened. The DNS record is proxied (orange cloud).
- **One app:** `castaway-web` serves the HTML pages (`/`), the API (`/api`), and login (`/auth`) from one binary and one deploy.
- **Login:** Discord OAuth2 authorization-code flow with `golang.org/x/oauth2` and the `identify` scope. No self-hosted identity provider. Sessions are random IDs in an `HttpOnly; Secure; SameSite=Lax` cookie, stored hashed in a Postgres `web_sessions` table with expiry.
- **Who gets in:** only Discord users linked to a player in the current season. Everyone else sees a **Request access** page (see Access requests below).
- **Frontend:** server-rendered Go `html/template` plus HTMX, with web components only where interaction needs them. There is no build step and no JS framework. Static assets are embedded with `embed`.
- **Scope:** the current season only. The site reads the instance from config; there is no season switcher.

## Everything under public `/api`: auth model

Today the API trusts a service bearer token plus an `X-Discord-User-ID` header. Once the API is public, a leaked token would let anyone act as any user, including admins. Split this into two listeners and three credential types:

| Caller | Reaches the API via | Credential |
|---|---|---|
| Browser (players) | tunnel → public listener `:8080` | session cookie; CSRF token on writes |
| probst (you) | tunnel → public listener `:8080`, `https://castaway.bry-guy.net/api` | **Discord login** via `probst login` (below); gets a CLI session token bound to your Discord ID, stored hashed, expiring, revocable, sent as `Authorization: Bearer` |
| Discord bot | cluster Service → internal listener `:8081` | existing service token plus actor header; the tunnel never routes to `:8081` |

- The public listener never accepts the service token or the actor header. The actor always comes from the session or the personal token. Admin rights still come from `instance_admins`.
- The internal listener keeps today's behavior, so the bot does not change.
- Admins are Discord users listed in `instance_admins` (already how admin rights work). Adding one is `probst admin add USER --instance I`, which admins can run.
- probst drops its `token` and `discord_user_id` config fields, gains `probst login` / `logout`, and switches `api_url` to `…/api`. The tailnet Caddy route and DNS-only record for `castaway.bry-guy.net` are removed.
- Paths move under `/api` (for example `/api/instances/...`). The OpenAPI spec and Hurl tests follow. The bot's internal client keeps the old paths on `:8081`, or both listeners mount the same router under a prefix; decide at implementation.

## Access requests

Emailing `root@bry-guy.net` would mean adding SMTP delivery and bounce and deliverability handling. Instead, use the Discord pipeline that already exists:

1. A signed-in but unlinked user clicks **Request access**. A row is stored with their Discord ID, username, and time, limited to one pending request per user.
2. The bot DMs the admin contact (`CASTAWAY_ADMIN_CONTACT_DISCORD_USER_ID`, which the draft watcher already uses) with the requester's details.
3. Approve with `probst access approve USER --as PLAYER` (links to an existing player or creates one), or deny with `probst access deny USER`. An admin page on the site can come later.

If you want email too, add it later as a second notifier through a transactional email API such as Resend or Postmark. It is not needed for v1.

## Infrastructure (`~/dev/infra`)

1. **Tunnel (Terraform in `cloudflare/`):** `cloudflare_zero_trust_tunnel_cloudflared` plus its config (ingress `castaway.bry-guy.net` → `http://castaway-web.castaway.svc:8080`, default `http_status:404`), and a proxied CNAME to `<tunnel-id>.cfargotunnel.com`. This replaces the current DNS-only record.
2. **`cloudflared` in k3s:** a Deployment in its own namespace with 2 replicas and the tunnel token from 1Password via the existing secrets flow. A NetworkPolicy allows egress only to Cloudflare and to `castaway-web:8080`.
3. **Cloudflare edge (free tier):** always HTTPS, a WAF managed ruleset, a rate-limiting rule on `/auth/*` and `/api/*`, and Bot Fight Mode.
4. **Caddy:** remove the Castaway site block. Probst traffic goes through the tunnel from then on.

## App work (`castaway-web`)

1. Public/internal listener split and `/api` prefix. Keep existing regression tests, add tests proving `:8080` rejects the service token and actor header.
2. Discord OAuth login, logout, sessions, and CSRF middleware. Register the Discord app's redirect `https://castaway.bry-guy.net/auth/callback`. Store the client ID and secret in 1Password.
3. `probst login`: probst listens on `127.0.0.1:<random port>` and opens `https://castaway.bry-guy.net/auth/cli?port=…&state=…`. The site runs the normal Discord login, then redirects the browser to the loopback with a one-time code. probst exchanges the code (`POST /api/auth/cli/exchange`) for a CLI session token and saves it in `~/.config/probst/config.json` (0600). The Discord client secret never leaves the server, and Discord only needs the one site redirect URI registered. This is the same pattern `gcloud`/`az` use. `probst logout` revokes the token. Revocation of all sessions for a user is an admin command.
4. Access requests: table, page, bot DM, and `probst access approve|deny`.
5. Pages: leaderboard (tribes, draft+bonus), my draft, tribes. Responses set a CSP, `X-Frame-Options`, and security headers.
6. Docs: README, requirements, readiness checklist, and a runbook covering tunnel rotation, revoking all sessions, and rolling back to tailnet-only.

## Games later

A game is a web component that renders its own grid or canvas and posts moves with `fetch` or HTMX. The server holds the solution and scores every move, so the client never sees the answer:

- **Castawordle:** a 6×5 grid plus an on-screen keyboard. The server checks each guess and returns tile colors.
- **Word search:** a letter grid where the player drags to select. The server validates selected coordinates.
- **Prisoner's dilemma:** buttons plus a scheduled reveal. It is plain HTMX with no component.

Revisit the frontend stack only for real-time or animation-heavy games (for example multiplayer at 60 fps). This is deferred until one is actually wanted.

## Effort

| Piece | Estimate |
|---|---|
| Tunnel, DNS, cloudflared, NetworkPolicy, edge rules | ~1 day |
| Listener split, `/api` prefix, `probst login`, admin command | ~2 days |
| Discord login, sessions, CSRF, access requests | ~2 days |
| Read-only pages and hardening | ~1–2 days |

## Decisions (continued)

- Discord application: reuse the bot's existing application (Jeff Probst) for OAuth, adding the redirect URI and client secret. A separate application stays an option if isolation is ever wanted.

## Break-glass

If Cloudflare is down, `kubectl port-forward` to `:8081` and use the service token (`probst --server http://127.0.0.1:8081` with `PROBST_TOKEN`).

## Progress

Done (commit `c6245d1` and follow-ups; castaway-web CI and the full integration suite pass):

- castaway-web public listener (`PUBLIC_PORT`, default off). It serves `/`, `/auth/login|cli|callback|logout`, `/access-request`, and `/api/*`. `/api` accepts only sessions (cookie or probst bearer); the service token and caller-supplied `X-Discord-User-ID` are ignored. Non-admins reach only an allowlist of read routes. Bot queues and bootstrap return 404. Cookie writes require a same-origin `Origin`.
- Migration 018: `web_sessions`, `web_cli_codes`, `access_requests` (tokens stored as SHA-256).
- `probst login` / `logout` (loopback redirect), `probst admin add`, `probst access list|approve|deny`.
- The bot DMs the admin contact once per access request.
- Deploy: container and Service port `public` 8090. Secret sync passes `DISCORD_OAUTH_CLIENT_SECRET` when `CASTAWAY_DISCORD_OAUTH_CLIENT_SECRET` is set.
- Test `TestPublicListener` runs against a fake Discord.

Go-live steps (need you; nothing applied yet):

1. Discord developer portal → Jeff Probst application → OAuth2: add redirect `https://castaway.bry-guy.net/auth/callback` and copy the client secret into 1Password as `CASTAWAY_DISCORD_OAUTH_CLIENT_SECRET` (plus an fnox entry). Run `castaway:secrets:apply`.
2. Add to `web-configmap.yaml`: `PUBLIC_PORT: "8090"`, `PUBLIC_BASE_URL: https://castaway.bry-guy.net`, `DISCORD_OAUTH_CLIENT_ID: <application id>`, `PUBLIC_INSTANCE_ID: <Season 51 public UUID>`. Deploy. The pod refuses to start if the secret is missing.
3. Infra (`~/dev/infra`, commit `76aa05f`, off by default): give the Cloudflare API token Account → Cloudflare Tunnel → Edit, set `castaway_public = true`, and run `mise run selfhost:cloudflare-dns:plan`. Review the plan: it should create the tunnel and its config and change the castaway record to proxied. Apply. Store `tofu output -raw castaway_tunnel_token` as `CASTAWAY_CLOUDFLARED_TOKEN` (1Password + fnox), then run `castaway:secrets:apply`.
4. Add `- public` to the home-k3s kustomization resources (`deploy/environments/home-k3s/public/`: cloudflared with 2 replicas, plus NetworkPolicies limiting cloudflared egress and castaway-web's 8090 ingress). Argo syncs it.
5. Cloudflare dashboard (free): Always Use HTTPS, WAF managed rules, a rate-limit rule on `/auth/*` and `/api/*`, Bot Fight Mode. Remove the Castaway site from the platform Caddy (`scripts/selfhost-homepage-caddy-apply.sh`).
6. `probst login` with `api_url: https://castaway.bry-guy.net/api`. The service token and `discord_user_id` then come out of the probst config.

Not done yet: HTMX and web components (the page is a plain server-rendered table for now), my-draft and tribes pages, rate limiting inside the app (Cloudflare handles it at the edge), session cleanup job.

### Tailnet preview (current state)

- The tailnet Caddy (`~/dev/infra` `0c09897`) sends `/`, `/auth/*`, `/access-request`, `/api/*` on castaway.bry-guy.net to castaway-web:8090 and everything else to 8080. probst's current config keeps working unchanged.
- `/` returns 502 until the public listener is enabled. That needs `CASTAWAY_DISCORD_OAUTH_CLIENT_SECRET` (step 1), then the configmap (step 2).
- The Cloudflare tunnel is blocked: the API token lacks Account → Cloudflare Tunnel → Edit. Once it has that, apply with `castaway_tunnel = true` (the DNS record stays on the tailnet), store the token, and add `public/` to the kustomization.
- There's no ingress NetworkPolicy on 8090: the tailnet Caddy reaches it from the host, and 8080 (more privileged) is already unrestricted.
