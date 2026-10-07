-- name: DeleteItems :exec
DELETE FROM staged_items WHERE job_id = ?;

-- name: InsertItem :exec
INSERT INTO staged_items (job_id, path, shape, parts, size, ignored, consoles, confidence,
                          suggested_kind, title_id, version_code, display_version, disc_number)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListItems :many
SELECT * FROM staged_items WHERE job_id = ? ORDER BY path;
