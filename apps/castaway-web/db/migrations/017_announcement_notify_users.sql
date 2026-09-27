-- When true the bot lets user mentions (<@id>) notify; @everyone and roles never do.
ALTER TABLE announcements ADD COLUMN notify_users BOOLEAN NOT NULL DEFAULT false;
