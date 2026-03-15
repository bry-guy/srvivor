CREATE TABLE instance_episodes (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    episode_number INTEGER NOT NULL CHECK (episode_number >= 0),
    label TEXT NOT NULL,
    airs_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, episode_number),
    UNIQUE (instance_id, airs_at)
);

CREATE INDEX instance_episodes_instance_airs_at_idx
    ON instance_episodes(instance_id, airs_at);
