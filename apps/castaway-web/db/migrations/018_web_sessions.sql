-- Discord-login sessions for the public site (browser cookies) and probst (CLI bearer tokens).
-- Only SHA-256 hashes of tokens are stored.
CREATE TABLE web_sessions (
    id BIGSERIAL PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    discord_user_id TEXT NOT NULL,
    discord_username TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('browser', 'cli')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX web_sessions_discord_user_id_idx ON web_sessions(discord_user_id);

-- One-time codes handed to probst's loopback listener and exchanged for a CLI session.
CREATE TABLE web_cli_codes (
    code_hash BYTEA PRIMARY KEY,
    discord_user_id TEXT NOT NULL,
    discord_username TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL
);

-- Signed-in Discord users who aren't linked to a player yet.
CREATE TABLE access_requests (
    discord_user_id TEXT PRIMARY KEY,
    discord_username TEXT NOT NULL DEFAULT '',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at TIMESTAMPTZ
);
