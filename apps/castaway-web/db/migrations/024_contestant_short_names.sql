-- Short names as on the show (survivoR `castaway`, pinned d4a75af9), used where drafts are displayed.
-- Earlier seasons were stored with short names already, so they stay NULL and fall back to name.
ALTER TABLE contestants ADD COLUMN short_name TEXT;

UPDATE contestants c SET short_name = v.short_name
FROM (VALUES
    ('Aaliyah Puglia', 'Aaliyah'),
    ('Alexis Levine', 'Alexis'),
    ('Ana Sani', 'Ana'),
    ('Angelica "Jelly" Loblack', 'Jelly'),
    ('Brady Booker', 'Brady'),
    ('Carter Krull', 'Carter'),
    ('Cristian Chavez', 'Cristian'),
    ('Danny "Kilby" Kilby', 'Danny'),
    ('Devin Way', 'Devin'),
    ('Eric Macksoud', 'Eric'),
    ('Jenna Doore', 'Jenna'),
    ('Kristin Flickinger', 'Kristin'),
    ('Lewis Kelly', 'Lewis'),
    ('Linnea Capobianco', 'Linnea'),
    ('Maggie Nestor', 'Maggie'),
    ('Mike Pinsky', 'Mike'),
    ('Ori Jean-Charles', 'Ori'),
    ('Patt Cannaday', 'Patt'),
    ('Rob Antonson', 'Rob'),
    ('Sharonda Cox', 'Sharonda'),
    ('Thien "An" Nguyen', 'Thien An')
) AS v(name, short_name)
WHERE c.name = v.name;
