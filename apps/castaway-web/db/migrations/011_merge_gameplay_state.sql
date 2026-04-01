CREATE TABLE participant_pony_ownerships (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    owner_participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    contestant_id BIGINT NOT NULL REFERENCES contestants(id) ON DELETE CASCADE,
    source_activity_occurrence_id BIGINT REFERENCES activity_occurrences(id) ON DELETE SET NULL,
    acquired_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'released', 'revoked')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (released_at IS NULL OR released_at > acquired_at)
);

CREATE UNIQUE INDEX participant_pony_ownerships_active_contestant_idx
    ON participant_pony_ownerships(instance_id, contestant_id)
    WHERE status = 'active';

CREATE INDEX participant_pony_ownerships_owner_idx
    ON participant_pony_ownerships(instance_id, owner_participant_id);

CREATE INDEX participant_pony_ownerships_contestant_idx
    ON participant_pony_ownerships(instance_id, contestant_id);

CREATE TABLE participant_loans (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    activity_id BIGINT REFERENCES instance_activities(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'repaid', 'defaulted', 'cancelled')),
    principal_points INTEGER NOT NULL CHECK (principal_points >= 0),
    interest_points INTEGER NOT NULL CHECK (interest_points >= 0),
    principal_repaid_points INTEGER NOT NULL DEFAULT 0 CHECK (principal_repaid_points >= 0),
    interest_repaid_points INTEGER NOT NULL DEFAULT 0 CHECK (interest_repaid_points >= 0),
    granted_at TIMESTAMPTZ NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (principal_repaid_points <= principal_points),
    CHECK (interest_repaid_points <= interest_points),
    CHECK (settled_at IS NULL OR settled_at >= granted_at)
);

CREATE UNIQUE INDEX participant_loans_active_participant_idx
    ON participant_loans(instance_id, participant_id)
    WHERE status = 'active';

CREATE INDEX participant_loans_instance_status_idx
    ON participant_loans(instance_id, status);

CREATE INDEX participant_loans_participant_idx
    ON participant_loans(instance_id, participant_id);
