-- name: UpsertOutcomePosition :one
WITH resolved_instance AS (
    SELECT id AS instance_internal_id, public_id AS instance_id
    FROM instances
    WHERE instances.public_id = sqlc.arg(instance_id)
), resolved_contestant AS (
    SELECT c.id AS contestant_internal_id, c.public_id AS contestant_id
    FROM contestants c
    JOIN instance_contestants ic ON ic.contestant_id = c.id
    JOIN resolved_instance ri ON ri.instance_internal_id = ic.instance_id
    WHERE c.public_id = sqlc.arg(contestant_id)
), upserted AS (
    INSERT INTO outcome_positions (instance_id, position, contestant_id, updated_at)
    VALUES (
        (SELECT instance_internal_id FROM resolved_instance),
        sqlc.arg(position),
        (SELECT contestant_internal_id FROM resolved_contestant),
        NOW()
    )
    ON CONFLICT (instance_id, position)
    DO UPDATE SET contestant_id = EXCLUDED.contestant_id, updated_at = NOW()
    RETURNING instance_id, position, contestant_id, updated_at
)
SELECT
    (SELECT instance_id FROM resolved_instance) AS instance_id,
    upserted.position,
    (SELECT contestant_id FROM resolved_contestant) AS contestant_id,
    upserted.updated_at
FROM upserted;

-- name: ListOutcomePositionsByInstance :many
SELECT
    i.public_id AS instance_id,
    op.position,
    c.public_id AS contestant_id,
    op.updated_at
FROM outcome_positions op
JOIN instances i ON i.id = op.instance_id
LEFT JOIN contestants c ON c.id = op.contestant_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY op.position ASC;
