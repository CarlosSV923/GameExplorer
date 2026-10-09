-- +goose Up
-- Where an assigned unassigned file came from, to give it back unchanged
-- when its job is cancelled (RF-27).
ALTER TABLE upload_jobs ADD COLUMN unassigned_from TEXT NOT NULL DEFAULT '';
ALTER TABLE upload_jobs ADD COLUMN unassigned_reason TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE upload_jobs DROP COLUMN unassigned_reason;
ALTER TABLE upload_jobs DROP COLUMN unassigned_from;
