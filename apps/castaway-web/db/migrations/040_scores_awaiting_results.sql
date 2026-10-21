-- An automated scores post drafted before its game is scored carries a placeholder for the results. It's held
-- at send time (never sent with the placeholder) until automation redrafts it with the results.
ALTER TABLE announcements
    ADD COLUMN awaiting_results BOOLEAN NOT NULL DEFAULT false;
