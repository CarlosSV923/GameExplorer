-- +goose Up
-- Phase 10.5 (docs/spec.md RF-07b, RF-26, RF-27a): an unassigned file keeps
-- the console it came from and the IGDB game picked on upload, to prefill
-- Asignar; job_id marks the files an assignment in progress is using.
ALTER TABLE unassigned_files ADD COLUMN console TEXT NOT NULL DEFAULT '';
ALTER TABLE unassigned_files ADD COLUMN igdb_id INTEGER;
ALTER TABLE unassigned_files ADD COLUMN job_id TEXT NOT NULL DEFAULT '';
-- A staged file of an assignment that stays in _unassigned/ until it is
-- stored (path relative to _unassigned/); NULL for files in staging.
ALTER TABLE staged_files ADD COLUMN unassigned_id INTEGER;

-- +goose Down
ALTER TABLE staged_files DROP COLUMN unassigned_id;
ALTER TABLE unassigned_files DROP COLUMN job_id;
ALTER TABLE unassigned_files DROP COLUMN igdb_id;
ALTER TABLE unassigned_files DROP COLUMN console;
