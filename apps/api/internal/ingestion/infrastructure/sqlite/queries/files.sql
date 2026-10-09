-- name: DeleteFiles :exec
DELETE FROM staged_files WHERE job_id = ?;

-- name: InsertFile :exec
INSERT INTO staged_files (job_id, path, size, unassigned_id) VALUES (?, ?, ?, ?);

-- name: ListFiles :many
SELECT path, size, unassigned_id FROM staged_files WHERE job_id = ? ORDER BY path;
