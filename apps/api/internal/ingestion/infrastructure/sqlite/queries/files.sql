-- name: DeleteFiles :exec
DELETE FROM staged_files WHERE job_id = ?;

-- name: InsertFile :exec
INSERT INTO staged_files (job_id, path, size) VALUES (?, ?, ?);

-- name: ListFiles :many
SELECT path, size FROM staged_files WHERE job_id = ? ORDER BY path;
