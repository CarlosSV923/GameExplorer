-- name: ListConsoles :many
SELECT id, slug, display_name, igdb_platform_id, release_year, logo_image_id, extensions, detector_key, sort_order,
       (SELECT COUNT(DISTINCT g.id)
        FROM games g
        JOIN game_items i ON i.game_id = g.id AND i.trash_entry_id IS NULL
        WHERE g.console_id = consoles.id) AS game_count
FROM consoles
ORDER BY sort_order, id;

-- name: UpdateConsolePlatformMetadata :exec
UPDATE consoles
SET logo_image_id = sqlc.narg(logo_image_id),
    release_year  = COALESCE(sqlc.narg(release_year), release_year)
WHERE id = sqlc.arg(id);

-- name: InsertConsole :one
INSERT INTO consoles (slug, display_name, igdb_platform_id, release_year, logo_image_id, extensions, sort_order)
VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM consoles))
RETURNING id;

-- name: UpdateConsole :exec
UPDATE consoles SET slug = ?, display_name = ?, extensions = ? WHERE id = ?;

-- name: DeleteConsole :exec
DELETE FROM consoles WHERE id = ?;

-- name: SetConsoleOrder :exec
UPDATE consoles SET sort_order = ? WHERE id = ?;

-- name: ConsoleHasGames :one
-- Any game, even one whose items are all in the trash.
SELECT EXISTS (SELECT 1 FROM games WHERE console_id = ?);
