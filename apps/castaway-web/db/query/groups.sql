-- name: CreateParticipantGroup :one
INSERT INTO participant_groups (instance_id, name, kind, metadata)
SELECT i.id, sqlc.arg(name), sqlc.arg(kind), sqlc.arg(metadata)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    public_id AS id,
    (SELECT public_id FROM instances WHERE id = participant_groups.instance_id) AS instance_id,
    name,
    kind,
    metadata,
    created_at,
    updated_at;

-- name: ListParticipantGroupsByInstance :many
SELECT
    pg.public_id AS id,
    i.public_id AS instance_id,
    pg.name,
    pg.kind,
    pg.metadata,
    pg.created_at,
    pg.updated_at
FROM participant_groups pg
JOIN instances i ON i.id = pg.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY pg.kind ASC, pg.name ASC;

-- name: GetParticipantGroup :one
SELECT
    pg.public_id AS id,
    i.public_id AS instance_id,
    pg.name,
    pg.kind,
    pg.metadata,
    pg.created_at,
    pg.updated_at
FROM participant_groups pg
JOIN instances i ON i.id = pg.instance_id
WHERE pg.public_id = sqlc.arg(id);

-- name: CreateParticipantGroupMembershipPeriod :one
WITH resolved AS (
    SELECT
        pg.id AS participant_group_internal_id,
        p.id AS participant_internal_id,
        pg.public_id AS participant_group_id,
        p.public_id AS participant_id
    FROM participant_groups pg
    JOIN participants p ON p.public_id = sqlc.arg(participant_id)
       AND p.instance_id = pg.instance_id
    WHERE pg.public_id = sqlc.arg(participant_group_id)
)
INSERT INTO participant_group_membership_periods (
    participant_group_id,
    participant_id,
    role,
    starts_at,
    ends_at,
    metadata
)
SELECT
    participant_group_internal_id,
    participant_internal_id,
    sqlc.arg(role),
    sqlc.arg(starts_at),
    sqlc.arg(ends_at),
    sqlc.arg(metadata)
FROM resolved
RETURNING
    id,
    (SELECT participant_group_id FROM resolved) AS participant_group_id,
    (SELECT participant_id FROM resolved) AS participant_id,
    role,
    starts_at,
    ends_at,
    metadata,
    created_at;

-- name: ListParticipantGroupMembershipPeriods :many
SELECT
    pgmp.id,
    pg.public_id AS participant_group_id,
    p.public_id AS participant_id,
    pgmp.role,
    pgmp.starts_at,
    pgmp.ends_at,
    pgmp.metadata,
    pgmp.created_at
FROM participant_group_membership_periods pgmp
JOIN participant_groups pg ON pg.id = pgmp.participant_group_id
JOIN participants p ON p.id = pgmp.participant_id
WHERE pg.public_id = sqlc.arg(participant_group_id)
ORDER BY pgmp.starts_at ASC, pgmp.id ASC;

-- name: ListActiveParticipantGroupMembershipsAt :many
SELECT
    pgmp.id,
    pg.public_id AS participant_group_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pgmp.role,
    pgmp.starts_at,
    pgmp.ends_at,
    pgmp.metadata,
    pgmp.created_at
FROM participant_group_membership_periods pgmp
JOIN participant_groups pg ON pg.id = pgmp.participant_group_id
JOIN participants p ON p.id = pgmp.participant_id
WHERE pg.public_id = sqlc.arg(participant_group_id)
  AND pgmp.starts_at <= sqlc.arg(at)
  AND (pgmp.ends_at IS NULL OR pgmp.ends_at > sqlc.arg(at))
ORDER BY p.name ASC, pgmp.id ASC;

-- name: ListActiveParticipantMembershipsAt :many
SELECT
    pgmp.id,
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    pg.kind AS participant_group_kind,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pgmp.role,
    pgmp.starts_at,
    pgmp.ends_at,
    pgmp.metadata,
    pgmp.created_at
FROM participant_group_membership_periods pgmp
JOIN participant_groups pg ON pg.id = pgmp.participant_group_id
JOIN participants p ON p.id = pgmp.participant_id
WHERE p.public_id = sqlc.arg(participant_id)
  AND pgmp.starts_at <= sqlc.arg(at)
  AND (pgmp.ends_at IS NULL OR pgmp.ends_at > sqlc.arg(at))
ORDER BY pg.kind ASC, pg.name ASC, pgmp.id ASC;
