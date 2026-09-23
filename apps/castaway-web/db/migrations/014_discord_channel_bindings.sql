CREATE TABLE discord_channel_bindings (
    guild_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    PRIMARY KEY (guild_id, channel_id)
);
