-- name: CreateRepositoryScanRun :one
INSERT INTO repository_scan_runs (
    repository_id,
    policy_id,
    trigger,
    status,
    error_message,
    started_at,
    finished_at,
    findings_total,
    findings_critical,
    findings_high,
    findings_medium,
    findings_low
) VALUES (
    $1,
    $2,
    $3,
    'queued',
    '',
    NOW(),
    NULL,
    0,
    0,
    0,
    0,
    0
)
RETURNING id, repository_id, policy_id, trigger, status, error_message, findings_total, findings_critical, findings_high, findings_medium, findings_low, started_at, finished_at, created_at, updated_at;

-- name: MarkRepositoryScanRunRunning :exec
UPDATE repository_scan_runs
SET status = 'running',
    error_message = '',
    started_at = NOW(),
    finished_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: MarkRepositoryScanRunSuccess :exec
UPDATE repository_scan_runs
SET status = 'success',
    error_message = '',
    findings_total = $2,
    findings_critical = $3,
    findings_high = $4,
    findings_medium = $5,
    findings_low = $6,
    finished_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: MarkRepositoryScanRunFailed :exec
UPDATE repository_scan_runs
SET status = 'failed',
    error_message = $2,
    finished_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: CreateRepositoryScanFinding :one
INSERT INTO repository_scan_findings (
    scan_run_id,
    repository_id,
    policy_id,
    package_id,
    package_name,
    manager,
    registry,
    version_spec,
    resolved_version,
    advisory_id,
    aliases,
    title,
    summary,
    severity,
    fixed_version,
    reference_url,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15,
    $16,
    'open'
)
ON CONFLICT (manager, registry, package_name, resolved_version, advisory_id)
DO UPDATE SET
    scan_run_id = EXCLUDED.scan_run_id,
    policy_id = EXCLUDED.policy_id,
    package_id = EXCLUDED.package_id,
    manager = EXCLUDED.manager,
    registry = EXCLUDED.registry,
    version_spec = EXCLUDED.version_spec,
    resolved_version = EXCLUDED.resolved_version,
    aliases = EXCLUDED.aliases,
    title = EXCLUDED.title,
    summary = EXCLUDED.summary,
    severity = EXCLUDED.severity,
    fixed_version = EXCLUDED.fixed_version,
    reference_url = EXCLUDED.reference_url,
    status = 'open',
    updated_at = NOW()
RETURNING id, scan_run_id, repository_id, policy_id, package_id, package_name, manager, registry, version_spec, resolved_version, advisory_id, aliases, title, summary, severity, fixed_version, reference_url, status, created_at, updated_at;

-- name: LinkRepositoryScanRunFinding :exec
INSERT INTO repository_scan_run_findings (
    scan_run_id,
    finding_id
) VALUES (
    $1,
    $2
)
ON CONFLICT (scan_run_id, finding_id)
DO NOTHING;

-- name: UpsertRepositoryFindingOccurrence :exec
INSERT INTO repository_finding_occurrences (
    repository_id,
    finding_id,
    status,
    first_seen_at,
    last_seen_at
) VALUES (
    $1,
    $2,
    'open',
    NOW(),
    NOW()
)
ON CONFLICT (repository_id, finding_id)
DO UPDATE SET
    status = 'open',
    last_seen_at = NOW(),
    updated_at = NOW();

