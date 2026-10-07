-- +goose Up
CREATE TABLE consoles (
    id               INTEGER PRIMARY KEY,
    -- Folder name inside the library (ES-DE style, e.g. "ps2").
    slug             TEXT    NOT NULL UNIQUE,
    display_name     TEXT    NOT NULL,
    igdb_platform_id INTEGER UNIQUE,
    release_year     INTEGER,
    logo_image_id    TEXT,
    -- JSON array of lower-case extensions including the dot, e.g. [".iso"].
    extensions       TEXT    NOT NULL DEFAULT '[]',
    -- Built-in consoles have a header detector; user-added ones rely on extensions.
    detector_key     TEXT,
    sort_order       INTEGER NOT NULL,
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose Down
DROP TABLE consoles;
