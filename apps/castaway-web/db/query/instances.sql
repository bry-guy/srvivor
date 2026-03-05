-- name: CreateInstance :one
INSERT INTO instances (name, season)
VALUES ($1, $2)
RETURNING public_id AS id, name, season, created_at;

-- name: GetInstance :one
SELECT public_id AS id, name, season, created_at
FROM instances
WHERE public_id = sqlc.arg(id);

-- name: ListInstances :many
SELECT public_id AS id, name, season, created_at
FROM instances
ORDER BY created_at DESC;

-- name: DeleteInstanceByNameSeason :exec
DELETE FROM instances
WHERE name = $1 AND season = $2;
