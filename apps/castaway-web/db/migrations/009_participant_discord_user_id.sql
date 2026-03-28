ALTER TABLE participants ADD COLUMN discord_user_id TEXT;

CREATE UNIQUE INDEX participants_instance_discord_user_id_idx
    ON participants(instance_id, discord_user_id)
    WHERE discord_user_id IS NOT NULL;
