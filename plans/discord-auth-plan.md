# Discord User Authentication Plan

Status: `planning`

## Goal

Allow Discord users to authenticate through the bot and view privileged information (secret bonus points) that belongs to their linked participant account.

## Context

Today, all `castaway-web` API routes are protected by service-to-service bearer token auth (`requireServiceAuth`). The Discord bot authenticates to the API as a service principal (`castaway-discord-bot`), but there is no concept of an end-user identity flowing through the system. Bonus point ledger entries have a `visibility` field (`public`, `secret`, `revealed`), and secret entries are already queryable (`GetSecretBonusTotalByParticipant`, `GetAvailableSecretBalanceByParticipant`, `ListAllBonusPointLedgerEntriesForParticipant`) but are not exposed through any API route or bot command today. Participants currently have no link to a Discord user ID.

## Design

### 1. Link participants to Discord users (database)

Add a nullable `discord_user_id TEXT` column to the `participants` table via a new migration.

```sql
-- 008_participant_discord_user_id.sql
ALTER TABLE participants ADD COLUMN discord_user_id TEXT;
CREATE UNIQUE INDEX participants_instance_discord_user_id_idx
    ON participants(instance_id, discord_user_id)
    WHERE discord_user_id IS NOT NULL;
```

This allows at most one participant per Discord user per instance, while keeping the column optional for participants without Discord accounts. Add a corresponding sqlc query to look up a participant by instance + discord_user_id, and a mutation to set/clear the discord_user_id on an existing participant.

### 2. Add API routes for linking; enrich existing bonus-ledger route

Add the following new routes to `castaway-web`, all under the existing `requireServiceAuth` middleware:

| Method | Path | Purpose |
|--------|------|---------|
| `PUT` | `/instances/:instanceID/participants/:participantID/discord-link` | Set `discord_user_id` on a participant |
| `DELETE` | `/instances/:instanceID/participants/:participantID/discord-link` | Clear the link |

**No new `secret-bonus` route.** Instead, modify the existing `GET /instances/:instanceID/participants/:participantID/bonus-ledger` handler to conditionally include secret entries:

- If the request includes a `X-Discord-User-ID` header **and** its value matches the participant's stored `discord_user_id`, return **all** ledger entries (visible + secret) using the existing `ListAllBonusPointLedgerEntriesForParticipant` query, and include the full bonus total (visible + secret).
- Otherwise, return only visible entries (current behavior, unchanged).

The response shape is identical in both cases — entries already carry a `visibility` field, so consumers can distinguish `secret` from `public`/`revealed` entries. This avoids adding a new route and keeps the API surface minimal. The bot simply adds the `X-Discord-User-ID` header to the same bonus-ledger call it would already make.

### 3. Add sqlc queries

```sql
-- query/participants.sql additions
-- name: GetParticipantByDiscordUserID :one
SELECT ... FROM participants
WHERE instance_id = (SELECT id FROM instances WHERE public_id = $1)
  AND discord_user_id = $2;

-- name: SetParticipantDiscordUserID :exec
UPDATE participants SET discord_user_id = $2
WHERE public_id = $1;

-- name: ClearParticipantDiscordUserID :exec
UPDATE participants SET discord_user_id = NULL
WHERE public_id = $1;
```

### 4. Discord bot: `/castaway link` command

Add a new slash command subgroup or top-level subcommand:

- `/castaway link` — Links the invoking Discord user to a participant in the current instance. The bot sends `PUT /instances/:id/participants/:pid/discord-link` with the interaction user's Discord ID. Requires the user to specify (or autocomplete) which participant they are. Responds with confirmation (ephemeral).
- `/castaway unlink` — Clears the link. Responds with confirmation (ephemeral).

### 5. Discord bot: `/castaway myscore` command

A new command that:
1. Resolves the instance (same logic as existing commands).
2. Looks up the participant linked to the invoking Discord user's ID (via a new API call or by listing participants and checking the discord link).
3. Calls `GET /instances/:id/participants/:pid/bonus-ledger` with the `X-Discord-User-ID` header set to the invoking user's Discord ID. Because the user is authenticated, the response will include secret entries.
4. Formats and returns an ephemeral message showing the user's secret bonus point total and ledger breakdown alongside their regular score.

This is ephemeral so only the requesting user sees their secret data.

### 6. Update the bot's castaway client

Add methods to the `castaway.Client`:
- `LinkDiscordUser(ctx, instanceID, participantID, discordUserID)` — `PUT` to the discord-link endpoint.
- `UnlinkDiscordUser(ctx, instanceID, participantID)` — `DELETE` to the discord-link endpoint.
- `GetBonusLedger(ctx, instanceID, participantID, discordUserID)` — `GET` to the existing bonus-ledger endpoint; when `discordUserID` is non-empty, passes the `X-Discord-User-ID` header to get secret entries included.

## Implementation order

1. Database migration (`008_participant_discord_user_id.sql`)
2. sqlc queries + regenerate (`participants.sql`, run `sqlc generate`)
3. `castaway-web` API routes (discord-link PUT/DELETE, bonus-ledger auth enrichment)
4. `castaway-web` tests for new routes
5. `castaway.Client` methods in the bot
6. Bot slash commands (`link`, `unlink`, `myscore`) + handler logic
7. Bot command registration (update `commands.go`)
8. Bot handler tests

## Security considerations

- The bot is already authenticated via bearer token; no new service auth mechanism is needed.
- Discord user ID verification happens server-side: the API checks that the `X-Discord-User-ID` header matches the participant's stored `discord_user_id`. The bot is trusted to pass the real user ID from the interaction.
- Secret data is only included in the `bonus-ledger` response when the `X-Discord-User-ID` header is present and matches the participant's stored `discord_user_id`. Without the header (or on mismatch), the response contains only visible entries, preserving existing behavior. Secret data is never mixed into the `leaderboard` routes.
- The `/castaway myscore` response is ephemeral (only visible to the invoking user).
- The link operation is idempotent. Re-linking to a different participant in the same instance will fail due to the unique index, requiring an explicit unlink first.

## Out of scope

- OAuth2 browser-based Discord login (not needed; Discord interactions already carry verified user identity).
- Admin workflows to link users on someone else's behalf.
- Showing secret bonus data in the leaderboard aggregate.
