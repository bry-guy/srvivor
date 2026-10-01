-- Pronouns for copy generation (announcements, recaps); not shown on the site.
ALTER TABLE participants
    ADD COLUMN pronouns TEXT CHECK (pronouns IN ('he/him', 'she/her', 'they/them'));
