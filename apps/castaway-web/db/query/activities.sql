-- name: CreateInstanceActivity :one
INSERT INTO instance_activities (
    instance_id,
    activity_type,
    name,
    status,
    starts_at,
    ends_at,
    metadata
)
SELECT
    i.id,
    sqlc.arg(activity_type),
    sqlc.arg(name),
    sqlc.arg(status),
    sqlc.arg(starts_at),
    sqlc.arg(ends_at),
    sqlc.arg(metadata)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    public_id AS id,
    (SELECT public_id FROM instances WHERE id = instance_activities.instance_id) AS instance_id,
    activity_type,
    name,
    status,
    starts_at,
    ends_at,
    metadata,
    created_at,
    updated_at;

-- name: ListInstanceActivitiesByInstance :many
SELECT
    ia.public_id AS id,
    i.public_id AS instance_id,
    ia.activity_type,
    ia.name,
    ia.status,
    ia.starts_at,
    ia.ends_at,
    ia.metadata,
    ia.created_at,
    ia.updated_at
FROM instance_activities ia
JOIN instances i ON i.id = ia.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY ia.starts_at ASC, ia.id ASC;

-- name: GetInstanceActivity :one
SELECT
    ia.public_id AS id,
    i.public_id AS instance_id,
    ia.activity_type,
    ia.name,
    ia.status,
    ia.starts_at,
    ia.ends_at,
    ia.metadata,
    ia.created_at,
    ia.updated_at
FROM instance_activities ia
JOIN instances i ON i.id = ia.instance_id
WHERE ia.public_id = sqlc.arg(id);

-- name: CreateActivityGroupAssignment :one
WITH resolved AS (
    SELECT
        ia.id AS activity_internal_id,
        pg.id AS participant_group_internal_id,
        ia.public_id AS activity_id,
        pg.public_id AS participant_group_id
    FROM instance_activities ia
    JOIN participant_groups pg ON pg.public_id = sqlc.arg(participant_group_id)
       AND pg.instance_id = ia.instance_id
    WHERE ia.public_id = sqlc.arg(activity_id)
)
INSERT INTO activity_group_assignments (
    activity_id,
    participant_group_id,
    role,
    starts_at,
    ends_at,
    configuration
)
SELECT
    activity_internal_id,
    participant_group_internal_id,
    sqlc.arg(role),
    sqlc.arg(starts_at),
    sqlc.arg(ends_at),
    sqlc.arg(configuration)
FROM resolved
RETURNING
    id,
    (SELECT activity_id FROM resolved) AS activity_id,
    (SELECT participant_group_id FROM resolved) AS participant_group_id,
    role,
    starts_at,
    ends_at,
    configuration,
    created_at;

-- name: ListActivityGroupAssignments :many
SELECT
    aga.id,
    ia.public_id AS activity_id,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    aga.role,
    aga.starts_at,
    aga.ends_at,
    aga.configuration,
    aga.created_at
FROM activity_group_assignments aga
JOIN instance_activities ia ON ia.id = aga.activity_id
JOIN participant_groups pg ON pg.id = aga.participant_group_id
WHERE ia.public_id = sqlc.arg(activity_id)
ORDER BY aga.starts_at ASC, aga.id ASC;

-- name: ListActiveActivityGroupAssignmentsAt :many
SELECT
    aga.id,
    ia.public_id AS activity_id,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    aga.role,
    aga.starts_at,
    aga.ends_at,
    aga.configuration,
    aga.created_at
FROM activity_group_assignments aga
JOIN instance_activities ia ON ia.id = aga.activity_id
JOIN participant_groups pg ON pg.id = aga.participant_group_id
WHERE ia.public_id = sqlc.arg(activity_id)
  AND aga.starts_at <= sqlc.arg(at)
  AND (aga.ends_at IS NULL OR aga.ends_at > sqlc.arg(at))
ORDER BY pg.name ASC, aga.id ASC;

-- name: CreateActivityParticipantAssignment :one
WITH resolved_activity AS (
    SELECT
        ia.id AS activity_internal_id,
        ia.instance_id,
        ia.public_id AS activity_id
    FROM instance_activities ia
    WHERE ia.public_id = sqlc.arg(activity_id)
), resolved_participant AS (
    SELECT
        p.id AS participant_internal_id,
        p.public_id AS participant_id
    FROM participants p
    JOIN resolved_activity ra ON ra.instance_id = p.instance_id
    WHERE p.public_id = sqlc.arg(participant_id)
), resolved_group AS (
    SELECT
        pg.id AS participant_group_internal_id,
        pg.public_id AS participant_group_id
    FROM participant_groups pg
    JOIN resolved_activity ra ON ra.instance_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(participant_group_id)
)
INSERT INTO activity_participant_assignments (
    activity_id,
    participant_id,
    participant_group_id,
    role,
    starts_at,
    ends_at,
    configuration
)
SELECT
    ra.activity_internal_id,
    rp.participant_internal_id,
    rg.participant_group_internal_id,
    sqlc.arg(role),
    sqlc.arg(starts_at),
    sqlc.arg(ends_at),
    sqlc.arg(configuration)
FROM resolved_activity ra
CROSS JOIN resolved_participant rp
LEFT JOIN resolved_group rg ON TRUE
RETURNING
    id,
    (SELECT activity_id FROM resolved_activity) AS activity_id,
    (SELECT participant_id FROM resolved_participant) AS participant_id,
    (SELECT participant_group_id FROM resolved_group) AS participant_group_id,
    role,
    starts_at,
    ends_at,
    configuration,
    created_at;

-- name: ListActivityParticipantAssignments :many
SELECT
    apa.id,
    ia.public_id AS activity_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    apa.role,
    apa.starts_at,
    apa.ends_at,
    apa.configuration,
    apa.created_at
FROM activity_participant_assignments apa
JOIN instance_activities ia ON ia.id = apa.activity_id
JOIN participants p ON p.id = apa.participant_id
LEFT JOIN participant_groups pg ON pg.id = apa.participant_group_id
WHERE ia.public_id = sqlc.arg(activity_id)
ORDER BY apa.starts_at ASC, apa.id ASC;

-- name: ListActiveActivityParticipantAssignmentsAt :many
SELECT
    apa.id,
    ia.public_id AS activity_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    apa.role,
    apa.starts_at,
    apa.ends_at,
    apa.configuration,
    apa.created_at
FROM activity_participant_assignments apa
JOIN instance_activities ia ON ia.id = apa.activity_id
JOIN participants p ON p.id = apa.participant_id
LEFT JOIN participant_groups pg ON pg.id = apa.participant_group_id
WHERE ia.public_id = sqlc.arg(activity_id)
  AND apa.starts_at <= sqlc.arg(at)
  AND (apa.ends_at IS NULL OR apa.ends_at > sqlc.arg(at))
ORDER BY p.name ASC, apa.id ASC;
