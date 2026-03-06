-- name: CreateParticipant :one
INSERT INTO participants (instance_id, name)
SELECT i.id, sqlc.arg(name)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    public_id AS id,
    (SELECT public_id FROM instances WHERE id = participants.instance_id) AS instance_id,
    name,
    created_at;

-- name: ListParticipantsByInstance :many
SELECT
    p.public_id AS id,
    i.public_id AS instance_id,
    p.name,
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
    p.created_at
FROM participants p
JOIN instances i ON i.id = p.instance_id
WHERE p.public_id = sqlc.arg(id);
