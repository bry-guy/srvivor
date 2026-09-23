-- name: GetWordleRound :one
SELECT
    ao.public_id AS id,
    i.public_id AS instance_id,
    ia.public_id AS activity_id,
    ia.activity_type,
    ao.occurrence_type,
    ao.status AS occurrence_status,
    ao.effective_at,
    ao.name,
    wr.round_key,
    wr.opens_at,
    wr.cutoff_at,
    wr.closed_at,
    wr.resolution_response,
    wr.created_at,
    wr.updated_at
FROM wordle_rounds wr
JOIN activity_occurrences ao ON ao.id = wr.activity_occurrence_id
JOIN instance_activities ia ON ia.id = wr.activity_id
JOIN instances i ON i.id = ia.instance_id
WHERE ao.public_id = sqlc.arg(id);

-- name: GetWordleRoundByActivityAndKey :one
SELECT
    ao.public_id AS id,
    i.public_id AS instance_id,
    ia.public_id AS activity_id,
    ia.activity_type,
    ao.occurrence_type,
    ao.status AS occurrence_status,
    ao.effective_at,
    ao.name,
    wr.round_key,
    wr.opens_at,
    wr.cutoff_at,
    wr.closed_at,
    wr.resolution_response,
    wr.created_at,
    wr.updated_at
FROM wordle_rounds wr
JOIN activity_occurrences ao ON ao.id = wr.activity_occurrence_id
JOIN instance_activities ia ON ia.id = wr.activity_id
JOIN instances i ON i.id = ia.instance_id
WHERE ia.public_id = sqlc.arg(activity_id)
  AND wr.round_key = sqlc.arg(round_key);

-- name: LockWordleRound :one
SELECT
    ao.public_id AS id,
    i.public_id AS instance_id,
    ia.public_id AS activity_id,
    ia.activity_type,
    ao.occurrence_type,
    ao.status AS occurrence_status,
    ao.effective_at,
    ao.name,
    wr.round_key,
    wr.opens_at,
    wr.cutoff_at,
    wr.closed_at,
    wr.resolution_response,
    wr.created_at,
    wr.updated_at
FROM wordle_rounds wr
JOIN activity_occurrences ao ON ao.id = wr.activity_occurrence_id
JOIN instance_activities ia ON ia.id = wr.activity_id
JOIN instances i ON i.id = ia.instance_id
WHERE ao.public_id = sqlc.arg(id)
FOR UPDATE OF wr;

-- name: CreateWordleRound :one
INSERT INTO wordle_rounds (
    activity_id,
    activity_occurrence_id,
    round_key,
    opens_at,
    cutoff_at
)
SELECT
    ia.id,
    ao.id,
    sqlc.arg(round_key),
    sqlc.arg(opens_at),
    sqlc.arg(cutoff_at)
FROM instance_activities ia
JOIN activity_occurrences ao
  ON ao.public_id = sqlc.arg(activity_occurrence_id)
 AND ao.activity_id = ia.id
WHERE ia.public_id = sqlc.arg(activity_id)
RETURNING (
    SELECT public_id
    FROM activity_occurrences
    WHERE id = wordle_rounds.activity_occurrence_id
) AS id;

-- name: UpdateWordleRoundClosedAt :one
UPDATE wordle_rounds wr
SET closed_at = COALESCE(wr.closed_at, sqlc.arg(closed_at)),
    updated_at = NOW()
FROM activity_occurrences ao
WHERE wr.activity_occurrence_id = ao.id
  AND ao.public_id = sqlc.arg(id)
RETURNING wr.closed_at, wr.updated_at;

-- name: UpdateWordleRoundResolution :one
UPDATE wordle_rounds wr
SET resolution_response = sqlc.arg(resolution_response),
    updated_at = NOW()
FROM activity_occurrences ao
WHERE wr.activity_occurrence_id = ao.id
  AND ao.public_id = sqlc.arg(id)
RETURNING wr.resolution_response, wr.updated_at;
