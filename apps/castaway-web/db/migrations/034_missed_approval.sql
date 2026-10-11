-- When a held post's send time passes without approval, admins get one more DM saying so (reply "yes" to
-- post it now). This records that the missed-time DM went out.
ALTER TABLE announcements ADD COLUMN approval_missed_notified_at TIMESTAMPTZ;
