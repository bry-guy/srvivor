-- Each version of a message in a watched draft thread is handled once, so restarts and replays never
-- re-save drafts or re-send problem DMs.
CREATE TABLE draft_thread_messages (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL,
    version TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, version)
);
