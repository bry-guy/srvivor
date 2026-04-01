# Auth and Authorization Plan

Status: `done`

## Goal

Define the production authentication and authorization model for `castaway-web`.

## Current baseline

- bot-to-API traffic uses service-to-service bearer auth
- Discord user identity can now be forwarded to `castaway-web` via `X-Discord-User-ID`
- participant-private reads follow a three-tier visibility model:
  - public
  - linked self
  - admin

## Implemented auth slice

The first concrete end-user auth slice is now in progress / partially implemented through Discord-linked participants:

- participants can be linked to Discord users
- `bonus-ledger` is auth-aware instead of split into public/private endpoints
- `activity-history` is auth-aware and now suppresses secret bonus history for public callers
- the Discord bot keeps one-command semantics instead of adding `my*` or `admin*` aliases

## Resolution

The first production auth model is now fully deployed:

- Admin identity is DB-backed via `instance_admins` table (not config allowlist)
- Bot forwards Discord user identity to web via `X-Discord-User-ID` header
- Web enforces self/admin/public visibility per-request
- Link/unlink operations require instance admin authorization (403 for non-admins)
- No OAuth/login flow needed — Discord interaction identity + DB mapping covers gameplay goals

## Remaining open questions (future work)

- whether any direct human-facing API access is needed outside Discord (see `admin-cli-architecture.md`)
- what future write workflows require stronger authorization than current trusted-bot service auth
- how credential rotation and secret management should evolve in production

## Related threads

- `../../plans/discord-auth-plan.md` — concrete Discord-linked participant slice
- `service-to-service-authentication-planning.md` — self-hosted bearer-auth baseline
