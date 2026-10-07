-- +goose Up
-- Games of each console, matched to IGDB (spec RF-10, RF-11).
CREATE TABLE games (
    id             INTEGER PRIMARY KEY,
    console_id     INTEGER NOT NULL REFERENCES consoles (id),
    igdb_id        INTEGER NOT NULL,
    title          TEXT    NOT NULL,
    -- Directory name inside the console folder.
    folder         TEXT    NOT NULL,
    release_year   INTEGER,
    cover_image_id TEXT,
    summary        TEXT,
    genres         TEXT    NOT NULL DEFAULT '[]', -- JSON array
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL,
    UNIQUE (console_id, igdb_id)
);
CREATE UNIQUE INDEX games_folder ON games (console_id, folder COLLATE NOCASE);

-- Stored items: base game, update, DLC or disc.
CREATE TABLE game_items (
    id          INTEGER PRIMARY KEY,
    game_id     INTEGER NOT NULL REFERENCES games (id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL,           -- base | update | dlc | disc
    label       TEXT    NOT NULL DEFAULT '', -- update version or DLC name
    disc_number INTEGER NOT NULL DEFAULT 0,
    shape       TEXT    NOT NULL,           -- file | disc | folder
    files       TEXT    NOT NULL,           -- JSON array, relative to the game folder
    size        INTEGER NOT NULL,
    title_id    TEXT    NOT NULL DEFAULT '',
    source_job  TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    -- Set while the item is in the trash (spec RF-30).
    trashed_at  TEXT,
    trash_dir   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX game_items_game ON game_items (game_id);
CREATE INDEX game_items_source ON game_items (source_job);

-- Journal of library changes in progress: undone on restart if the process
-- stopped before the change was recorded (RF-11).
CREATE TABLE library_operations (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    moves        TEXT NOT NULL, -- JSON [{from, to}], absolute paths, in order
    created_dirs TEXT NOT NULL, -- JSON array, absolute paths, in creation order
    created_at   TEXT NOT NULL
);

-- +goose Down
DROP TABLE library_operations;
DROP TABLE game_items;
DROP TABLE games;
