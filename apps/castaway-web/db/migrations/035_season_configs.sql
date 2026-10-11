-- The season's schedule/config (seasons/NN.yaml) and scores template, synced in by `probst season sync`.
-- The database copy is what the scheduled job runs; the file is only Probst's input.
CREATE TABLE season_configs (
    instance_id BIGINT PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    season_yaml TEXT NOT NULL,
    scores_template TEXT NOT NULL,
    revision TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    updated_by TEXT NOT NULL DEFAULT ''
);
