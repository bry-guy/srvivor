-- name: ListInstanceTribeMembershipsAt :many
SELECT
    pg.public_id AS participant_group_id,
    pg.name AS participant_group_name,
    p.public_id AS participant_id,
    p.name AS participant_name,
    pgmp.starts_at
FROM participant_group_membership_periods pgmp
JOIN participant_groups pg ON pg.id = pgmp.participant_group_id
JOIN participants p ON p.id = pgmp.participant_id
JOIN instances i ON i.id = pg.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND pg.kind = 'tribe'
  AND pgmp.starts_at <= sqlc.arg(at)
  AND (pgmp.ends_at IS NULL OR pgmp.ends_at > sqlc.arg(at))
ORDER BY pg.name ASC, p.name ASC;

-- name: CountInstanceTribeMembershipsStartingAtOrAfter :one
SELECT COUNT(*)::bigint
FROM participant_group_membership_periods pgmp
JOIN participant_groups pg ON pg.id = pgmp.participant_group_id
JOIN instances i ON i.id = pg.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND pg.kind = 'tribe'
  AND pgmp.starts_at >= sqlc.arg(at);

-- name: EndInstanceTribeMembershipsAt :exec
UPDATE participant_group_membership_periods pgmp
SET ends_at = sqlc.arg(at)
FROM participant_groups pg, instances i
WHERE pg.id = pgmp.participant_group_id
  AND i.id = pg.instance_id
  AND i.public_id = sqlc.arg(instance_id)
  AND pg.kind = 'tribe'
  AND pgmp.starts_at < sqlc.arg(at)
  AND (pgmp.ends_at IS NULL OR pgmp.ends_at > sqlc.arg(at));

-- name: GetActivityOccurrenceBySourceRef :one
SELECT
    ao.public_id AS id,
    ao.occurrence_type,
    ao.name,
    ao.effective_at,
    ao.status,
    ao.metadata
FROM activity_occurrences ao
JOIN instance_activities ia ON ia.id = ao.activity_id
WHERE ia.public_id = sqlc.arg(activity_id)
  AND ao.source_ref = sqlc.arg(source_ref);
