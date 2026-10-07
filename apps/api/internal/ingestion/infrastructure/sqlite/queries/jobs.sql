-- name: CreateJob :exec
INSERT INTO upload_jobs (id, file_name, size, received, status, error, origin_console, storage_path, progress, warning, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetJob :one
SELECT * FROM upload_jobs WHERE id = ?;

-- name: SaveJob :exec
UPDATE upload_jobs
SET received = ?, status = ?, error = ?, storage_path = ?, progress = ?, warning = ?, updated_at = ?
WHERE id = ?;

-- name: ListJobs :many
SELECT * FROM upload_jobs ORDER BY created_at DESC LIMIT ?;

-- name: ListStaleJobs :many
SELECT * FROM upload_jobs WHERE status = ? AND updated_at < ? ORDER BY updated_at;

-- name: JobExists :one
SELECT EXISTS(SELECT 1 FROM upload_jobs WHERE id = ?);
