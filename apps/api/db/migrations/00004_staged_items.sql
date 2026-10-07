-- +goose Up
ALTER TABLE upload_jobs ADD COLUMN progress INTEGER NOT NULL DEFAULT 0;
ALTER TABLE upload_jobs ADD COLUMN warning  TEXT    NOT NULL DEFAULT '';

-- Reviewable units found in an upload (spec RF-06/RF-07).
CREATE TABLE staged_items (
    job_id          TEXT    NOT NULL REFERENCES upload_jobs (id) ON DELETE CASCADE,
    -- Relative to the job's staging directory; for discs, the .cue sheet.
    path            TEXT    NOT NULL,
    shape           TEXT    NOT NULL,           -- file | disc | folder
    parts           TEXT    NOT NULL,           -- JSON array of relative paths
    size            INTEGER NOT NULL,
    ignored         INTEGER NOT NULL DEFAULT 0, -- junk listed for transparency
    consoles        TEXT    NOT NULL,           -- JSON array of candidate slugs
    confidence      TEXT    NOT NULL,           -- header | extension | none
    suggested_kind  TEXT    NOT NULL DEFAULT '',
    title_id        TEXT    NOT NULL DEFAULT '',
    version_code    TEXT    NOT NULL DEFAULT '',
    display_version TEXT    NOT NULL DEFAULT '',
    disc_number     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (job_id, path)
);

-- +goose Down
DROP TABLE staged_items;
ALTER TABLE upload_jobs DROP COLUMN warning;
ALTER TABLE upload_jobs DROP COLUMN progress;
