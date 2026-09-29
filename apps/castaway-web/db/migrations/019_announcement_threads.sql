-- An announcement with a thread posts inside a bot-created thread shared by every announcement with the
-- same key in that channel. The first one posts the starter message and opens the thread.
ALTER TABLE announcements ADD COLUMN thread JSONB CHECK (thread IS NULL OR (thread ? 'key' AND thread ? 'name' AND thread ? 'starter'));

CREATE TABLE announcement_threads (
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL,
    thread_key TEXT NOT NULL,
    thread_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (instance_id, channel_id, thread_key)
);
