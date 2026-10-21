-- Pink joins the tribe colors (🩷). Postgres names an inline column CHECK <table>_<column>_check.
ALTER TABLE participant_groups DROP CONSTRAINT participant_groups_color_check,
    ADD CONSTRAINT participant_groups_color_check CHECK (color IN ('red', 'orange', 'yellow', 'green', 'blue', 'purple', 'pink', 'brown', 'black', 'white'));
