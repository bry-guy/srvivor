-- name: CreateActivityOccurrence :one
WITH resolved_activity AS (
    SELECT
        ia.id AS activity_internal_id,
        ia.public_id AS activity_id
    FROM instance_activities ia
    WHERE ia.public_id = sqlc.arg(activity_id)
)
INSERT INTO activity_occurrences (
    activity_id,
    occurrence_type,
    name,
    effective_at,
    starts_at,
    ends_at,
    status,
    source_ref,
    metadata
)
SELECT
    activity_internal_id,
    sqlc.arg(occurrence_type),
    sqlc.arg(name),
    sqlc.arg(effective_at),
    sqlc.arg(starts_at),
    sqlc.arg(ends_at),
    sqlc.arg(status),
    sqlc.arg(source_ref),
    sqlc.arg(metadata)
FROM resolved_activity
RETURNING
    public_id AS id,
    (SELECT activity_id FROM resolved_activity) AS activity_id,
    occurrence_type,
    name,
    effective_at,
    starts_at,
    ends_at,
    status,
    source_ref,
    metadata,
    created_at,
    updated_at;

-- name: ListActivityOccurrencesByActivity :many
SELECT
    ao.public_id AS id,
    ia.public_id AS activity_id,
    ao.occurrence_type,
    ao.name,
    ao.effective_at,
    ao.starts_at,
    ao.ends_at,
    ao.status,
    ao.source_ref,
    ao.metadata,
    ao.created_at,
    ao.updated_at
FROM activity_occurrences ao
JOIN instance_activities ia ON ia.id = ao.activity_id
WHERE ia.public_id = sqlc.arg(activity_id)
ORDER BY ao.effective_at ASC, ao.id ASC;

-- name: GetActivityOccurrence :one
SELECT
    ao.public_id AS id,
    ia.public_id AS activity_id,
    ao.occurrence_type,
    ao.name,
    ao.effective_at,
    ao.starts_at,
    ao.ends_at,
    ao.status,
    ao.source_ref,
    ao.metadata,
    ao.created_at,
    ao.updated_at
FROM activity_occurrences ao
JOIN instance_activities ia ON ia.id = ao.activity_id
WHERE ao.public_id = sqlc.arg(id);

-- name: CreateActivityOccurrenceGroup :one
WITH resolved_occurrence AS (
    SELECT
        ao.id AS activity_occurrence_internal_id,
        ao.public_id AS activity_occurrence_id,
        ia.instance_id
    FROM activity_occurrences ao
    JOIN instance_activities ia ON ia.id = ao.activity_id
    WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
), resolved_group AS (
    SELECT
        pg.id AS participant_group_internal_id,
        pg.public_id AS participant_group_id
    FROM participant_groups pg
    JOIN resolved_occurrence ro ON ro.instance_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(participant_group_id)
)
INSERT INTO activity_occurrence_groups (
    activity_occurrence_id,
    participant_group_id,
    role,
    result,
    metadata
)
SELECT
    ro.activity_occurrence_internal_id,
    rg.participant_group_internal_id,
    sqlc.arg(role),
    sqlc.arg(result),
    sqlc.arg(metadata)
FROM resolved_occurrence ro
CROSS JOIN resolved_group rg
RETURNING
    id,
    (SELECT activity_occurrence_id FROM resolved_occurrence) AS activity_occurrence_id,
    (SELECT participant_group_id FROM resolved_group) AS participant_group_id,
    role,
    result,
    metadata,
    created_at;

-- name: ListActivityOccurrenceGroups :many
SELECT
    aog.id,
    ao.public_id AS activity_occurrence_id,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    aog.role,
    aog.result,
    aog.metadata,
    aog.created_at
FROM activity_occurrence_groups aog
JOIN activity_occurrences ao ON ao.id = aog.activity_occurrence_id
JOIN participant_groups pg ON pg.id = aog.participant_group_id
WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
ORDER BY pg.name ASC, aog.id ASC;

