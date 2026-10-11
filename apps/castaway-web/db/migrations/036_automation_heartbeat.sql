-- Each successful scheduled run stamps its season config; if that goes stale admins get one DM per stall.
ALTER TABLE season_configs ADD COLUMN heartbeat_at TIMESTAMPTZ;
