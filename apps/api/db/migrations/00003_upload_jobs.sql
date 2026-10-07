-- +goose Up
CREATE TABLE upload_jobs (
    -- The tus upload id (random hex).
    id             TEXT    PRIMARY KEY,
    file_name      TEXT    NOT NULL,
    size           INTEGER NOT NULL,
    received       INTEGER NOT NULL DEFAULT 0,
    status         TEXT    NOT NULL,
    error          TEXT    NOT NULL DEFAULT '',
    origin_console TEXT,
    storage_path   TEXT    NOT NULL DEFAULT '',
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL
);

CREATE INDEX upload_jobs_status_updated ON upload_jobs (status, updated_at);
CREATE INDEX upload_jobs_created ON upload_jobs (created_at);

-- +goose Down
DROP TABLE upload_jobs;
