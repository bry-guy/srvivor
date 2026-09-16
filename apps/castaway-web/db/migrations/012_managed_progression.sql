ALTER TABLE instances
    ADD COLUMN progression_mode TEXT NOT NULL DEFAULT 'legacy'
    CHECK (progression_mode IN ('legacy', 'managed'));

CREATE TABLE instance_draft_progress (
    instance_id BIGINT PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'open', 'closed')),
    opened_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE instance_episode_progress (
    instance_id BIGINT NOT NULL,
    episode_number INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'locked'
        CHECK (status IN ('locked', 'started', 'completed', 'scored')),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    scored_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, episode_number),
    FOREIGN KEY (instance_id, episode_number)
        REFERENCES instance_episodes(instance_id, episode_number)
        ON DELETE CASCADE
);

CREATE TABLE instance_score_revisions (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    revision_number INTEGER NOT NULL CHECK (revision_number > 0),
    episode_number INTEGER NOT NULL CHECK (episode_number >= 0),
    reason TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, revision_number)
);

CREATE TABLE instance_score_revision_rows (
    revision_id BIGINT NOT NULL REFERENCES instance_score_revisions(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    participant_name TEXT NOT NULL,
    score INTEGER NOT NULL,
    draft_points INTEGER NOT NULL,
    bonus_points INTEGER NOT NULL,
    total_points INTEGER NOT NULL,
    points_available INTEGER NOT NULL,
    PRIMARY KEY (revision_id, participant_id)
);

CREATE TABLE instance_progression_commands (
    id BIGSERIAL PRIMARY KEY,
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    command_key TEXT NOT NULL,
    operation TEXT NOT NULL,
    actor_discord_user_id TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    payload_hash TEXT NOT NULL,
    payload JSONB NOT NULL,
    response JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, command_key)
);

CREATE INDEX instance_score_revisions_latest_idx
    ON instance_score_revisions(instance_id, revision_number DESC);
