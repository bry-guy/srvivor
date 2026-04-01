# Discord User Authentication Plan

Status: `done`

## Goal

Allow Discord users to see their own private Castaway data through the existing bot commands, while keeping most read flows publicly runnable.

## Current decisions

- Keep existing bot-to-API bearer auth (`requireServiceAuth`).
- Flow Discord user identity through `castaway-web` via `X-Discord-User-ID`.
- Do **not** add duplicate `my*` or `admin*` commands.
- Existing commands return data based on caller privilege:
  - public caller → public/revealed data only
  - linked self → self-private data included
  - admin → full target participant data
- Leaderboards stay public-safe; private bonus visibility is handled through participant-targeted routes.

## Implemented slice

### Database / queries

- Added `participants.discord_user_id`
- Added unique per-instance Discord link index
- Added sqlc queries for:
  - get linked participant by instance + Discord user ID
  - set participant Discord user ID
  - clear participant Discord user ID

### API routes

Added:

- `GET /instances/:instanceID/participants/me`
- `PUT /instances/:instanceID/participants/:participantID/discord-link`
- `DELETE /instances/:instanceID/participants/:participantID/discord-link`

Updated existing routes to be auth-aware:

- `GET /instances/:instanceID/participants/:participantID/bonus-ledger`
  - public → visible entries only
  - linked self / admin → visible + secret entries
- `GET /instances/:instanceID/participants/:participantID/activity-history`
  - public → secret history impact excluded
  - linked self / admin → secret history impact included

### Bot behavior

Added:

- `/castaway link participant:<name> [instance] [season]`
- `/castaway unlink [instance] [season]`

Updated existing commands:

- `/castaway score participant:<name> [instance] [season]`
  - public callers see public-safe totals
  - linked self / admins see private totals for the target participant
- `/castaway history participant:<name> [instance] [season]`
  - public callers do not see secret bonus history
  - linked self / admins do

Private responses are sent ephemerally when the bot can determine the caller is linked self or admin.

## Resolution

All items in this plan are now deployed and verified live:

- Admin identity is DB-backed via `instance_admins` (not config allowlist)
- Live Discord behavior verified: link/unlink, self-score with secret bonus, ephemeral private responses
- The `self/admin/public` pattern is established and reusable for future participant-private routes
- Web auth plan (`apps/castaway-web/plans/auth-and-authorization-planning.md`) is updated and marked done
