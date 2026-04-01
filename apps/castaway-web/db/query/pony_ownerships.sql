-- name: CreateParticipantPonyOwnership :one
WITH resolved_instance AS (
    SELECT i.id AS instance_internal_id, i.public_id AS instance_id
    FROM instances i
    WHERE i.public_id = sqlc.arg(instance_id)
), resolved_owner AS (
    SELECT p.id AS owner_internal_id, p.public_id AS owner_participant_id
    FROM participants p
    JOIN resolved_instance ri ON ri.instance_internal_id = p.instance_id
    WHERE p.public_id = sqlc.arg(owner_participant_id)
), resolved_contestant AS (
    SELECT c.id AS contestant_internal_id, c.public_id AS contestant_id
    FROM contestants c
    JOIN instance_contestants ic ON ic.contestant_id = c.id
    JOIN resolved_instance ri ON ri.instance_internal_id = ic.instance_id
    WHERE c.public_id = sqlc.arg(contestant_id)
), resolved_occurrence AS (
    SELECT ao.id AS occurrence_internal_id, ao.public_id AS source_activity_occurrence_id
    FROM activity_occurrences ao
    JOIN instance_activities ia ON ia.id = ao.activity_id
    JOIN resolved_instance ri ON ri.instance_internal_id = ia.instance_id
    WHERE ao.public_id = sqlc.arg(source_activity_occurrence_id)
)
INSERT INTO participant_pony_ownerships (
    instance_id,
    owner_participant_id,
    contestant_id,
    source_activity_occurrence_id,
    acquired_at,
    released_at,
    status,
    metadata
)
SELECT
    ri.instance_internal_id,
    ro.owner_internal_id,
    rc.contestant_internal_id,
    rao.occurrence_internal_id,
    sqlc.arg(acquired_at),
    sqlc.arg(released_at),
    sqlc.arg(status),
    sqlc.arg(metadata)
FROM resolved_instance ri
CROSS JOIN resolved_owner ro
CROSS JOIN resolved_contestant rc
LEFT JOIN resolved_occurrence rao ON TRUE
RETURNING
    public_id AS id,
    (SELECT instance_id FROM resolved_instance) AS instance_id,
    (SELECT owner_participant_id FROM resolved_owner) AS owner_participant_id,
    (SELECT contestant_id FROM resolved_contestant) AS contestant_id,
    (SELECT source_activity_occurrence_id FROM resolved_occurrence) AS source_activity_occurrence_id,
    acquired_at,
    released_at,
    status,
    metadata,
    created_at,
    updated_at;

-- name: ListActiveParticipantPonyOwnershipsByContestantAt :many
SELECT
    ppo.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS owner_participant_id,
    p.name AS owner_participant_name,
    c.public_id AS contestant_id,
    c.name AS contestant_name,
    ao.public_id AS source_activity_occurrence_id,
    ppo.acquired_at,
    ppo.released_at,
    ppo.status,
    ppo.metadata,
    ppo.created_at,
    ppo.updated_at
FROM participant_pony_ownerships ppo
JOIN instances i ON i.id = ppo.instance_id
JOIN participants p ON p.id = ppo.owner_participant_id
JOIN contestants c ON c.id = ppo.contestant_id
LEFT JOIN activity_occurrences ao ON ao.id = ppo.source_activity_occurrence_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND c.public_id = sqlc.arg(contestant_id)
  AND ppo.status = 'active'
  AND ppo.acquired_at <= sqlc.arg(at)
  AND (ppo.released_at IS NULL OR ppo.released_at > sqlc.arg(at))
ORDER BY p.name ASC, ppo.id ASC;

-- name: ListActiveParticipantPonyOwnershipsByOwnerAt :many
SELECT
    ppo.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS owner_participant_id,
    p.name AS owner_participant_name,
    c.public_id AS contestant_id,
    c.name AS contestant_name,
    ao.public_id AS source_activity_occurrence_id,
    ppo.acquired_at,
    ppo.released_at,
    ppo.status,
    ppo.metadata,
    ppo.created_at,
    ppo.updated_at
FROM participant_pony_ownerships ppo
JOIN instances i ON i.id = ppo.instance_id
JOIN participants p ON p.id = ppo.owner_participant_id
JOIN contestants c ON c.id = ppo.contestant_id
LEFT JOIN activity_occurrences ao ON ao.id = ppo.source_activity_occurrence_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(owner_participant_id)
  AND ppo.status = 'active'
  AND ppo.acquired_at <= sqlc.arg(at)
  AND (ppo.released_at IS NULL OR ppo.released_at > sqlc.arg(at))
ORDER BY c.name ASC, ppo.id ASC;
