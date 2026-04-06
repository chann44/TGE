-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS repository_scan_runs (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL,
    policy_id BIGINT REFERENCES policies(id) ON DELETE SET NULL,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    findings_total INTEGER NOT NULL DEFAULT 0,
    findings_critical INTEGER NOT NULL DEFAULT 0,
    findings_high INTEGER NOT NULL DEFAULT 0,
    findings_medium INTEGER NOT NULL DEFAULT 0,
    findings_low INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('queued', 'running', 'success', 'failed')),
    CHECK (trigger IN ('manual', 'policy', 'schedule', 'connect', 'sync'))
);

CREATE TABLE IF NOT EXISTS repository_scan_findings (
    id BIGSERIAL PRIMARY KEY,
    scan_run_id BIGINT NOT NULL REFERENCES repository_scan_runs(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL,
    policy_id BIGINT REFERENCES policies(id) ON DELETE SET NULL,
    package_id BIGINT,
    package_name TEXT NOT NULL,
    manager TEXT NOT NULL,
    registry TEXT NOT NULL,
    version_spec TEXT NOT NULL DEFAULT '',
    resolved_version TEXT NOT NULL DEFAULT '',
    advisory_id TEXT NOT NULL,
    aliases TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    severity severity NOT NULL DEFAULT 'medium',
    fixed_version TEXT NOT NULL DEFAULT '',
    reference_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('open', 'resolved')),
    UNIQUE (scan_run_id, package_name, advisory_id)
);

CREATE TABLE IF NOT EXISTS repository_scan_finding_sources (
    finding_id BIGINT NOT NULL REFERENCES repository_scan_findings(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    provider_record_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (finding_id, source),
    CHECK (source IN ('osv', 'ghsa', 'nvd', 'custom'))
);

CREATE INDEX IF NOT EXISTS repository_scan_runs_repository_id_idx
ON repository_scan_runs (repository_id, started_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_runs_status_idx
ON repository_scan_runs (status, started_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_findings_repository_id_idx
ON repository_scan_findings (repository_id, created_at DESC);

CREATE INDEX IF NOT EXISTS repository_scan_findings_scan_run_id_idx
ON repository_scan_findings (scan_run_id);

CREATE INDEX IF NOT EXISTS repository_scan_findings_advisory_idx
ON repository_scan_findings (advisory_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS repository_scan_findings_advisory_idx;
DROP INDEX IF EXISTS repository_scan_findings_scan_run_id_idx;
DROP INDEX IF EXISTS repository_scan_findings_repository_id_idx;
DROP INDEX IF EXISTS repository_scan_runs_status_idx;
DROP INDEX IF EXISTS repository_scan_runs_repository_id_idx;

DROP TABLE IF EXISTS repository_scan_finding_sources;
DROP TABLE IF EXISTS repository_scan_findings;
DROP TABLE IF EXISTS repository_scan_runs;
-- +goose StatementEnd
