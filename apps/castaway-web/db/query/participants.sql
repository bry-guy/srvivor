-- name: CreateParticipant :one
INSERT INTO participants (instance_id, name)
SELECT i.id, sqlc.arg(name)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    public_id AS id,
    (SELECT public_id FROM instances WHERE id = participants.instance_id) AS instance_id,
    name,
    discord_user_id,
    created_at;

-- name: ListParticipantsByInstance :many
SELECT
    p.public_id AS id,
    i.public_id AS instance_id,
    p.name,
    p.discord_user_id,
    p.created_at
FROM participants p
JOIN instances i ON i.id = p.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY p.created_at ASC;

-- name: GetParticipant :one
SELECT
    p.public_id AS id,
    i.public_id AS instance_id,
    p.name,
    p.discord_user_id,
    p.created_at
FROM participants p
JOIN instances i ON i.id = p.instance_id
WHERE p.public_id = sqlc.arg(id);

-- name: GetParticipantByDiscordUserID :one
SELECT
    p.public_id AS id,
    i.public_id AS instance_id,
    p.name,
    p.discord_user_id,
    p.created_at
FROM participants p
JOIN instances i ON i.id = p.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.discord_user_id = sqlc.arg(discord_user_id);

-- name: SetParticipantDiscordUserID :one
UPDATE participants p
SET discord_user_id = sqlc.arg(discord_user_id)
WHERE p.public_id = sqlc.arg(id)
RETURNING
    p.public_id AS id,
    (SELECT public_id FROM instances WHERE id = p.instance_id) AS instance_id,
    p.name,
    p.discord_user_id,
    p.created_at;

-- name: ClearParticipantDiscordUserID :one
UPDATE participants p
SET discord_user_id = NULL
WHERE p.public_id = sqlc.arg(id)
RETURNING
    p.public_id AS id,
    (SELECT public_id FROM instances WHERE id = p.instance_id) AS instance_id,
    p.name,
    p.discord_user_id,
    p.created_at;
