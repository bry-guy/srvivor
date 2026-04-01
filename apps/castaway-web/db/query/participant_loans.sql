-- name: CreateParticipantLoan :one
WITH resolved_instance AS (
    SELECT i.id AS instance_internal_id, i.public_id AS instance_id
    FROM instances i
    WHERE i.public_id = sqlc.arg(instance_id)
), resolved_participant AS (
    SELECT p.id AS participant_internal_id, p.public_id AS participant_id
    FROM participants p
    JOIN resolved_instance ri ON ri.instance_internal_id = p.instance_id
    WHERE p.public_id = sqlc.arg(participant_id)
), resolved_activity AS (
    SELECT ia.id AS activity_internal_id, ia.public_id AS activity_id
    FROM instance_activities ia
    JOIN resolved_instance ri ON ri.instance_internal_id = ia.instance_id
    WHERE ia.public_id = sqlc.arg(activity_id)
)
INSERT INTO participant_loans (
    instance_id,
    participant_id,
    activity_id,
    status,
    principal_points,
    interest_points,
    principal_repaid_points,
    interest_repaid_points,
    granted_at,
    due_at,
    settled_at,
    metadata
)
SELECT
    ri.instance_internal_id,
    rp.participant_internal_id,
    ra.activity_internal_id,
    sqlc.arg(status),
    sqlc.arg(principal_points),
    sqlc.arg(interest_points),
    sqlc.arg(principal_repaid_points),
    sqlc.arg(interest_repaid_points),
    sqlc.arg(granted_at),
    sqlc.arg(due_at),
    sqlc.arg(settled_at),
    sqlc.arg(metadata)
FROM resolved_instance ri
CROSS JOIN resolved_participant rp
LEFT JOIN resolved_activity ra ON TRUE
RETURNING
    public_id AS id,
    (SELECT instance_id FROM resolved_instance) AS instance_id,
    (SELECT participant_id FROM resolved_participant) AS participant_id,
    (SELECT activity_id FROM resolved_activity) AS activity_id,
    status,
    principal_points,
    interest_points,
    principal_repaid_points,
    interest_repaid_points,
    granted_at,
    due_at,
    settled_at,
    metadata,
    created_at,
    updated_at;

-- name: GetActiveParticipantLoanByParticipant :one
SELECT
    pl.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    ia.public_id AS activity_id,
    pl.status,
    pl.principal_points,
    pl.interest_points,
    pl.principal_repaid_points,
    pl.interest_repaid_points,
    pl.granted_at,
    pl.due_at,
    pl.settled_at,
    pl.metadata,
    pl.created_at,
    pl.updated_at
FROM participant_loans pl
JOIN instances i ON i.id = pl.instance_id
JOIN participants p ON p.id = pl.participant_id
LEFT JOIN instance_activities ia ON ia.id = pl.activity_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND p.public_id = sqlc.arg(participant_id)
  AND pl.status = 'active';

-- name: UpdateParticipantLoan :one
UPDATE participant_loans pl
SET status = sqlc.arg(status),
    principal_points = sqlc.arg(principal_points),
    interest_points = sqlc.arg(interest_points),
    principal_repaid_points = sqlc.arg(principal_repaid_points),
    interest_repaid_points = sqlc.arg(interest_repaid_points),
    due_at = sqlc.arg(due_at),
    settled_at = sqlc.arg(settled_at),
    metadata = sqlc.arg(metadata),
    updated_at = NOW()
WHERE pl.public_id = sqlc.arg(id)
RETURNING
    pl.public_id AS id,
    (SELECT i.public_id FROM instances i WHERE i.id = pl.instance_id) AS instance_id,
    (SELECT p.public_id FROM participants p WHERE p.id = pl.participant_id) AS participant_id,
    (SELECT ia.public_id FROM instance_activities ia WHERE ia.id = pl.activity_id) AS activity_id,
    pl.status,
    pl.principal_points,
    pl.interest_points,
    pl.principal_repaid_points,
    pl.interest_repaid_points,
    pl.granted_at,
    pl.due_at,
    pl.settled_at,
    pl.metadata,
    pl.created_at,
    pl.updated_at;

-- name: ListActiveParticipantLoansByInstance :many
SELECT
    pl.public_id AS id,
    i.public_id AS instance_id,
    p.public_id AS participant_id,
    p.name AS participant_name,
    ia.public_id AS activity_id,
    pl.status,
    pl.principal_points,
    pl.interest_points,
    pl.principal_repaid_points,
    pl.interest_repaid_points,
    pl.granted_at,
    pl.due_at,
    pl.settled_at,
    pl.metadata,
    pl.created_at,
    pl.updated_at
FROM participant_loans pl
JOIN instances i ON i.id = pl.instance_id
JOIN participants p ON p.id = pl.participant_id
LEFT JOIN instance_activities ia ON ia.id = pl.activity_id
WHERE i.public_id = sqlc.arg(instance_id)
  AND pl.status = 'active'
ORDER BY p.name ASC, pl.id ASC;
