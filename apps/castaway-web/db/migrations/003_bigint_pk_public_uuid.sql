ALTER TABLE draft_picks RENAME TO draft_picks_legacy;
ALTER TABLE outcome_positions RENAME TO outcome_positions_legacy;
ALTER TABLE instance_contestants RENAME TO instance_contestants_legacy;
ALTER TABLE participants RENAME TO participants_legacy;
ALTER TABLE contestants RENAME TO contestants_legacy;
ALTER TABLE instances RENAME TO instances_legacy;

DROP INDEX IF EXISTS outcome_positions_unique_contestant_per_instance;

CREATE TABLE instances (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    season INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE contestants (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE participants (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, name)
);

CREATE TABLE instance_contestants (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    contestant_id BIGINT NOT NULL REFERENCES contestants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, contestant_id)
);

CREATE TABLE draft_picks (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    participant_id BIGINT NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    contestant_id BIGINT NOT NULL REFERENCES contestants(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (participant_id, position),
    UNIQUE (participant_id, contestant_id),
    FOREIGN KEY (instance_id, contestant_id)
        REFERENCES instance_contestants(instance_id, contestant_id)
        ON DELETE CASCADE
);

CREATE TABLE outcome_positions (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position > 0),
    contestant_id BIGINT REFERENCES contestants(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, position),
    FOREIGN KEY (instance_id, contestant_id)
        REFERENCES instance_contestants(instance_id, contestant_id)
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX outcome_positions_unique_contestant_per_instance
    ON outcome_positions(instance_id, contestant_id)
    WHERE contestant_id IS NOT NULL;

INSERT INTO instances (public_id, name, season, created_at)
SELECT id, name, season, created_at
FROM instances_legacy;

INSERT INTO contestants (public_id, name, created_at)
SELECT id, name, created_at
FROM contestants_legacy;

INSERT INTO participants (public_id, instance_id, name, created_at)
SELECT p.id, i.id, p.name, p.created_at
FROM participants_legacy p
JOIN instances i ON i.public_id = p.instance_id;

INSERT INTO instance_contestants (instance_id, contestant_id, created_at)
SELECT i.id, c.id, ic.created_at
FROM instance_contestants_legacy ic
JOIN instances i ON i.public_id = ic.instance_id
JOIN contestants c ON c.public_id = ic.contestant_id;

INSERT INTO draft_picks (instance_id, participant_id, contestant_id, position, created_at)
SELECT i.id, p.id, c.id, dp.position, dp.created_at
FROM draft_picks_legacy dp
JOIN instances i ON i.public_id = dp.instance_id
JOIN participants p ON p.public_id = dp.participant_id
JOIN contestants c ON c.public_id = dp.contestant_id;

INSERT INTO outcome_positions (instance_id, position, contestant_id, updated_at)
SELECT
    i.id,
    op.position,
    c.id,
    op.updated_at
FROM outcome_positions_legacy op
JOIN instances i ON i.public_id = op.instance_id
LEFT JOIN contestants c ON c.public_id = op.contestant_id;

DROP TABLE draft_picks_legacy;
DROP TABLE outcome_positions_legacy;
DROP TABLE instance_contestants_legacy;
DROP TABLE participants_legacy;
DROP TABLE contestants_legacy;
DROP TABLE instances_legacy;
