-- Approval gate: a draft with approval_send_at waits for an instance admin's "yes" (via bot DM), then
-- sends at that time. approval_notified_at records when the DM went out.
ALTER TABLE announcements
    ADD COLUMN approval_send_at TIMESTAMPTZ,
    ADD COLUMN approval_notified_at TIMESTAMPTZ;
