-- Spell It Out: arrange letter tiles (the phrase's letters plus random decoys) into the hidden phrase.
-- Each player's clock starts when they reveal their tiles; the three fastest solves score at cutoff.
CREATE TABLE scramble_games (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    episode_number INTEGER, -- NULL: an admin-only test that never scores
    phrase TEXT NOT NULL CHECK (phrase ~ '^[A-Z]+( [A-Z]+)*$'),
    decoys TEXT NOT NULL CHECK (decoys ~ '^[A-Z]*$'),
    opens_at TIMESTAMPTZ NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL CHECK (cutoff_at > opens_at),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, episode_number)
);

CREATE TABLE scramble_plays (
    game_id BIGINT NOT NULL REFERENCES scramble_games(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    tiles TEXT NOT NULL, -- this player's shuffled tray
    started_at TIMESTAMPTZ NOT NULL,
    solved_at TIMESTAMPTZ,
    wrong_checks INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, participant_id)
);
