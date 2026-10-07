-- +goose Up
-- The six consoles the MVP starts with (spec RF-40). IGDB platform ids:
-- https://api-docs.igdb.com/#platform. Logos and release years are refreshed
-- from IGDB in phase 2.
INSERT INTO consoles (slug, display_name, igdb_platform_id, release_year, extensions, detector_key, sort_order) VALUES
    ('switch', 'Nintendo Switch',   130, 2017, '[".nsp",".xci",".nsz",".xcz"]', 'switch', 10),
    ('wii',    'Wii',                 5, 2006, '[".iso",".wbfs",".rvz"]',        'wii',    20),
    ('gc',     'Nintendo GameCube',  21, 2001, '[".iso",".gcm",".ciso",".rvz"]', 'gc',     30),
    ('psx',    'PlayStation',         7, 1994, '[".cue",".bin",".chd",".pbp"]',  'psx',    40),
    ('ps2',    'PlayStation 2',       8, 2000, '[".iso",".chd",".bin",".cue"]',  'ps2',    50),
    ('ps3',    'PlayStation 3',       9, 2006, '[".iso",".pkg"]',                'ps3',    60);

-- +goose Down
DELETE FROM consoles WHERE slug IN ('switch', 'wii', 'gc', 'psx', 'ps2', 'ps3');