-- name: AddRepositoryScanFindingSource :exec
INSERT INTO repository_scan_finding_sources (
    finding_id,
    source,
    provider_record_id
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (finding_id, source)
DO UPDATE SET
    provider_record_id = EXCLUDED.provider_record_id;

-- name: ListLatestRepositoryScanRunByRepo :many
SELECT id, repository_id, policy_id, trigger, status, error_message, findings_total, findings_critical, findings_high, findings_medium, findings_low, started_at, finished_at, created_at, updated_at
FROM repository_scan_runs
WHERE repository_id = $1
ORDER BY started_at DESC
LIMIT 1;

-- name: ListRepositoryScanRunsByUser :many
SELECT
    s.id,
    s.repository_id,
    r.full_name AS repository_full_name,
    COALESCE(p.name, '') AS policy_name,
    s.policy_id,
    s.trigger,
    s.status,
    s.error_message,
    s.findings_total,
    s.findings_critical,
    s.findings_high,
    s.findings_medium,
    s.findings_low,
    s.started_at,
    s.finished_at,
    s.created_at,
    s.updated_at
FROM repository_scan_runs s
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
LEFT JOIN policies p ON p.id = s.policy_id
WHERE r.user_id = $1
ORDER BY s.started_at DESC;

-- name: CountRepositoryScansByStatus :one
SELECT COUNT(*)::BIGINT
FROM repository_scan_runs
WHERE status = $1;

-- name: CountRepositoryScansSuccessSince :one
SELECT COUNT(*)::BIGINT
FROM repository_scan_runs
WHERE status = 'success'
  AND finished_at >= $1;

-- name: CountRepositoryScansFailedSince :one
SELECT COUNT(*)::BIGINT
FROM repository_scan_runs
WHERE status = 'failed'
  AND finished_at >= $1;

-- name: ListRepositoryScanRunsByRepoAndUser :many
SELECT
    s.id,
    s.repository_id,
    r.full_name AS repository_full_name,
    COALESCE(p.name, '') AS policy_name,
    s.policy_id,
    s.trigger,
    s.status,
    s.error_message,
    s.findings_total,
    s.findings_critical,
    s.findings_high,
    s.findings_medium,
    s.findings_low,
    s.started_at,
    s.finished_at,
    s.created_at,
    s.updated_at
FROM repository_scan_runs s
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
LEFT JOIN policies p ON p.id = s.policy_id
WHERE r.user_id = $1
  AND s.repository_id = $2
ORDER BY s.started_at DESC;

-- name: GetRepositoryScanRunByIDAndUser :one
SELECT
    s.id,
    s.repository_id,
    r.full_name AS repository_full_name,
    COALESCE(p.name, '') AS policy_name,
    s.policy_id,
    s.trigger,
    s.status,
    s.error_message,
    s.findings_total,
    s.findings_critical,
    s.findings_high,
    s.findings_medium,
    s.findings_low,
    s.started_at,
    s.finished_at,
    s.created_at,
    s.updated_at
FROM repository_scan_runs s
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
LEFT JOIN policies p ON p.id = s.policy_id
WHERE r.user_id = $1
  AND s.id = $2;

-- name: ListRepositoryScanFindingsByRunAndUser :many
SELECT
    f.id,
    srf.scan_run_id,
    s.repository_id,
    s.policy_id,
    f.package_id,
    f.package_name,
    f.manager,
    f.registry,
    f.version_spec,
    f.resolved_version,
    f.advisory_id,
    f.aliases,
    f.title,
    f.summary,
    f.severity,
    f.fixed_version,
    f.reference_url,
    COALESCE(rfo.status, 'open') AS status,
    f.created_at,
    f.updated_at
FROM repository_scan_findings f
INNER JOIN repository_scan_run_findings srf ON srf.finding_id = f.id
INNER JOIN repository_scan_runs s ON s.id = srf.scan_run_id
LEFT JOIN repository_finding_occurrences rfo ON rfo.finding_id = f.id AND rfo.repository_id = s.repository_id
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
WHERE r.user_id = $1
  AND srf.scan_run_id = $2
ORDER BY f.severity DESC, f.package_name ASC;

-- name: ListRepositoryScanFindingSourcesByRunAndUser :many
SELECT
    fs.finding_id,
    fs.source,
    fs.provider_record_id
FROM repository_scan_finding_sources fs
INNER JOIN repository_scan_findings f ON f.id = fs.finding_id
INNER JOIN repository_scan_run_findings srf ON srf.finding_id = f.id
INNER JOIN repository_scan_runs s ON s.id = srf.scan_run_id
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
WHERE r.user_id = $1
  AND srf.scan_run_id = $2
ORDER BY fs.finding_id, fs.source;

-- name: ListLatestRepositoryFindingsByRepoAndUser :many
WITH latest AS (
    SELECT s.id, s.repository_id, s.policy_id
    FROM repository_scan_runs s
    INNER JOIN repositories r ON r.github_repo_id = s.repository_id
    WHERE r.user_id = $1
      AND s.repository_id = $2
      AND s.status IN ('success', 'failed')
    ORDER BY s.started_at DESC
    LIMIT 1
)
SELECT
    f.id,
    latest.id AS scan_run_id,
    latest.repository_id,
    latest.policy_id,
    f.package_id,
    f.package_name,
    f.manager,
    f.registry,
    f.version_spec,
    f.resolved_version,
    f.advisory_id,
    f.aliases,
    f.title,
    f.summary,
    f.severity,
    f.fixed_version,
    f.reference_url,
    COALESCE(rfo.status, 'open') AS status,
    f.created_at,
    f.updated_at
FROM repository_scan_findings f
INNER JOIN repository_scan_run_findings srf ON srf.finding_id = f.id
INNER JOIN latest ON latest.id = srf.scan_run_id
LEFT JOIN repository_finding_occurrences rfo ON rfo.finding_id = f.id AND rfo.repository_id = latest.repository_id
ORDER BY f.severity DESC, f.package_name ASC;

-- name: ListLatestRepositoryFindingSourcesByRepoAndUser :many
WITH latest AS (
    SELECT s.id, s.repository_id, s.policy_id
    FROM repository_scan_runs s
    INNER JOIN repositories r ON r.github_repo_id = s.repository_id
    WHERE r.user_id = $1
      AND s.repository_id = $2
      AND s.status IN ('success', 'failed')
    ORDER BY s.started_at DESC
    LIMIT 1
)
SELECT
    fs.finding_id,
    fs.source,
    fs.provider_record_id
FROM repository_scan_finding_sources fs
INNER JOIN repository_scan_findings f ON f.id = fs.finding_id
INNER JOIN repository_scan_run_findings srf ON srf.finding_id = f.id
INNER JOIN latest ON latest.id = srf.scan_run_id
ORDER BY fs.finding_id, fs.source;

-- name: CreateRepositoryScanLog :exec
INSERT INTO repository_scan_logs (
    scan_run_id,
    repository_id,
    level,
    message,
    directory_path
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
);

-- name: ListRepositoryScanLogsByRunAndUser :many
SELECT
    l.id,
    l.scan_run_id,
    l.repository_id,
    l.level,
    l.message,
    l.directory_path,
    l.created_at
FROM repository_scan_logs l
INNER JOIN repository_scan_runs s ON s.id = l.scan_run_id
INNER JOIN repositories r ON r.github_repo_id = s.repository_id
WHERE r.user_id = $1
  AND l.scan_run_id = $2
ORDER BY l.created_at ASC, l.id ASC;

-- name: ListFindingsByUser :many
SELECT
    f.id,
    COALESCE(sr.scan_run_id, f.scan_run_id) AS scan_run_id,
    occ.repository_id,
    r.full_name AS repository_full_name,
    COALESCE(p.name, '') AS policy_name,
    f.package_name,
    f.manager,
    f.registry,
    f.version_spec,
    f.resolved_version,
    f.advisory_id,
    f.aliases,
    f.title,
    f.summary,
    f.severity,
    f.fixed_version,
    f.reference_url,
    occ.status,
    COALESCE((
        SELECT ARRAY_AGG(src.source ORDER BY src.source)
        FROM (
            SELECT DISTINCT fs2.source
            FROM repository_scan_finding_sources fs2
            WHERE fs2.finding_id = f.id
        ) AS src
    ), ARRAY[]::TEXT[])::TEXT[] AS sources,
    f.created_at,
    f.updated_at
FROM repository_scan_findings f
INNER JOIN LATERAL (
    SELECT
        rfo.repository_id,
        rfo.status,
        rfo.last_seen_at
    FROM repository_finding_occurrences rfo
    INNER JOIN repositories r2 ON r2.github_repo_id = rfo.repository_id
    WHERE rfo.finding_id = f.id
      AND r2.user_id = $1
    ORDER BY rfo.last_seen_at DESC, rfo.repository_id DESC
    LIMIT 1
) occ ON TRUE
INNER JOIN repositories r ON r.github_repo_id = occ.repository_id
LEFT JOIN LATERAL (
    SELECT srf.scan_run_id
    FROM repository_scan_run_findings srf
    INNER JOIN repository_scan_runs s2 ON s2.id = srf.scan_run_id
    WHERE srf.finding_id = f.id
      AND s2.repository_id = occ.repository_id
    ORDER BY s2.started_at DESC, s2.id DESC
    LIMIT 1
) sr ON TRUE
LEFT JOIN repository_scan_runs s ON s.id = sr.scan_run_id
LEFT JOIN policies p ON p.id = s.policy_id
ORDER BY f.created_at DESC, f.id DESC;

-- name: GetFindingByIDAndUser :one
SELECT
    f.id,
    COALESCE(sr.scan_run_id, f.scan_run_id) AS scan_run_id,
    occ.repository_id,
    r.full_name AS repository_full_name,
    COALESCE(p.name, '') AS policy_name,
    f.package_name,
    f.manager,
    f.registry,
    f.version_spec,
    f.resolved_version,
    f.advisory_id,
    f.aliases,
    f.title,
    f.summary,
    f.severity,
    f.fixed_version,
    f.reference_url,
    occ.status,
    COALESCE((
        SELECT ARRAY_AGG(src.source ORDER BY src.source)
        FROM (
            SELECT DISTINCT fs2.source
            FROM repository_scan_finding_sources fs2
            WHERE fs2.finding_id = f.id
        ) AS src
    ), ARRAY[]::TEXT[])::TEXT[] AS sources,
    f.created_at,
    f.updated_at
FROM repository_scan_findings f
INNER JOIN LATERAL (
    SELECT
        rfo.repository_id,
        rfo.status,
        rfo.last_seen_at
    FROM repository_finding_occurrences rfo
    INNER JOIN repositories r2 ON r2.github_repo_id = rfo.repository_id
    WHERE rfo.finding_id = f.id
      AND r2.user_id = $1
    ORDER BY rfo.last_seen_at DESC, rfo.repository_id DESC
    LIMIT 1
) occ ON TRUE
INNER JOIN repositories r ON r.github_repo_id = occ.repository_id
LEFT JOIN LATERAL (
    SELECT srf.scan_run_id
    FROM repository_scan_run_findings srf
    INNER JOIN repository_scan_runs s2 ON s2.id = srf.scan_run_id
    WHERE srf.finding_id = f.id
      AND s2.repository_id = occ.repository_id
    ORDER BY s2.started_at DESC, s2.id DESC
    LIMIT 1
) sr ON TRUE
LEFT JOIN repository_scan_runs s ON s.id = sr.scan_run_id
LEFT JOIN policies p ON p.id = s.policy_id
WHERE r.user_id = $1
  AND f.id = $2
;
