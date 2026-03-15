CREATE TABLE participant_groups (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, kind, name)
);

CREATE INDEX participant_groups_instance_kind_idx
    ON participant_groups(instance_id, kind);

CREATE TABLE participant_group_membership_periods (
    id BIGSERIAL PRIMARY KEY,
    participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR ends_at > starts_at),
    UNIQUE (participant_group_id, participant_id, role, starts_at)
);

CREATE INDEX participant_group_membership_periods_group_starts_at_idx
    ON participant_group_membership_periods(participant_group_id, starts_at);

CREATE INDEX participant_group_membership_periods_participant_starts_at_idx
    ON participant_group_membership_periods(participant_id, starts_at);

CREATE TABLE instance_activities (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    activity_type TEXT NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX instance_activities_instance_activity_type_idx
    ON instance_activities(instance_id, activity_type);

CREATE INDEX instance_activities_instance_starts_at_idx
    ON instance_activities(instance_id, starts_at);

CREATE TABLE activity_group_assignments (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE,
    participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR ends_at > starts_at),
    UNIQUE (activity_id, participant_group_id, role, starts_at)
);

CREATE INDEX activity_group_assignments_activity_starts_at_idx
    ON activity_group_assignments(activity_id, starts_at);

CREATE INDEX activity_group_assignments_group_starts_at_idx
    ON activity_group_assignments(participant_group_id, starts_at);

CREATE TABLE activity_participant_assignments (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL,
    role TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR ends_at > starts_at),
    UNIQUE (activity_id, participant_id, role, starts_at)
);

CREATE INDEX activity_participant_assignments_activity_starts_at_idx
    ON activity_participant_assignments(activity_id, starts_at);

CREATE INDEX activity_participant_assignments_participant_starts_at_idx
    ON activity_participant_assignments(participant_id, starts_at);
