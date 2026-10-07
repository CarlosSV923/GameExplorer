-- +goose Up
-- Trash entries group what was sent to the trash together: one item, or a
-- whole game (spec RF-25, RF-30). Their files live in trash/<dir>/.
CREATE TABLE trash_entries (
    id         INTEGER PRIMARY KEY,
    game_id    INTEGER NOT NULL REFERENCES games (id) ON DELETE CASCADE,
    whole_game INTEGER NOT NULL DEFAULT 0,
    reason     TEXT    NOT NULL, -- deleted | replaced
    dir        TEXT    NOT NULL UNIQUE,
    trashed_at TEXT    NOT NULL
);
CREATE INDEX trash_entries_trashed ON trash_entries (trashed_at);

-- No REFERENCES: SQLite cannot drop a column that is part of a foreign key.
ALTER TABLE game_items ADD COLUMN trash_entry_id INTEGER;
-- Set by the integrity check while a file is missing from disk (RF-26).
ALTER TABLE game_items ADD COLUMN missing_since TEXT;
CREATE INDEX game_items_trash ON game_items (trash_entry_id);

-- Items replaced in phase 5 each become an entry of their own.
INSERT INTO trash_entries (game_id, whole_game, reason, dir, trashed_at)
SELECT game_id, 0, 'replaced', trash_dir, trashed_at FROM game_items WHERE trashed_at IS NOT NULL;
UPDATE game_items
SET trash_entry_id = (SELECT e.id FROM trash_entries e WHERE e.dir = game_items.trash_dir)
WHERE trashed_at IS NOT NULL;

ALTER TABLE game_items DROP COLUMN trashed_at;
ALTER TABLE game_items DROP COLUMN trash_dir;

-- +goose Down
ALTER TABLE game_items ADD COLUMN trashed_at TEXT;
ALTER TABLE game_items ADD COLUMN trash_dir TEXT NOT NULL DEFAULT '';
UPDATE game_items
SET trashed_at = (SELECT e.trashed_at FROM trash_entries e WHERE e.id = game_items.trash_entry_id),
    trash_dir  = (SELECT e.dir FROM trash_entries e WHERE e.id = game_items.trash_entry_id)
WHERE trash_entry_id IS NOT NULL;
DROP INDEX game_items_trash;
ALTER TABLE game_items DROP COLUMN missing_since;
ALTER TABLE game_items DROP COLUMN trash_entry_id;
DROP TABLE trash_entries;
