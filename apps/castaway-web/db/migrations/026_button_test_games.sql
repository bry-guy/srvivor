-- Press the Button admin test games have no episode: only admins see them and they never score.
ALTER TABLE button_games ALTER COLUMN episode_number DROP NOT NULL;

-- A private Castawordle is one player's own puzzle for a scored week: it replaces their result in the
-- original game's round, only they can see it, and it has no round of its own.
ALTER TABLE castawordle_games
    ADD COLUMN replaces_game_id BIGINT REFERENCES castawordle_games(id) ON DELETE CASCADE,
    ADD COLUMN player_id BIGINT REFERENCES participants(id) ON DELETE CASCADE,
    ADD CONSTRAINT castawordle_games_private_check
        CHECK ((replaces_game_id IS NULL) = (player_id IS NULL) AND (replaces_game_id IS NULL OR (episode_number IS NULL AND wordle_round_id IS NULL)));

CREATE UNIQUE INDEX castawordle_games_private_idx ON castawordle_games (replaces_game_id, player_id) WHERE replaces_game_id IS NOT NULL;
