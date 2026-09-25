-- Drafts are saved but never sent until scheduled.
ALTER TABLE announcements DROP CONSTRAINT announcements_status_check;
ALTER TABLE announcements ADD CONSTRAINT announcements_status_check CHECK (status IN ('draft', 'pending', 'sending', 'sent', 'failed'));
