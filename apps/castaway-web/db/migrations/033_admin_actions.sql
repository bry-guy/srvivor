-- An admin alert can carry an action the admins approve by replying "yes" (e.g. an episode import that
-- needs judgment). Raising the alert again with a different action replaces it and bumps the revision, so
-- a "yes" to the old version is refused. The server runs exactly the stored action on approval.
ALTER TABLE admin_alerts
    ADD COLUMN action_kind TEXT CHECK (action_kind IN ('episode_import')),
    ADD COLUMN action JSONB,
    ADD COLUMN revision TEXT,
    ADD COLUMN applied_at TIMESTAMPTZ,
    ADD COLUMN applied_by TEXT,
    ADD CONSTRAINT admin_alerts_action_complete CHECK ((action_kind IS NULL) = (action IS NULL) AND (action IS NULL) = (revision IS NULL));
