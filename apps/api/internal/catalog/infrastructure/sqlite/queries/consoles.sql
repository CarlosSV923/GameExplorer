-- name: ListConsoleSettings :many
SELECT slug, display_name, sort_order, logo_image_id, release_year FROM console_settings;

-- name: UpsertConsoleOrder :exec
INSERT INTO console_settings (slug, sort_order) VALUES (?, ?)
ON CONFLICT (slug) DO UPDATE SET sort_order = excluded.sort_order;

-- name: SetConsoleDisplayName :exec
UPDATE console_settings SET display_name = ? WHERE slug = ?;

-- name: SetConsolePlatformMetadata :exec
UPDATE console_settings
SET logo_image_id = sqlc.narg(logo_image_id),
    release_year  = COALESCE(sqlc.narg(release_year), release_year)
WHERE slug = sqlc.arg(slug);

-- name: ListConsoleExtensions :many
SELECT slug, extension FROM console_extensions ORDER BY slug, rowid;

-- name: AddConsoleExtension :exec
INSERT OR IGNORE INTO console_extensions (slug, extension) VALUES (?, ?);

-- name: RemoveConsoleExtension :exec
DELETE FROM console_extensions WHERE slug = ? AND extension = ?;

-- name: CountGamesByConsole :many
-- Games with at least one file outside the trash.
SELECT g.console, COUNT(DISTINCT g.id) AS games
FROM games g
JOIN game_items i ON i.game_id = g.id AND i.trash_entry_id IS NULL
GROUP BY g.console;

-- name: ListConsoleItemFiles :many
SELECT i.file FROM game_items i JOIN games g ON g.id = i.game_id WHERE g.console = ?;
