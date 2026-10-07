-- A tribe's buff color, the one source for colored dots and emoji in posts (🟣 purple, 🟡 yellow, ...).
ALTER TABLE participant_groups
    ADD COLUMN color TEXT CHECK (color IN ('red', 'orange', 'yellow', 'green', 'blue', 'purple', 'brown', 'black', 'white'));

UPDATE participant_groups g SET color = CASE g.name WHEN 'Savu' THEN 'purple' WHEN 'Toka' THEN 'yellow' END
FROM instances i
WHERE i.id = g.instance_id AND i.season = 51 AND g.kind = 'tribe' AND g.name IN ('Savu', 'Toka');
