-- name: CreateJob :exec
INSERT INTO upload_jobs (id, file_name, size, received, status, error, warning, progress, console, title, igdb_id,
                         invalid_reason, group_id, group_size, merged_into, storage_path, unassigned_origin,
                         unassigned_from, unassigned_reason, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetJob :one
SELECT * FROM upload_jobs WHERE id = ?;

-- name: SaveJob :exec
UPDATE upload_jobs
SET received = ?, status = ?, error = ?, warning = ?, progress = ?, console = ?, invalid_reason = ?,
    merged_into = ?, storage_path = ?, updated_at = ?
WHERE id = ?;

-- name: ListJobs :many
SELECT * FROM upload_jobs ORDER BY created_at DESC LIMIT ?;

-- name: ListStaleJobs :many
SELECT * FROM upload_jobs WHERE status = ? AND updated_at < ? ORDER BY updated_at;

-- name: JobExists :one
SELECT EXISTS(SELECT 1 FROM upload_jobs WHERE id = ?);

-- name: ListGroup :many
SELECT * FROM upload_jobs WHERE group_id = ? ORDER BY created_at, id;
