-- name: GetInstanceProgressionMode :one
SELECT progression_mode
FROM instances
WHERE public_id = sqlc.arg(instance_id);

-- name: LockInstanceForProgression :one
SELECT public_id AS id, progression_mode
FROM instances
WHERE public_id = sqlc.arg(instance_id)
FOR UPDATE;

-- name: SetInstanceProgressionMode :exec
UPDATE instances
SET progression_mode = sqlc.arg(progression_mode)
WHERE public_id = sqlc.arg(instance_id);

-- name: HasFutureOutcomeCommand :one
SELECT EXISTS (
    SELECT 1
    FROM instance_progression_commands ipc
    JOIN instances i ON i.id = ipc.instance_id
    WHERE i.public_id = sqlc.arg(instance_id)
      AND ipc.operation IN ('outcome.upsert', 'outcome.correct')
      AND ipc.effective_at > sqlc.arg(effective_at)
) AS has_future_outcome;

-- name: InitializeInstanceDraftProgress :exec
INSERT INTO instance_draft_progress (instance_id)
SELECT id
FROM instances
WHERE public_id = sqlc.arg(instance_id)
ON CONFLICT (instance_id) DO NOTHING;

-- name: InitializeInstanceEpisodeProgress :exec
INSERT INTO instance_episode_progress (instance_id, episode_number)
SELECT ie.instance_id, ie.episode_number
FROM instance_episodes ie
JOIN instances i ON i.id = ie.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
ON CONFLICT (instance_id, episode_number) DO NOTHING;

-- name: GetInstanceDraftProgress :one
SELECT
    i.public_id AS instance_id,
    idp.status,
    idp.opened_at,
    idp.closed_at,
    idp.updated_at
FROM instance_draft_progress idp
JOIN instances i ON i.id = idp.instance_id
WHERE i.public_id = sqlc.arg(instance_id);

-- name: LockInstanceDraftProgress :one
SELECT
    i.public_id AS instance_id,
    idp.status,
    idp.opened_at,
    idp.closed_at,
    idp.updated_at
FROM instance_draft_progress idp
JOIN instances i ON i.id = idp.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
FOR UPDATE;

-- name: UpdateInstanceDraftProgress :one
UPDATE instance_draft_progress idp
SET status = sqlc.arg(status),
    opened_at = sqlc.arg(opened_at),
    closed_at = sqlc.arg(closed_at),
    updated_at = NOW()
FROM instances i
WHERE idp.instance_id = i.id
  AND i.public_id = sqlc.arg(instance_id)
RETURNING
    i.public_id AS instance_id,
    idp.status,
    idp.opened_at,
    idp.closed_at,
    idp.updated_at;

-- name: GetInstanceEpisodeProgress :one
SELECT
    i.public_id AS instance_id,
    iep.episode_number,
    iep.status,
    iep.started_at,
    iep.completed_at,
    iep.scored_at,
    iep.updated_at
FROM instance_episode_progress iep
JOIN instances i ON i.id = iep.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND iep.episode_number = sqlc.arg(episode_number);

-- name: LockInstanceEpisodeProgress :one
SELECT
    i.public_id AS instance_id,
    iep.episode_number,
    iep.status,
    iep.started_at,
    iep.completed_at,
    iep.scored_at,
    iep.updated_at
FROM instance_episode_progress iep
JOIN instances i ON i.id = iep.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND iep.episode_number = sqlc.arg(episode_number)
FOR UPDATE;

-- name: UpdateInstanceEpisodeProgress :one
UPDATE instance_episode_progress iep
SET status = sqlc.arg(status),
    started_at = sqlc.arg(started_at),
    completed_at = sqlc.arg(completed_at),
    scored_at = sqlc.arg(scored_at),
    updated_at = NOW()
FROM instances i
WHERE iep.instance_id = i.id
  AND i.public_id = sqlc.arg(instance_id)
  AND iep.episode_number = sqlc.arg(episode_number)
RETURNING
    i.public_id AS instance_id,
    iep.episode_number,
    iep.status,
    iep.started_at,
    iep.completed_at,
    iep.scored_at,
    iep.updated_at;

-- name: GetPreviousInstanceEpisodeProgress :one
SELECT
    i.public_id AS instance_id,
    iep.episode_number,
    iep.status,
    iep.started_at,
    iep.completed_at,
    iep.scored_at,
    iep.updated_at
FROM instance_episode_progress iep
JOIN instances i ON i.id = iep.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND iep.episode_number > 0
  AND iep.episode_number < sqlc.arg(episode_number)
ORDER BY iep.episode_number DESC
LIMIT 1;

-- name: GetLatestManagedEpisodeProgress :one
SELECT
    i.public_id AS instance_id,
    iep.episode_number,
    iep.status,
    iep.started_at,
    iep.completed_at,
    iep.scored_at,
    iep.updated_at
FROM instance_episode_progress iep
JOIN instances i ON i.id = iep.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND iep.status IN ('started', 'completed', 'scored')
ORDER BY iep.episode_number DESC
LIMIT 1;

-- name: GetLatestManagedEpisodeBrief :one
SELECT
    ie.public_id AS id,
    i.public_id AS instance_id,
    ie.episode_number,
    ie.label,
    ie.airs_at,
    ie.metadata,
    ie.created_at,
    ie.updated_at
FROM instance_episode_progress iep
JOIN instance_episodes ie
  ON ie.instance_id = iep.instance_id
 AND ie.episode_number = iep.episode_number
JOIN instances i ON i.id = ie.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND iep.status IN ('started', 'completed', 'scored')
ORDER BY iep.episode_number DESC
LIMIT 1;

-- name: GetProgressionCommand :one
SELECT
    ipc.id,
    i.public_id AS instance_id,
    ipc.command_key,
    ipc.operation,
    ipc.actor_discord_user_id,
    ipc.effective_at,
    ipc.payload_hash,
    ipc.payload,
    ipc.response,
    ipc.created_at
FROM instance_progression_commands ipc
JOIN instances i ON i.id = ipc.instance_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND ipc.command_key = sqlc.arg(command_key);

-- name: CreateProgressionCommand :one
INSERT INTO instance_progression_commands (
    instance_id,
    command_key,
    operation,
    actor_discord_user_id,
    effective_at,
    payload_hash,
    payload,
    response
)
SELECT
    i.id,
    sqlc.arg(command_key),
    sqlc.arg(operation),
    sqlc.arg(actor_discord_user_id),
    sqlc.arg(effective_at),
    sqlc.arg(payload_hash),
    sqlc.arg(payload),
    sqlc.arg(response)
FROM instances i
WHERE i.public_id = sqlc.arg(instance_id)
RETURNING id, command_key, operation, actor_discord_user_id, effective_at, payload_hash, payload, response, created_at;
