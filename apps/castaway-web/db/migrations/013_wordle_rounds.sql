ALTER TABLE activity_occurrences
    ADD CONSTRAINT activity_occurrences_id_activity_id_key UNIQUE (id, activity_id);

CREATE TABLE wordle_rounds (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES instance_activities(id) ON DELETE CASCADE,
    activity_occurrence_id BIGINT NOT NULL,
    round_key TEXT NOT NULL,
    opens_at TIMESTAMPTZ NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ,
    resolution_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (activity_occurrence_id),
    UNIQUE (activity_id, round_key),
    FOREIGN KEY (activity_occurrence_id, activity_id)
        REFERENCES activity_occurrences(id, activity_id)
        ON DELETE CASCADE,
    CHECK (char_length(btrim(round_key)) BETWEEN 1 AND 64),
    CHECK (cutoff_at > opens_at)
);

CREATE INDEX wordle_rounds_activity_id_idx
    ON wordle_rounds(activity_id);
