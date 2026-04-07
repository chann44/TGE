-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS repository_finding_occurrences (
    repository_id BIGINT NOT NULL,
    finding_id BIGINT NOT NULL REFERENCES repository_scan_findings(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'open',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (repository_id, finding_id),
    CHECK (status IN ('open', 'resolved'))
);

INSERT INTO repository_finding_occurrences (
    repository_id,
    finding_id,
    status,
    first_seen_at,
    last_seen_at,
    created_at,
    updated_at
)
SELECT
    s.repository_id,
    srf.finding_id,
    'open' AS status,
    MIN(COALESCE(s.started_at, s.created_at, NOW())) AS first_seen_at,
    MAX(COALESCE(s.finished_at, s.started_at, s.updated_at, NOW())) AS last_seen_at,
    MIN(COALESCE(s.created_at, NOW())) AS created_at,
    MAX(COALESCE(s.updated_at, NOW())) AS updated_at
FROM repository_scan_run_findings srf
INNER JOIN repository_scan_runs s ON s.id = srf.scan_run_id
GROUP BY s.repository_id, srf.finding_id
ON CONFLICT (repository_id, finding_id)
DO UPDATE SET
    first_seen_at = LEAST(repository_finding_occurrences.first_seen_at, EXCLUDED.first_seen_at),
    last_seen_at = GREATEST(repository_finding_occurrences.last_seen_at, EXCLUDED.last_seen_at),
    updated_at = NOW();

WITH ranked AS (
    SELECT
        id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
), duplicates AS (
    SELECT id AS duplicate_id, keep_id
    FROM ranked
    WHERE row_num > 1
)
INSERT INTO repository_scan_run_findings (scan_run_id, finding_id)
SELECT srf.scan_run_id, d.keep_id
FROM repository_scan_run_findings srf
INNER JOIN duplicates d ON d.duplicate_id = srf.finding_id
ON CONFLICT (scan_run_id, finding_id) DO NOTHING;

WITH ranked AS (
    SELECT
        id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
), duplicates AS (
    SELECT id AS duplicate_id, keep_id
    FROM ranked
    WHERE row_num > 1
)
INSERT INTO repository_scan_finding_sources (finding_id, source, provider_record_id, created_at)
SELECT DISTINCT ON (d.keep_id, s.source)
    d.keep_id,
    s.source,
    s.provider_record_id,
    s.created_at
FROM repository_scan_finding_sources s
INNER JOIN duplicates d ON d.duplicate_id = s.finding_id
ORDER BY d.keep_id, s.source, s.created_at DESC, s.provider_record_id DESC
ON CONFLICT (finding_id, source)
DO UPDATE SET provider_record_id = EXCLUDED.provider_record_id;

WITH ranked AS (
    SELECT
        id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
), duplicates AS (
    SELECT id AS duplicate_id, keep_id
    FROM ranked
    WHERE row_num > 1
)
INSERT INTO repository_finding_occurrences (
    repository_id,
    finding_id,
    status,
    first_seen_at,
    last_seen_at,
    created_at,
    updated_at
)
SELECT
    rfo.repository_id,
    d.keep_id,
    rfo.status,
    rfo.first_seen_at,
    rfo.last_seen_at,
    rfo.created_at,
    rfo.updated_at
FROM repository_finding_occurrences rfo
INNER JOIN duplicates d ON d.duplicate_id = rfo.finding_id
ON CONFLICT (repository_id, finding_id)
DO UPDATE SET
    first_seen_at = LEAST(repository_finding_occurrences.first_seen_at, EXCLUDED.first_seen_at),
    last_seen_at = GREATEST(repository_finding_occurrences.last_seen_at, EXCLUDED.last_seen_at),
    updated_at = NOW();

WITH ranked AS (
    SELECT
        id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        ROW_NUMBER() OVER (
            PARTITION BY manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
)
DELETE FROM repository_scan_findings f
USING ranked r
WHERE f.id = r.id
  AND r.row_num > 1;

DROP INDEX IF EXISTS repository_scan_findings_dedupe_idx;

ALTER TABLE repository_scan_findings
DROP CONSTRAINT IF EXISTS repository_scan_findings_repository_id_manager_registry_package_name_resolved_ve_key;

ALTER TABLE repository_scan_findings
DROP CONSTRAINT IF EXISTS repository_scan_findings_manager_registry_package_name_resolved_version_advi_key;

CREATE UNIQUE INDEX IF NOT EXISTS repository_scan_findings_global_dedupe_idx
ON repository_scan_findings (manager, registry, package_name, resolved_version, advisory_id);

CREATE INDEX IF NOT EXISTS repository_finding_occurrences_finding_id_idx
ON repository_finding_occurrences (finding_id);

CREATE INDEX IF NOT EXISTS repository_finding_occurrences_repository_id_idx
ON repository_finding_occurrences (repository_id, last_seen_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS repository_finding_occurrences_repository_id_idx;
DROP INDEX IF EXISTS repository_finding_occurrences_finding_id_idx;
DROP INDEX IF EXISTS repository_scan_findings_global_dedupe_idx;

CREATE UNIQUE INDEX IF NOT EXISTS repository_scan_findings_dedupe_idx
ON repository_scan_findings (repository_id, manager, registry, package_name, resolved_version, advisory_id);

DROP TABLE IF EXISTS repository_finding_occurrences;
-- +goose StatementEnd
