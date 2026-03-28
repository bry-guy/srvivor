-- name: IsInstanceAdmin :one
SELECT EXISTS (
    SELECT 1
    FROM instance_admins ia
    JOIN instances i ON i.id = ia.instance_id
    WHERE i.public_id = sqlc.arg(instance_id)
      AND ia.discord_user_id = sqlc.arg(discord_user_id)
);

-- name: CountInstanceAdmins :one
SELECT COUNT(*)::bigint
FROM instance_admins ia
JOIN instances i ON i.id = ia.instance_id
WHERE i.public_id = sqlc.arg(instance_id);

-- name: CreateInstanceAdmin :one
INSERT INTO instance_admins (instance_id, discord_user_id)
SELECT i.id, sqlc.arg(discord_user_id)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    id,
    instance_id,
    discord_user_id,
    created_at;

-- name: DeleteInstanceAdmin :exec
DELETE FROM instance_admins ia
USING instances i
WHERE ia.instance_id = i.id
  AND i.public_id = sqlc.arg(instance_id)
  AND ia.discord_user_id = sqlc.arg(discord_user_id);

-- name: ListInstanceAdmins :many
SELECT
    ia.id,
    i.public_id AS instance_id,
    ia.discord_user_id,
    ia.created_at
FROM instance_admins ia
JOIN instances i ON i.id = ia.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY ia.created_at ASC;
