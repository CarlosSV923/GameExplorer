-- +goose Up
-- Multi-volume archives: each part is its own upload job until the set is complete.
ALTER TABLE upload_jobs ADD COLUMN volume_set   TEXT    NOT NULL DEFAULT '';
ALTER TABLE upload_jobs ADD COLUMN volume_index INTEGER NOT NULL DEFAULT 0;
ALTER TABLE upload_jobs ADD COLUMN merged_into  TEXT    NOT NULL DEFAULT '';
CREATE INDEX upload_jobs_volume_set ON upload_jobs (volume_set, status);

-- +goose Down
DROP INDEX upload_jobs_volume_set;
ALTER TABLE upload_jobs DROP COLUMN merged_into;
ALTER TABLE upload_jobs DROP COLUMN volume_index;
ALTER TABLE upload_jobs DROP COLUMN volume_set;
