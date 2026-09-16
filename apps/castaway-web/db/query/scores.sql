-- name: NextInstanceScoreRevisionNumber :one
SELECT COALESCE(MAX(isr.revision_number), 0)::INTEGER + 1 AS revision_number
FROM instance_score_revisions isr
JOIN instances i ON i.id = isr.instance_id
WHERE i.public_id = sqlc.arg(instance_id);

-- name: GetLatestInstanceScoreRevision :one
SELECT
    isr.id,
    isr.public_id,
    isr.revision_number,
    isr.episode_number,
    isr.reason,
    isr.effective_at,
    isr.input_snapshot,
    isr.created_at
FROM instance_score_revisions isr
JOIN instances i ON i.id = isr.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY isr.revision_number DESC
LIMIT 1;

-- name: CreateInstanceScoreRevision :one
INSERT INTO instance_score_revisions (
    instance_id,
    revision_number,
    episode_number,
    reason,
    effective_at,
    input_snapshot
)
SELECT
    i.id,
    sqlc.arg(revision_number),
    sqlc.arg(episode_number),
    sqlc.arg(reason),
    sqlc.arg(effective_at),
    sqlc.arg(input_snapshot)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING id, public_id, revision_number, episode_number, reason, effective_at, input_snapshot, created_at;

-- name: CreateInstanceScoreRevisionRow :exec
INSERT INTO instance_score_revision_rows (
    revision_id,
    participant_id,
    participant_name,
    score,
    draft_points,
    bonus_points,
    total_points,
    points_available
)
SELECT
    sqlc.arg(revision_id),
    p.id,
    sqlc.arg(participant_name),
    sqlc.arg(score),
    sqlc.arg(draft_points),
    sqlc.arg(bonus_points),
    sqlc.arg(total_points),
    sqlc.arg(points_available)
FROM participants p
JOIN instances i ON i.id = p.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id);

-- name: ListLatestInstanceScoreRevisionRows :many
WITH latest AS (
    SELECT isr.id, isr.public_id, isr.revision_number, isr.episode_number, isr.reason,
           isr.effective_at, isr.input_snapshot, isr.created_at
    FROM instance_score_revisions isr
    JOIN instances i ON i.id = isr.instance_id
    WHERE i.public_id = sqlc.arg(instance_id)
    ORDER BY isr.revision_number DESC
    LIMIT 1
)
SELECT
    latest.public_id AS revision_id,
    latest.revision_number,
    latest.episode_number,
    latest.reason,
    latest.effective_at,
    latest.input_snapshot,
    latest.created_at AS revision_created_at,
    p.public_id AS participant_id,
    isrr.participant_name,
    isrr.score,
    isrr.draft_points,
    isrr.bonus_points,
    isrr.total_points,
    isrr.points_available
FROM latest
JOIN instance_score_revision_rows isrr ON isrr.revision_id = latest.id
JOIN participants p ON p.id = isrr.participant_id
ORDER BY isrr.total_points DESC, isrr.participant_name ASC;
