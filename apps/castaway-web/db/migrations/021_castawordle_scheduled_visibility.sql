ALTER TABLE castawordle_games
    ADD COLUMN episode_number INTEGER CHECK (episode_number >= 0),
    ADD CONSTRAINT castawordle_games_instance_episode_fk
        FOREIGN KEY (instance_id, episode_number)
        REFERENCES instance_episodes (instance_id, episode_number);

CREATE UNIQUE INDEX castawordle_games_instance_episode_idx
    ON castawordle_games (instance_id, episode_number)
    WHERE episode_number IS NOT NULL;
