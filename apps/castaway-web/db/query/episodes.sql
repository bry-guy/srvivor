-- name: CreateInstanceEpisode :one
INSERT INTO instance_episodes (instance_id, episode_number, label, airs_at, metadata)
SELECT i.id, sqlc.arg(episode_number), sqlc.arg(label), sqlc.arg(airs_at), sqlc.arg(metadata)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING
    public_id AS id,
    (SELECT public_id FROM instances WHERE id = instance_episodes.instance_id) AS instance_id,
    episode_number,
    label,
    airs_at,
    metadata,
    created_at,
    updated_at;

-- name: ListInstanceEpisodes :many
SELECT
    ie.public_id AS id,
    i.public_id AS instance_id,
    ie.episode_number,
    ie.label,
    ie.airs_at,
    ie.metadata,
    ie.created_at,
    ie.updated_at
FROM instance_episodes ie
JOIN instances i ON i.id = ie.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY ie.episode_number ASC;

-- name: GetCurrentEpisodeAt :one
SELECT
    ie.public_id AS id,
    i.public_id AS instance_id,
    ie.episode_number,
    ie.label,
    ie.airs_at,
    ie.metadata,
    ie.created_at,
    ie.updated_at
FROM instance_episodes ie
JOIN instances i ON i.id = ie.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND ie.airs_at <= sqlc.arg(at)
ORDER BY ie.airs_at DESC, ie.episode_number DESC
LIMIT 1;

-- name: ListEpisodeBoundaryWindows :many
SELECT
    ie.episode_number,
    ie.label,
    ie.airs_at AS starts_at,
    LEAD(ie.airs_at) OVER (
        PARTITION BY ie.instance_id
        ORDER BY ie.airs_at ASC, ie.episode_number ASC
    ) AS ends_at
FROM instance_episodes ie
JOIN instances i ON i.id = ie.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ORDER BY ie.episode_number ASC;
