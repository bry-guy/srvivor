CREATE TABLE castawordle_games (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 80),
    answer TEXT NOT NULL CHECK (answer ~ '^[A-Z]{4,8}$'),
    dictionary_version TEXT NOT NULL,
    opens_at TIMESTAMPTZ NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (cutoff_at > opens_at)
);

CREATE INDEX castawordle_games_instance_id_idx ON castawordle_games(instance_id);

CREATE TABLE castawordle_plays (
    game_id BIGINT NOT NULL REFERENCES castawordle_games(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    guesses JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(guesses) = 'array' AND jsonb_array_length(guesses) <= 6),
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'solved', 'exhausted')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (game_id, participant_id)
);
