-- Press the Button: one game per episode week. Presses are counted per player and scored at cutoff.
CREATE TABLE button_games (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    episode_number INTEGER NOT NULL,
    opens_at TIMESTAMPTZ NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL CHECK (cutoff_at > opens_at),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, episode_number)
);

CREATE TABLE button_presses (
    game_id BIGINT NOT NULL REFERENCES button_games(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    presses BIGINT NOT NULL CHECK (presses > 0),
    PRIMARY KEY (game_id, participant_id)
);
