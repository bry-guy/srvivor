-- Inside Track: flick 12 balls up three foam tracks into three cups. Each player gets one run per game; the
-- page saves which cups are filled and how many balls are used after every ball, so a reload resumes it.
-- Score: cups filled, plus balls left once all three cups are filled.
CREATE TABLE track_games (
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

CREATE TABLE track_plays (
    game_id BIGINT NOT NULL REFERENCES track_games(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    cups_mask INTEGER NOT NULL DEFAULT 0 CHECK (cups_mask BETWEEN 0 AND 7), -- bit per cup: red, yellow, blue
    balls_used INTEGER NOT NULL DEFAULT 0 CHECK (balls_used BETWEEN 0 AND 12),
    finished_at TIMESTAMPTZ,
    PRIMARY KEY (game_id, participant_id)
);
