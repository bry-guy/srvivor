ALTER TABLE contestants RENAME TO contestants_legacy;

CREATE TABLE contestants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE instance_contestants (
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    contestant_id UUID NOT NULL REFERENCES contestants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, contestant_id)
);

INSERT INTO contestants (name, created_at)
SELECT name, MIN(created_at)
FROM contestants_legacy
GROUP BY name;

INSERT INTO instance_contestants (instance_id, contestant_id, created_at)
SELECT c_legacy.instance_id, c.id, c_legacy.created_at
FROM contestants_legacy c_legacy
JOIN contestants c ON c.name = c_legacy.name
ON CONFLICT (instance_id, contestant_id) DO NOTHING;

ALTER TABLE draft_picks DROP CONSTRAINT IF EXISTS draft_picks_contestant_id_fkey;
ALTER TABLE outcome_positions DROP CONSTRAINT IF EXISTS outcome_positions_contestant_id_fkey;

ALTER TABLE draft_picks
    ADD CONSTRAINT draft_picks_instance_contestant_fkey
    FOREIGN KEY (instance_id, contestant_id)
    REFERENCES instance_contestants(instance_id, contestant_id)
    ON DELETE CASCADE;

ALTER TABLE outcome_positions
    ADD CONSTRAINT outcome_positions_instance_contestant_fkey
    FOREIGN KEY (instance_id, contestant_id)
    REFERENCES instance_contestants(instance_id, contestant_id)
    ON DELETE CASCADE;

DROP TABLE contestants_legacy;
