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

-- name: LockInstanceNameSeason :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key), 0));

-- name: LockInstancesByNameSeason :many
SELECT public_id, progression_mode
FROM instances
WHERE name = sqlc.arg(name)
  AND season = sqlc.arg(season)
FOR UPDATE;

-- name: DeleteInstanceByNameSeason :exec
DELETE FROM instances
WHERE name = $1
  AND season = $2
  AND progression_mode = 'legacy';

-- name: UpdateInstanceName :one
UPDATE instances
SET name = $2
WHERE public_id = $1
RETURNING public_id AS id, name, season, created_at;
