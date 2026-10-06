-- Press the Button scoring needs when each player first and last pressed (first/last presser) and which
-- Eastern days they pressed on (streaks).
ALTER TABLE button_presses
    ADD COLUMN first_press_at TIMESTAMPTZ,
    ADD COLUMN last_press_at TIMESTAMPTZ,
    ADD COLUMN press_days DATE[] NOT NULL DEFAULT '{}';
