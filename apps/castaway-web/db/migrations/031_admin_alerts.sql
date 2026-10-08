-- One-time DMs to a season's admins (e.g. "next week has no game"). The key makes raising the same alert
-- again a no-op, so a scheduled checker can run every few minutes.
CREATE TABLE admin_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id BIGINT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    alert_key TEXT NOT NULL CHECK (char_length(alert_key) BETWEEN 1 AND 120),
    body TEXT NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 1900),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    UNIQUE (instance_id, alert_key)
);
