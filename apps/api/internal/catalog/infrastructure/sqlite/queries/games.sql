-- name: FindGameByIGDB :one
SELECT id, console_id, igdb_id, title, folder, release_year, cover_image_id, summary, genres, created_at, updated_at
FROM games
WHERE console_id = ? AND igdb_id = ?;

-- name: ListGameItems :many
SELECT id, game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at, trashed_at, trash_dir
FROM game_items
WHERE game_id = ?
ORDER BY id;

-- name: FolderTaken :one
SELECT EXISTS (SELECT 1 FROM games WHERE console_id = ? AND folder = ? COLLATE NOCASE);

-- name: HasItemsFromSource :one
SELECT EXISTS (SELECT 1 FROM game_items WHERE source_job = ?);

-- name: InsertGame :one
INSERT INTO games (console_id, igdb_id, title, folder, release_year, cover_image_id, summary, genres, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateGameMetadata :exec
UPDATE games
SET title = ?, release_year = ?, cover_image_id = ?, summary = ?, genres = ?, updated_at = ?
WHERE id = ?;

-- name: InsertGameItem :exec
INSERT INTO game_items (game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: TrashGameItem :exec
UPDATE game_items
SET trashed_at = ?, trash_dir = ?
WHERE id = ? AND trashed_at IS NULL;

-- name: InsertOperation :exec
INSERT INTO library_operations (id, source, moves, created_dirs, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListOperations :many
SELECT id, source, moves, created_dirs, created_at
FROM library_operations
ORDER BY created_at, id;

-- name: DeleteOperation :exec
DELETE FROM library_operations WHERE id = ?;
