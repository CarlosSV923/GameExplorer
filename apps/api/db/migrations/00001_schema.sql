-- +goose Up
-- Phase 10 schema (scope adjustment, docs/spec.md). The consoles themselves
-- are defined in code (internal/catalog/domain/consoles); the database only
-- keeps what the user changes and what IGDB says about them.
CREATE TABLE console_settings (
    slug          TEXT    PRIMARY KEY,
    -- NULL keeps the name defined in code.
    display_name  TEXT,
    sort_order    INTEGER NOT NULL,
    logo_image_id TEXT,
    release_year  INTEGER
);

-- Extensions added from the app (RF-41); the fixed ones come from the
-- environment or the code.
CREATE TABLE console_extensions (
    slug      TEXT NOT NULL,
    extension TEXT NOT NULL,
    PRIMARY KEY (slug, extension)
);

-- A game is a name within a console (RF-11), optionally linked to IGDB.
CREATE TABLE games (
    id             INTEGER PRIMARY KEY,
    console        TEXT    NOT NULL,
    title          TEXT    NOT NULL,
    -- Directory name inside the console folder: the sanitized title.
    folder         TEXT    NOT NULL,
    igdb_id        INTEGER,
    release_year   INTEGER,
    cover_image_id TEXT,
    summary        TEXT,
    genres         TEXT    NOT NULL DEFAULT '[]', -- JSON array
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL
);
CREATE UNIQUE INDEX games_folder ON games (console, folder COLLATE NOCASE);

-- Trash entries group what was sent to the trash together: one file, a whole
-- game, or unassigned files (game_id NULL). Their files live in trash/<dir>/.
CREATE TABLE trash_entries (
    id         INTEGER PRIMARY KEY,
    game_id    INTEGER REFERENCES games (id) ON DELETE CASCADE,
    whole_game INTEGER NOT NULL DEFAULT 0,
    reason     TEXT    NOT NULL, -- deleted | replaced
    dir        TEXT    NOT NULL UNIQUE,
    trashed_at TEXT    NOT NULL
);
CREATE INDEX trash_entries_trashed ON trash_entries (trashed_at);

-- Files of a game.
CREATE TABLE game_items (
    id             INTEGER PRIMARY KEY,
    game_id        INTEGER NOT NULL REFERENCES games (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL,            -- base | update | dlc | game
    label          TEXT    NOT NULL DEFAULT '', -- update version or DLC name
    file           TEXT    NOT NULL,            -- name inside the game folder
    size           INTEGER NOT NULL,
    source_job     TEXT    NOT NULL DEFAULT '',
    created_at     TEXT    NOT NULL,
    -- Set while the file is in the trash (RF-30).
    trash_entry_id INTEGER REFERENCES trash_entries (id)
);
CREATE INDEX game_items_game ON game_items (game_id);
CREATE INDEX game_items_source ON game_items (source_job);
CREATE INDEX game_items_trash ON game_items (trash_entry_id);

-- Files in _unassigned/ (RF-27).
CREATE TABLE unassigned_files (
    id             INTEGER PRIMARY KEY,
    -- Relative to _unassigned/, "/" separators.
    path           TEXT    NOT NULL,
    origin         TEXT    NOT NULL,
    reason         TEXT    NOT NULL, -- samba | upload | manual
    size           INTEGER NOT NULL,
    arrived_at     TEXT    NOT NULL,
    trash_entry_id INTEGER REFERENCES trash_entries (id)
);
CREATE UNIQUE INDEX unassigned_files_path ON unassigned_files (path COLLATE NOCASE) WHERE trash_entry_id IS NULL;
CREATE INDEX unassigned_files_trash ON unassigned_files (trash_entry_id);

-- Unknown files seen by the last scan: they move to _unassigned/ only when a
-- later scan finds them unchanged (not a copy in progress, RF-26).
CREATE TABLE scan_pending (
    path     TEXT    PRIMARY KEY, -- relative to the library root
    size     INTEGER NOT NULL,
    mod_time TEXT    NOT NULL
);

-- Journal of library changes in progress: undone on restart if the process
-- stopped before the change was recorded (RF-10).
CREATE TABLE library_operations (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    moves        TEXT NOT NULL, -- JSON [{from, to}], absolute paths, in order
    created_dirs TEXT NOT NULL, -- JSON array, absolute paths, in creation order
    created_at   TEXT NOT NULL
);

CREATE TABLE upload_jobs (
    -- The tus upload id (random hex).
    id             TEXT    PRIMARY KEY,
    file_name      TEXT    NOT NULL,
    size           INTEGER NOT NULL,
    received       INTEGER NOT NULL DEFAULT 0,
    status         TEXT    NOT NULL,
    error          TEXT    NOT NULL DEFAULT '',
    warning        TEXT    NOT NULL DEFAULT '',
    progress       INTEGER NOT NULL DEFAULT 0,
    console        TEXT    NOT NULL,
    title          TEXT    NOT NULL,
    igdb_id        INTEGER,
    invalid_reason TEXT    NOT NULL DEFAULT '',
    -- Parts of one multi-volume archive share a group (RF-03a).
    group_id       TEXT    NOT NULL DEFAULT '',
    group_size     INTEGER NOT NULL DEFAULT 0,
    merged_into    TEXT    NOT NULL DEFAULT '',
    storage_path   TEXT    NOT NULL DEFAULT '',
    -- Path inside _unassigned/ the job's file came from (RF-27).
    unassigned_origin TEXT NOT NULL DEFAULT '',
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL
);
CREATE INDEX upload_jobs_status_updated ON upload_jobs (status, updated_at);
CREATE INDEX upload_jobs_created ON upload_jobs (created_at);
CREATE INDEX upload_jobs_group ON upload_jobs (group_id, status);

-- Files found in an upload (RF-07).
CREATE TABLE staged_files (
    job_id TEXT    NOT NULL REFERENCES upload_jobs (id) ON DELETE CASCADE,
    -- Relative to the job's staging directory.
    path   TEXT    NOT NULL,
    size   INTEGER NOT NULL,
    PRIMARY KEY (job_id, path)
);

-- +goose Down
DROP TABLE staged_files;
DROP TABLE upload_jobs;
DROP TABLE library_operations;
DROP TABLE scan_pending;
DROP TABLE unassigned_files;
DROP TABLE game_items;
DROP TABLE trash_entries;
DROP TABLE games;
DROP TABLE console_extensions;
DROP TABLE console_settings;
