-- name: CreateBonusPointLedgerEntry :one
WITH resolved_instance AS (
    SELECT instances.id AS instance_internal_id, instances.public_id AS instance_id
    FROM instances
    WHERE instances.public_id = sqlc.arg(instance_id)
), resolved_participant AS (
    SELECT p.id AS participant_internal_id, p.public_id AS participant_id
    FROM participants p
    JOIN resolved_instance ri ON ri.instance_internal_id = p.instance_id
    WHERE p.public_id = sqlc.arg(participant_id)
), resolved_occurrence AS (
    SELECT ao.id AS activity_occurrence_internal_id, ao.public_id AS activity_occurrence_id
    FROM activity_occurrences ao
    JOIN instance_activities ia ON ia.id = ao.activity_id
    JOIN resolved_instance ri ON ri.instance_internal_id = ia.instance_id
    WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
), resolved_source_group AS (
    SELECT pg.id AS source_group_internal_id, pg.public_id AS source_group_id
    FROM participant_groups pg
    JOIN resolved_instance ri ON ri.instance_internal_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(source_group_id)
)
INSERT INTO bonus_point_ledger_entries (
    instance_id,
    participant_id,
    activity_occurrence_id,
    source_group_id,
    entry_kind,
    points,
    visibility,
    reason,
    effective_at,
    award_key,
    metadata
)
SELECT
    ri.instance_internal_id,
    rp.participant_internal_id,
    ro.activity_occurrence_internal_id,
    rsg.source_group_internal_id,
    sqlc.arg(entry_kind),
    sqlc.arg(points),
    sqlc.arg(visibility),
    sqlc.arg(reason),
    sqlc.arg(effective_at),
    sqlc.arg(award_key),
    sqlc.arg(metadata)
FROM resolved_instance ri
CROSS JOIN resolved_participant rp
CROSS JOIN resolved_occurrence ro
LEFT JOIN resolved_source_group rsg ON TRUE
RETURNING
    public_id AS id,
    (SELECT instance_id FROM resolved_instance) AS instance_id,
    (SELECT participant_id FROM resolved_participant) AS participant_id,
    (SELECT activity_occurrence_id FROM resolved_occurrence) AS activity_occurrence_id,
    (SELECT source_group_id FROM resolved_source_group) AS source_group_id,
    entry_kind,
    points,
    visibility,
    reason,
    effective_at,
    award_key,
    metadata,
    created_at;

-- name: ListAllBonusPointLedgerEntriesForParticipant :many
SELECT
    bple.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    ao.public_id AS activity_occurrence_id,
    ao.occurrence_type,
    ao.name AS occurrence_name,
    ia.public_id AS activity_id,
    ia.activity_type,
    ia.name AS activity_name,
    sg.public_id AS source_group_id,
    sg.name AS source_group_name,
    bple.entry_kind,
    bple.points,
    bple.visibility,
    bple.reason,
    bple.effective_at,
    bple.award_key,
    bple.metadata,
    bple.created_at
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
JOIN activity_occurrences ao ON ao.id = bple.activity_occurrence_id
JOIN instance_activities ia ON ia.id = ao.activity_id
LEFT JOIN participant_groups sg ON sg.id = bple.source_group_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
ORDER BY bple.effective_at ASC, bple.created_at ASC, bple.id ASC;

-- name: ListVisibleBonusPointLedgerEntriesForParticipant :many
SELECT
    bple.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    ao.public_id AS activity_occurrence_id,
    ao.occurrence_type,
    ao.name AS occurrence_name,
    ia.public_id AS activity_id,
    ia.activity_type,
    ia.name AS activity_name,
    sg.public_id AS source_group_id,
    sg.name AS source_group_name,
    bple.entry_kind,
    bple.points,
    bple.visibility,
    bple.reason,
    bple.effective_at,
    bple.award_key,
    bple.metadata,
    bple.created_at
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
JOIN activity_occurrences ao ON ao.id = bple.activity_occurrence_id
JOIN instance_activities ia ON ia.id = ao.activity_id
LEFT JOIN participant_groups sg ON sg.id = bple.source_group_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND bple.visibility IN ('public', 'revealed')
ORDER BY bple.effective_at ASC, bple.created_at ASC, bple.id ASC;

-- name: ListVisibleBonusPointLedgerEntriesByOccurrence :many
SELECT
    bple.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    ao.public_id AS activity_occurrence_id,
    ao.occurrence_type,
    ao.name AS occurrence_name,
    ia.public_id AS activity_id,
    ia.activity_type,
    ia.name AS activity_name,
    sg.public_id AS source_group_id,
    sg.name AS source_group_name,
    bple.entry_kind,
    bple.points,
    bple.visibility,
    bple.reason,
    bple.effective_at,
    bple.award_key,
    bple.metadata,
    bple.created_at
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
JOIN activity_occurrences ao ON ao.id = bple.activity_occurrence_id
JOIN instance_activities ia ON ia.id = ao.activity_id
LEFT JOIN participant_groups sg ON sg.id = bple.source_group_id
WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
  AND bple.visibility IN ('public', 'revealed')
ORDER BY p.name ASC, bple.effective_at ASC, bple.created_at ASC, bple.id ASC;

-- name: GetVisibleBonusTotalByParticipant :one
SELECT COALESCE(SUM(bple.points), 0)::INTEGER AS total_points
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND bple.visibility IN ('public', 'revealed');

-- name: GetSecretBonusTotalByParticipant :one
SELECT COALESCE(SUM(bple.points), 0)::INTEGER AS total_points
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND bple.visibility = 'secret';

-- name: GetVisibleBonusTotalByParticipantAsOf :one
SELECT COALESCE(SUM(bple.points), 0)::INTEGER AS total_points
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND bple.visibility IN ('public', 'revealed')
  AND bple.effective_at <= sqlc.arg(as_of);

-- name: GetAvailableSecretBalanceByParticipant :one
SELECT GREATEST(COALESCE(SUM(bple.points), 0), 0)::INTEGER AS total_points
FROM bonus_point_ledger_entries bple
JOIN instances i ON i.id = bple.instance_id
JOIN participants p ON p.id = bple.participant_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND bple.visibility = 'secret';
