# Public Castaway website

Status: `planning`

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
| probst (you) | tunnel → public listener `:8080`, `https://castaway.bry-guy.net/api` | **personal API token**: minted on the site's `/account` page after Discord login, bound to your Discord ID, stored hashed, revocable, sent as `Authorization: Bearer` |
| Discord bot | cluster Service → internal listener `:8081` | existing service token plus actor header; the tunnel never routes to `:8081` |

- The public listener never accepts the service token or the actor header. The actor always comes from the session or the personal token. Admin rights still come from `instance_admins`.
- The internal listener keeps today's behavior, so the bot does not change.
- probst replaces its `token` and `discord_user_id` config fields with a personal token and switches `api_url` to `…/api`. The tailnet Caddy route and DNS-only record for `castaway.bry-guy.net` are removed.
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
3. Personal API tokens (`/account` page to create and revoke), and a probst config migration.
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
| Listener split, `/api` prefix, personal tokens, probst migration | ~2 days |
| Discord login, sessions, CSRF, access requests | ~2 days |
| Read-only pages and hardening | ~1–2 days |

## Open questions

- A custom domain for Discord OAuth redirects is fine, but confirm the Discord application to use: reuse the bot's application or create a separate one.
- Keep a break-glass path for probst if Cloudflare is down, for example `kubectl port-forward` to `:8081` with the service token.
