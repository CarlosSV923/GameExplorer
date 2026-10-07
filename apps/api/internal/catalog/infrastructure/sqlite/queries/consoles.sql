-- name: ListConsoles :many
SELECT id, slug, display_name, igdb_platform_id, release_year, extensions, sort_order
FROM consoles
ORDER BY sort_order, id;
