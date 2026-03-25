CREATE TABLE participant_advantages (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL,
    advantage_type TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired')),
    source_activity_occurrence_id BIGINT REFERENCES activity_occurrences(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    effective_until TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (effective_until IS NULL OR effective_until > effective_at)
);

CREATE INDEX participant_advantages_instance_participant_idx
    ON participant_advantages(instance_id, participant_id);

CREATE INDEX participant_advantages_instance_type_idx
    ON participant_advantages(instance_id, advantage_type);

CREATE INDEX participant_advantages_instance_status_idx
    ON participant_advantages(instance_id, status);

CREATE INDEX participant_advantages_group_idx
    ON participant_advantages(participant_group_id)
    WHERE participant_group_id IS NOT NULL;
