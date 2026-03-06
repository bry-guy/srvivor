-- name: DeleteDraftPicksForParticipant :exec
DELETE FROM draft_picks dp
USING participants p
WHERE dp.participant_id = p.id
  AND p.public_id = sqlc.arg(participant_id);

-- name: CreateDraftPick :one
WITH resolved AS (
    SELECT
        i.id AS instance_internal_id,
        p.id AS participant_internal_id,
        c.id AS contestant_internal_id,
        i.public_id AS instance_id,
        p.public_id AS participant_id,
        c.public_id AS contestant_id
    FROM instances i
    JOIN participants p ON p.public_id = sqlc.arg(participant_id) AND p.instance_id = i.id
    JOIN contestants c ON c.public_id = sqlc.arg(contestant_id)
    JOIN instance_contestants ic ON ic.instance_id = i.id AND ic.contestant_id = c.id
    WHERE i.public_id = sqlc.arg(instance_id)
)
INSERT INTO draft_picks (instance_id, participant_id, contestant_id, position)
SELECT instance_internal_id, participant_internal_id, contestant_internal_id, sqlc.arg(position)
FROM resolved
RETURNING
    (SELECT instance_id FROM resolved) AS instance_id,
    (SELECT participant_id FROM resolved) AS participant_id,
    (SELECT contestant_id FROM resolved) AS contestant_id,
    position,
    created_at;

-- name: ListDraftPicksForParticipant :many
SELECT
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    c.public_id AS contestant_id,
    dp.position,
    dp.created_at
FROM draft_picks dp
JOIN instances i ON i.id = dp.instance_id
JOIN participants p ON p.id = dp.participant_id
JOIN contestants c ON c.id = dp.contestant_id
WHERE p.public_id = sqlc.arg(participant_id)
ORDER BY dp.position ASC;

-- name: ListDraftPicksForInstance :many
SELECT
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    c.public_id AS contestant_id,
    dp.position,
    dp.created_at
FROM draft_picks dp
JOIN instances i ON i.id = dp.instance_id
JOIN participants p ON p.id = dp.participant_id
JOIN contestants c ON c.id = dp.contestant_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY p.public_id ASC, dp.position ASC;
