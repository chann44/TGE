-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS repository_scan_logs (
    id BIGSERIAL PRIMARY KEY,
    scan_run_id BIGINT NOT NULL REFERENCES repository_scan_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL,
    level TEXT NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    directory_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (level IN ('debug', 'info', 'warn', 'error'))
);

CREATE INDEX IF NOT EXISTS repository_scan_logs_run_id_idx
ON repository_scan_logs (scan_run_id, created_at ASC);

CREATE INDEX IF NOT EXISTS repository_scan_logs_repository_id_idx
ON repository_scan_logs (repository_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS repository_scan_logs_repository_id_idx;
DROP INDEX IF EXISTS repository_scan_logs_run_id_idx;
DROP TABLE IF EXISTS repository_scan_logs;
-- +goose StatementEnd
