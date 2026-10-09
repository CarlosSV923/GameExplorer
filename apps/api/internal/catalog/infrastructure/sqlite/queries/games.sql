-- name: GetGame :one
SELECT id, console, title, folder, igdb_id, release_year, cover_image_id, summary, genres, created_at, updated_at
FROM games
WHERE id = ?;

-- name: GetGameByFolder :one
SELECT id, console, title, folder, igdb_id, release_year, cover_image_id, summary, genres, created_at, updated_at
FROM games
WHERE console = ? AND folder = ? COLLATE NOCASE;

-- name: ListGameItems :many
SELECT id, game_id, kind, label, file, size, source_job, created_at, trash_entry_id
FROM game_items
WHERE game_id = ?
ORDER BY id;

-- name: GetGameItem :one
SELECT id, game_id, kind, label, file, size, source_job, created_at, trash_entry_id
FROM game_items
WHERE id = ?;

-- name: ListGameSummaries :many
-- Games with at least one file outside the trash, by title.
SELECT g.id, g.console, g.igdb_id, g.title, g.folder, g.release_year, g.cover_image_id,
       COUNT(i.id) AS item_count, CAST(TOTAL(i.size) AS INTEGER) AS size
FROM games g
JOIN game_items i ON i.game_id = g.id AND i.trash_entry_id IS NULL
GROUP BY g.id
ORDER BY g.title COLLATE NOCASE, g.id;

-- name: ListLibraryFiles :many
SELECT i.id, i.game_id, g.console, g.folder, i.file
FROM game_items i
JOIN games g ON g.id = i.game_id
WHERE i.trash_entry_id IS NULL;

-- name: HasItemsFromSource :one
SELECT EXISTS (SELECT 1 FROM game_items WHERE source_job = ?);

-- name: InsertGame :one
INSERT INTO games (console, title, folder, igdb_id, release_year, cover_image_id, summary, genres, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateGame :exec
UPDATE games
SET console = ?, title = ?, folder = ?, igdb_id = ?, release_year = ?, cover_image_id = ?, summary = ?, genres = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteGame :exec
DELETE FROM games WHERE id = ?;

-- name: CountGameItems :one
SELECT COUNT(*) FROM game_items WHERE game_id = ?;

-- name: InsertGameItem :one
INSERT INTO game_items (game_id, kind, label, file, size, source_job, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateGameItem :exec
UPDATE game_items SET game_id = ?, kind = ?, label = ?, file = ? WHERE id = ?;

-- name: MoveGameItems :exec
UPDATE game_items SET game_id = sqlc.arg(to_game) WHERE game_id = sqlc.arg(from_game);

-- name: SetItemTrash :exec
UPDATE game_items SET trash_entry_id = ? WHERE id = ?;

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
SELECT id, game_id, kind, label, file, size, source_job, created_at, trash_entry_id
FROM game_items
WHERE trash_entry_id IS NOT NULL
ORDER BY id;

-- name: ListTrashUnassigned :many
SELECT id, path, origin, reason, size, arrived_at, trash_entry_id, console, igdb_id, job_id
FROM unassigned_files
WHERE trash_entry_id IS NOT NULL
ORDER BY id;

-- name: DeleteTrashEntry :exec
DELETE FROM trash_entries WHERE id = ?;

-- name: MoveTrashEntries :exec
UPDATE trash_entries SET game_id = sqlc.arg(to_game) WHERE game_id = sqlc.arg(from_game);

-- name: ListUnassigned :many
SELECT id, path, origin, reason, size, arrived_at, trash_entry_id, console, igdb_id, job_id
FROM unassigned_files
WHERE trash_entry_id IS NULL
ORDER BY arrived_at DESC, id DESC;

-- name: GetUnassigned :one
SELECT id, path, origin, reason, size, arrived_at, trash_entry_id, console, igdb_id, job_id
FROM unassigned_files
WHERE id = ?;

-- name: InsertUnassigned :one
INSERT INTO unassigned_files (path, origin, reason, size, arrived_at, trash_entry_id, console, igdb_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: SetUnassignedPlace :exec
UPDATE unassigned_files SET path = ?, trash_entry_id = ? WHERE id = ?;

-- name: DeleteUnassigned :exec
DELETE FROM unassigned_files WHERE id = ?;

-- name: ListScanPending :many
SELECT path, size, mod_time FROM scan_pending;

-- name: ClearScanPending :exec
DELETE FROM scan_pending;

-- name: InsertScanPending :exec
INSERT INTO scan_pending (path, size, mod_time) VALUES (?, ?, ?);

-- name: InsertOperation :exec
INSERT INTO library_operations (id, source, moves, created_dirs, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListOperations :many
SELECT id, source, moves, created_dirs, created_at
FROM library_operations
ORDER BY created_at, id;

-- name: DeleteOperation :exec
DELETE FROM library_operations WHERE id = ?;

-- name: SetUnassignedJob :exec
UPDATE unassigned_files SET job_id = ? WHERE id = ?;

-- name: ReleaseUnassignedJob :exec
UPDATE unassigned_files SET job_id = '' WHERE job_id = ?;
