CREATE TABLE instance_admins (
    id BIGSERIAL PRIMARY KEY,
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    discord_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, discord_user_id)
);

CREATE INDEX instance_admins_instance_id_idx
    ON instance_admins(instance_id);
