-- name: LockDiscordChannel :exec
SELECT pg_advisory_xact_lock(hashtextextended('discord-channel:' || sqlc.arg(channel_key)::text, 0));

-- name: GetDiscordChannelBinding :one
SELECT b.guild_id, b.channel_id, i.public_id AS instance_id
FROM discord_channel_bindings b
JOIN instances i ON i.id = b.instance_id
WHERE b.guild_id = sqlc.arg(guild_id) AND b.channel_id = sqlc.arg(channel_id);

-- name: SetDiscordChannelBinding :exec
INSERT INTO discord_channel_bindings (guild_id, channel_id, instance_id)
SELECT sqlc.arg(guild_id), sqlc.arg(channel_id), i.id FROM instances i WHERE i.public_id = sqlc.arg(instance_id)
ON CONFLICT (guild_id, channel_id) DO UPDATE SET instance_id = EXCLUDED.instance_id;

-- name: DeleteDiscordChannelBinding :exec
DELETE FROM discord_channel_bindings WHERE guild_id = sqlc.arg(guild_id) AND channel_id = sqlc.arg(channel_id);
