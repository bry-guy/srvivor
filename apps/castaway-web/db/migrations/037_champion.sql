-- Pick Your Champion (from the merge): each player picks one castaway, changeable until locks_at, then fixed for
-- the rest of the season. Challenges after the lock award pickers of the winners (+3 immunity, +1 reward).
CREATE TABLE champion_rounds (
    instance_id BIGINT PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    merge_episode INTEGER NOT NULL CHECK (merge_episode > 0),
    opens_at TIMESTAMPTZ NOT NULL,
    locks_at TIMESTAMPTZ NOT NULL CHECK (locks_at > opens_at),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE champion_picks (
    instance_id BIGINT NOT NULL REFERENCES champion_rounds(instance_id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    contestant_id BIGINT NOT NULL REFERENCES contestants(id) ON DELETE CASCADE,
    picked_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (instance_id, participant_id)
);
