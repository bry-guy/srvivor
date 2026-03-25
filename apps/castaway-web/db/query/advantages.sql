-- name: CreateParticipantAdvantage :one
WITH resolved_instance AS (
    SELECT i.id AS instance_internal_id, i.public_id AS instance_id
    FROM instances i
    WHERE i.public_id = sqlc.arg(instance_id)
), resolved_participant AS (
    SELECT p.id AS participant_internal_id, p.public_id AS participant_id
    FROM participants p
    JOIN resolved_instance ri ON ri.instance_internal_id = p.instance_id
    WHERE p.public_id = sqlc.arg(participant_id)
), resolved_group AS (
    SELECT pg.id AS group_internal_id, pg.public_id AS participant_group_id
    FROM participant_groups pg
    JOIN resolved_instance ri ON ri.instance_internal_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(participant_group_id)
), resolved_occurrence AS (
    SELECT ao.id AS occurrence_internal_id, ao.public_id AS source_activity_occurrence_id
    FROM activity_occurrences ao
    JOIN instance_activities ia ON ia.id = ao.activity_id
    JOIN resolved_instance ri ON ri.instance_internal_id = ia.instance_id
    WHERE ao.public_id = sqlc.arg(source_activity_occurrence_id)
)
INSERT INTO participant_advantages (
    instance_id,
    participant_id,
    participant_group_id,
    advantage_type,
    name,
    status,
    source_activity_occurrence_id,
    granted_at,
    effective_at,
    effective_until,
    metadata
)
SELECT
    ri.instance_internal_id,
    rp.participant_internal_id,
    rg.group_internal_id,
    sqlc.arg(advantage_type),
    sqlc.arg(name),
    sqlc.arg(status),
    ro.occurrence_internal_id,
    sqlc.arg(granted_at),
    sqlc.arg(effective_at),
    sqlc.arg(effective_until),
    sqlc.arg(metadata)
FROM resolved_instance ri
CROSS JOIN resolved_participant rp
LEFT JOIN resolved_group rg ON TRUE
LEFT JOIN resolved_occurrence ro ON TRUE
RETURNING
    public_id AS id,
    (SELECT instance_id FROM resolved_instance) AS instance_id,
    (SELECT participant_id FROM resolved_participant) AS participant_id,
    (SELECT participant_group_id FROM resolved_group) AS participant_group_id,
    advantage_type,
    name,
    status,
    (SELECT source_activity_occurrence_id FROM resolved_occurrence) AS source_activity_occurrence_id,
    granted_at,
    effective_at,
    effective_until,
    metadata,
    created_at,
    updated_at;

-- name: ListActiveAdvantagesByTypeForGroup :many
SELECT
    pa.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    pg.public_id AS participant_group_id,
    pa.advantage_type,
    pa.name,
    pa.status,
    pa.granted_at,
    pa.effective_at,
    pa.effective_until,
    pa.metadata,
    pa.created_at,
    pa.updated_at
FROM participant_advantages pa
JOIN instances i ON i.id = pa.instance_id
JOIN participants p ON p.id = pa.participant_id
LEFT JOIN participant_groups pg ON pg.id = pa.participant_group_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND pg.public_id = sqlc.arg(participant_group_id)
  AND pa.advantage_type = sqlc.arg(advantage_type)
  AND pa.status = 'active'
  AND pa.effective_at <= sqlc.arg(at)
  AND (pa.effective_until IS NULL OR pa.effective_until > sqlc.arg(at))
ORDER BY pa.granted_at ASC;

-- name: ListActiveAdvantagesByTypeForParticipant :many
SELECT
    pa.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    pg.public_id AS participant_group_id,
    pa.advantage_type,
    pa.name,
    pa.status,
    pa.granted_at,
    pa.effective_at,
    pa.effective_until,
    pa.metadata,
    pa.created_at,
    pa.updated_at
FROM participant_advantages pa
JOIN instances i ON i.id = pa.instance_id
JOIN participants p ON p.id = pa.participant_id
LEFT JOIN participant_groups pg ON pg.id = pa.participant_group_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND pa.advantage_type = sqlc.arg(advantage_type)
  AND pa.status = 'active'
  AND pa.effective_at <= sqlc.arg(at)
  AND (pa.effective_until IS NULL OR pa.effective_until > sqlc.arg(at))
ORDER BY pa.granted_at ASC;

-- name: MarkAdvantageUsed :exec
UPDATE participant_advantages
SET status = 'used', updated_at = NOW()
WHERE public_id = sqlc.arg(id)
  AND status = 'active';
