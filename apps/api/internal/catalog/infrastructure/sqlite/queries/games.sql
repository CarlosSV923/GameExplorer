-- name: FindGameByIGDB :one
SELECT id, console_id, igdb_id, title, folder, release_year, cover_image_id, summary, genres, created_at, updated_at
FROM games
WHERE console_id = ? AND igdb_id = ?;

-- name: GetGame :one
SELECT id, console_id, igdb_id, title, folder, release_year, cover_image_id, summary, genres, created_at, updated_at
FROM games
WHERE id = ?;

-- name: ListGameItems :many
SELECT id, game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at, trash_entry_id, missing_since
FROM game_items
WHERE game_id = ?
ORDER BY id;

-- name: GetGameItem :one
SELECT id, game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at, trash_entry_id, missing_since
FROM game_items
WHERE id = ?;

-- name: ListGameSummaries :many
-- Games with at least one item outside the trash, by title.
SELECT g.id, g.console_id, g.igdb_id, g.title, g.folder, g.release_year, g.cover_image_id,
       COUNT(i.id) AS item_count, CAST(TOTAL(i.size) AS INTEGER) AS size,
       COUNT(i.missing_since) AS missing_count
FROM games g
JOIN game_items i ON i.game_id = g.id AND i.trash_entry_id IS NULL
GROUP BY g.id
ORDER BY g.title COLLATE NOCASE, g.id;

-- name: FolderTaken :one
SELECT EXISTS (SELECT 1 FROM games WHERE console_id = ? AND folder = ? COLLATE NOCASE AND id != sqlc.arg(except_id));

-- name: HasItemsFromSource :one
SELECT EXISTS (SELECT 1 FROM game_items WHERE source_job = ?);

-- name: InsertGame :one
INSERT INTO games (console_id, igdb_id, title, folder, release_year, cover_image_id, summary, genres, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateGame :exec
UPDATE games
SET igdb_id = ?, title = ?, folder = ?, release_year = ?, cover_image_id = ?, summary = ?, genres = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteGame :exec
DELETE FROM games WHERE id = ?;

-- name: CountGameItems :one
SELECT COUNT(*) FROM game_items WHERE game_id = ?;

-- name: InsertGameItem :one
INSERT INTO game_items (game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateItemPlace :exec
UPDATE game_items SET game_id = ?, files = ? WHERE id = ?;

-- name: MoveGameItems :exec
UPDATE game_items SET game_id = sqlc.arg(to_game) WHERE game_id = sqlc.arg(from_game);

-- name: SetItemTrash :exec
UPDATE game_items SET trash_entry_id = ? WHERE id = ?;

-- name: SetItemMissing :exec
UPDATE game_items SET missing_since = ? WHERE id = ?;

-- name: DeleteGameItem :exec
DELETE FROM game_items WHERE id = ?;

-- name: InsertTrashEntry :one
INSERT INTO trash_entries (game_id, whole_game, reason, dir, trashed_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: GetTrashEntry :one
SELECT id, game_id, whole_game, reason, dir, trashed_at FROM trash_entries WHERE id = ?;

-- name: ListTrashEntries :many
SELECT id, game_id, whole_game, reason, dir, trashed_at FROM trash_entries ORDER BY trashed_at DESC, id DESC;

-- name: ListTrashItems :many
SELECT id, game_id, kind, label, disc_number, shape, files, size, title_id, source_job, created_at, trash_entry_id, missing_since
FROM game_items
WHERE trash_entry_id IS NOT NULL
ORDER BY id;

-- name: DeleteTrashEntry :exec
DELETE FROM trash_entries WHERE id = ?;

-- name: MoveTrashEntries :exec
UPDATE trash_entries SET game_id = sqlc.arg(to_game) WHERE game_id = sqlc.arg(from_game);

-- name: InsertOperation :exec
INSERT INTO library_operations (id, source, moves, created_dirs, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListOperations :many
SELECT id, source, moves, created_dirs, created_at
FROM library_operations
ORDER BY created_at, id;

-- name: DeleteOperation :exec
DELETE FROM library_operations WHERE id = ?;
