-- Approval gate: a draft with approval_send_at waits for an instance admin's "yes" (via bot DM), then sends
-- at exactly that time. approval_claimed_at leases the DM to the bot; approval_notified_at records delivery.
-- approval_gated stays set once gated: the post sends only after approval of its exact text, and only within
-- five minutes of its scheduled time.
ALTER TABLE announcements
    ADD COLUMN approval_send_at TIMESTAMPTZ,
    ADD COLUMN approval_claimed_at TIMESTAMPTZ,
    ADD COLUMN approval_notified_at TIMESTAMPTZ,
    ADD COLUMN approval_gated BOOLEAN NOT NULL DEFAULT false;
