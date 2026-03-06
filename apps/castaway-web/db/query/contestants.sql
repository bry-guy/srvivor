-- name: CreateContestant :one
WITH upserted AS (
    INSERT INTO contestants (name)
    VALUES (sqlc.arg(name))
    ON CONFLICT (name)
    DO UPDATE SET name = EXCLUDED.name
    RETURNING id, public_id, name, created_at
), linked AS (
    INSERT INTO instance_contestants (instance_id, contestant_id)
    SELECT i.id, u.id
    FROM instances i
    CROSS JOIN upserted u
    WHERE i.public_id = sqlc.arg(instance_id)
    ON CONFLICT (instance_id, contestant_id) DO NOTHING
)
SELECT public_id AS id, name, created_at
FROM upserted;

-- name: ListContestantsByInstance :many
SELECT c.public_id AS id, c.name, c.created_at
FROM contestants c
JOIN instance_contestants ic ON ic.contestant_id = c.id
JOIN instances i ON i.id = ic.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY c.name ASC;

-- name: GetContestant :one
SELECT public_id AS id, name, created_at
FROM contestants
WHERE public_id = sqlc.arg(id);

-- name: ListContestantsGlobal :many
SELECT public_id AS id, name, created_at
FROM contestants
ORDER BY name ASC;

-- name: InstanceHasContestant :one
SELECT EXISTS (
    SELECT 1
    FROM instance_contestants ic
    JOIN instances i ON i.id = ic.instance_id
    JOIN contestants c ON c.id = ic.contestant_id
    WHERE i.public_id = sqlc.arg(instance_id)
      AND c.public_id = sqlc.arg(contestant_id)
);
