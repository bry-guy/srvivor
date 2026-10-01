ALTER TABLE castawordle_games
    ADD COLUMN wordle_round_id UUID REFERENCES activity_occurrences(public_id) ON DELETE CASCADE,
    ADD CONSTRAINT castawordle_games_scored_episode_check
        CHECK (wordle_round_id IS NULL OR episode_number IS NOT NULL);

CREATE UNIQUE INDEX castawordle_games_wordle_round_id_idx
    ON castawordle_games (wordle_round_id)
    WHERE wordle_round_id IS NOT NULL;
