# Discord Guild Configuration Planning

Status: planning

## Goal

Support multiple Discord guilds with independent Castaway bot configuration, announcement routing, and guild-scoped operational state.

## Current state

- The bot has a single global target guild via `DISCORD_TARGET_SEVER_ID`.
- Public announcements use a single global `CASTAWAY_ANNOUNCEMENT_CHANNEL_ID`.
- Participant identity is linked at the Castaway instance level through `participants.discord_user_id`.
- Guild/user defaults in the bot state store already distinguish guild context for saved instance selection, but gameplay announcement routing is still global.

## Problems to solve

- Different Discord guilds should be able to host different Castaway seasons without sharing one announcement channel.
- A single bot deployment should be able to operate in more than one guild.
- Guild admins need a safe way to configure where public gameplay messages are published.
- Future guild-specific behavior should not require environment changes and redeploys for routine channel changes.

## Proposed direction

### Phase 1: Guild configuration persistence

Add bot-side persisted guild configuration in the existing state backend.

Suggested fields:
- `guild_id`
- `default_instance_id` or instance binding override
- `announcement_channel_id`
- `announcement_enabled`
- `admin_role_ids` or future permission overrides
- timestamps for creation/update

Notes:
- Keep `CASTAWAY_ANNOUNCEMENT_CHANNEL_ID` as a fallback default for now.
- Prefer guild config over env config when both exist.

### Phase 2: Multi-guild bot operation

- Stop treating `DISCORD_TARGET_SEVER_ID` as the only supported operating guild.
- Treat it as an optional bootstrap or development sync target.
- Ensure command handling, instance defaults, and announcement publishing use the interaction guild id or configured guild context.

### Phase 3: Admin configuration UX

Add admin-only commands such as:
- `/castaway config show`
- `/castaway config announcement-channel channel:#survivor`
- `/castaway config announcement-enabled true|false`
- `/castaway config instance instance:<name>`

### Phase 4: API and identity hardening

Evaluate whether guild-aware participant identity needs explicit persistence beyond instance-level `discord_user_id`.

Possible directions:
- keep `discord_user_id` instance-scoped and rely on Discord's globally stable user ids
- add guild-specific admin/member policy in bot state only
- if needed later, add guild-scoped membership/config tables in `castaway-web`

## Implementation notes

- Publish messages should use the current interaction guild when available.
- Background or system-driven announcements should resolve the guild from stored instance/guild configuration.
- Mention formatting should prefer actual Discord mentions like `<@discord_user_id>` instead of plain `@username` text.

## Open questions

- Should one Castaway instance be bound to exactly one guild, or can multiple guilds observe the same instance?
- Should guild configuration live only in bot state, or be promoted into `castaway-web` for auditability and shared access?
- Do we need per-guild role mappings for admin authorization, or is instance-admin state enough?

## Near-term recommendation

- Keep the current env-var announcement channel as the production fallback.
- Add guild-config persistence in the bot state store next.
- Follow with admin slash commands for channel configuration.
- Defer server-side guild tables until multi-guild usage creates a real need.
