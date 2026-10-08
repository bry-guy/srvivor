-- Season automation: survivoR episode imports, frozen weekly standings, and scores posts that stay tied
-- to the standings they were written from.

-- One row per imported episode: what it recorded and from which upstream revision (provenance only; a
-- replay from a newer revision with the same results is a no-op, different results are a conflict).
CREATE TABLE episode_imports (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    episode_number INTEGER NOT NULL CHECK (episode_number >= 0),
    source TEXT NOT NULL CHECK (source IN ('survivor', 'manual')),
    source_revision TEXT NOT NULL DEFAULT '',
    boot_positions INTEGER[] NOT NULL DEFAULT '{}',
    challenge_keys TEXT[] NOT NULL DEFAULT '{}',
    fingerprint TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, episode_number)
);

-- The public leaderboard as a week's scores post was written from; the next week's movers compare
-- against it. Retaken while that week's post is unsent, frozen once it's sent.
CREATE TABLE score_snapshots (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    week INTEGER NOT NULL CHECK (week > 0),
    fingerprint TEXT NOT NULL,
    leaderboard JSONB NOT NULL,
    taken_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (instance_id, week)
);

-- An automated scores post records the standings it describes (score_fingerprint) and the text automation
-- wrote (auto_body_md5; differs from md5(body) once an admin edits it). If standings move, it can't be
-- approved or sent until it's redrafted.
ALTER TABLE announcements
    ADD COLUMN score_fingerprint TEXT,
    ADD COLUMN auto_body_md5 TEXT;
