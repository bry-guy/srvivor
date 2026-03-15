CREATE TABLE activity_occurrences (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE,
    occurrence_type TEXT NOT NULL,
    name TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'recorded' CHECK (status IN ('recorded', 'resolved', 'cancelled')),
    source_ref TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX activity_occurrences_activity_effective_at_idx
    ON activity_occurrences(activity_id, effective_at);

CREATE INDEX activity_occurrences_activity_occurrence_type_idx
    ON activity_occurrences(activity_id, occurrence_type);

CREATE TABLE activity_occurrence_groups (
    id BIGSERIAL PRIMARY KEY,
    activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE,
    participant_group_id BIGINT NOT NULL REFERENCES participant_groups(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (activity_occurrence_id, participant_group_id, role)
);

CREATE INDEX activity_occurrence_groups_occurrence_idx
    ON activity_occurrence_groups(activity_occurrence_id);

CREATE INDEX activity_occurrence_groups_group_idx
    ON activity_occurrence_groups(participant_group_id);

CREATE TABLE activity_occurrence_participants (
    id BIGSERIAL PRIMARY KEY,
    activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    participant_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL,
    role TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (activity_occurrence_id, participant_id, role)
);

CREATE INDEX activity_occurrence_participants_occurrence_idx
    ON activity_occurrence_participants(activity_occurrence_id);

CREATE INDEX activity_occurrence_participants_participant_idx
    ON activity_occurrence_participants(participant_id);

CREATE TABLE bonus_point_ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    activity_occurrence_id BIGINT NOT NULL REFERENCES activity_occurrences(id) ON DELETE CASCADE,
    source_group_id BIGINT REFERENCES participant_groups(id) ON DELETE SET NULL,
    entry_kind TEXT NOT NULL CHECK (entry_kind IN ('award', 'correction', 'spend', 'conversion', 'reveal')),
    points INTEGER NOT NULL CHECK (points <> 0),
    visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'secret', 'revealed')),
    reason TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    award_key TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX bonus_point_ledger_entries_occurrence_participant_award_key_idx
    ON bonus_point_ledger_entries(activity_occurrence_id, participant_id, award_key)
    WHERE award_key IS NOT NULL;

CREATE INDEX bonus_point_ledger_entries_instance_participant_effective_at_idx
    ON bonus_point_ledger_entries(instance_id, participant_id, effective_at);

CREATE INDEX bonus_point_ledger_entries_instance_visibility_effective_at_idx
    ON bonus_point_ledger_entries(instance_id, visibility, effective_at);

CREATE INDEX bonus_point_ledger_entries_occurrence_idx
    ON bonus_point_ledger_entries(activity_occurrence_id);
