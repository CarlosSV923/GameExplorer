-- name: ListConsoles :many
SELECT id, slug, display_name, igdb_platform_id, release_year, logo_image_id, extensions, detector_key, sort_order
FROM consoles
ORDER BY sort_order, id;

-- name: UpdateConsolePlatformMetadata :exec
UPDATE consoles
SET logo_image_id = sqlc.narg(logo_image_id),
    release_year  = COALESCE(sqlc.narg(release_year), release_year)
WHERE id = sqlc.arg(id);