-- name: CreateActivityOccurrenceParticipant :one
WITH resolved_occurrence AS (
    SELECT
        ao.id AS activity_occurrence_internal_id,
        ao.public_id AS activity_occurrence_id,
        ia.instance_id
    FROM activity_occurrences ao
    JOIN instance_activities ia ON ia.id = ao.activity_id
    WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
), resolved_participant AS (
    SELECT
        p.id AS participant_internal_id,
        p.public_id AS participant_id
    FROM participants p
    JOIN resolved_occurrence ro ON ro.instance_id = p.instance_id
    WHERE p.public_id = sqlc.arg(participant_id)
), resolved_group AS (
    SELECT
        pg.id AS participant_group_internal_id,
        pg.public_id AS participant_group_id
    FROM participant_groups pg
    JOIN resolved_occurrence ro ON ro.instance_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(participant_group_id)
)
INSERT INTO activity_occurrence_participants (
    activity_occurrence_id,
    participant_id,
    participant_group_id,
    role,
    result,
    metadata
)
SELECT
    ro.activity_occurrence_internal_id,
    rp.participant_internal_id,
    rg.participant_group_internal_id,
    sqlc.arg(role),
    sqlc.arg(result),
    sqlc.arg(metadata)
FROM resolved_occurrence ro
CROSS JOIN resolved_participant rp
LEFT JOIN resolved_group rg ON TRUE
RETURNING
    id,
    (SELECT activity_occurrence_id FROM resolved_occurrence) AS activity_occurrence_id,
    (SELECT participant_id FROM resolved_participant) AS participant_id,
    (SELECT participant_group_id FROM resolved_group) AS participant_group_id,
    role,
    result,
    metadata,
    created_at;

-- name: ListActivityOccurrenceParticipants :many
SELECT
    aop.id,
    ao.public_id AS activity_occurrence_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    aop.role,
    aop.result,
    aop.metadata,
    aop.created_at
FROM activity_occurrence_participants aop
JOIN activity_occurrences ao ON ao.id = aop.activity_occurrence_id
JOIN participants p ON p.id = aop.participant_id
LEFT JOIN participant_groups pg ON pg.id = aop.participant_group_id
WHERE ao.public_id = sqlc.arg(activity_occurrence_id)
ORDER BY p.name ASC, aop.id ASC;

-- name: ListParticipantOccurrenceInvolvementByInstance :many
SELECT
    ia.public_id AS activity_id,
    ia.activity_type,
    ia.name AS activity_name,
    ia.status AS activity_status,
    ia.starts_at AS activity_starts_at,
    ia.ends_at AS activity_ends_at,
    ia.metadata AS activity_metadata,
    ia.created_at AS activity_created_at,
    ia.updated_at AS activity_updated_at,
    ao.public_id AS occurrence_id,
    ao.occurrence_type,
    ao.name AS occurrence_name,
    ao.effective_at,
    ao.starts_at,
    ao.ends_at,
    ao.status AS occurrence_status,
    ao.source_ref,
    ao.metadata AS occurrence_metadata,
    ao.created_at AS occurrence_created_at,
    ao.updated_at AS occurrence_updated_at,
    aop.id AS occurrence_participant_result_id,
    p.public_id AS participant_id,
    aop.role,
    aop.result,
    aop.metadata AS participant_metadata,
    aop.created_at AS participant_created_at,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name
FROM activity_occurrence_participants aop
JOIN activity_occurrences ao ON ao.id = aop.activity_occurrence_id
JOIN instance_activities ia ON ia.id = ao.activity_id
JOIN instances i ON i.id = ia.instance_id
JOIN participants p ON p.id = aop.participant_id
LEFT JOIN participant_groups pg ON pg.id = aop.participant_group_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
ORDER BY ia.starts_at ASC, ao.effective_at ASC, aop.id ASC;
