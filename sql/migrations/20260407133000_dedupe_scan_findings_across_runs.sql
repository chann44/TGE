-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS repository_scan_run_findings (
    scan_run_id BIGINT NOT NULL REFERENCES repository_scan_runs(id) ON DELETE CASCADE,
    finding_id BIGINT NOT NULL REFERENCES repository_scan_findings(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scan_run_id, finding_id)
);

INSERT INTO repository_scan_run_findings (scan_run_id, finding_id, created_at)
SELECT scan_run_id, id, created_at
FROM repository_scan_findings
ON CONFLICT (scan_run_id, finding_id) DO NOTHING;

WITH ranked AS (
    SELECT
        id,
        scan_run_id,
        repository_id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
), duplicates AS (
    SELECT id AS duplicate_id, keep_id, scan_run_id
    FROM ranked
    WHERE row_num > 1
)
INSERT INTO repository_scan_run_findings (scan_run_id, finding_id)
SELECT scan_run_id, keep_id
FROM duplicates
ON CONFLICT (scan_run_id, finding_id) DO NOTHING;

WITH ranked AS (
    SELECT
        id,
        repository_id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
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
        repository_id,
        manager,
        registry,
        package_name,
        resolved_version,
        advisory_id,
        FIRST_VALUE(id) OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS keep_id,
        ROW_NUMBER() OVER (
            PARTITION BY repository_id, manager, registry, package_name, resolved_version, advisory_id
            ORDER BY updated_at DESC, id DESC
        ) AS row_num
    FROM repository_scan_findings
)
DELETE FROM repository_scan_findings f
USING ranked r
WHERE f.id = r.id
  AND r.row_num > 1;

ALTER TABLE repository_scan_findings
DROP CONSTRAINT IF EXISTS repository_scan_findings_scan_run_id_package_name_advisory_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS repository_scan_findings_dedupe_idx
ON repository_scan_findings (repository_id, manager, registry, package_name, resolved_version, advisory_id);

CREATE INDEX IF NOT EXISTS repository_scan_run_findings_finding_id_idx
ON repository_scan_run_findings (finding_id);

CREATE INDEX IF NOT EXISTS repository_scan_run_findings_scan_run_id_idx
ON repository_scan_run_findings (scan_run_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS repository_scan_run_findings_scan_run_id_idx;
DROP INDEX IF EXISTS repository_scan_run_findings_finding_id_idx;
DROP INDEX IF EXISTS repository_scan_findings_dedupe_idx;

ALTER TABLE repository_scan_findings
ADD CONSTRAINT repository_scan_findings_scan_run_id_package_name_advisory_id_key
UNIQUE (scan_run_id, package_name, advisory_id);

DROP TABLE IF EXISTS repository_scan_run_findings;
-- +goose StatementEnd
